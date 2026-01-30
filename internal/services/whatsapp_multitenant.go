package services

import (
	"boot-whatsapp-golang/internal/config"
	"boot-whatsapp-golang/internal/models"
	"boot-whatsapp-golang/internal/repository"
	"boot-whatsapp-golang/pkg/logger"
	"context"
	"database/sql"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/skip2/go-qrcode"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"
)

type WhatsAppClient struct {
	Client  *whatsmeow.Client
	Session *models.WhatsAppSession

	cancelQR context.CancelFunc

	qrMu        sync.RWMutex
	lastQRCode  string
	lastQRTime  time.Time
	lastQRExpAt time.Time
}

func (c *WhatsAppClient) setQR(codeBase64 string, exp time.Time) {
	c.qrMu.Lock()
	c.lastQRCode = codeBase64
	c.lastQRTime = time.Now()
	c.lastQRExpAt = exp
	c.qrMu.Unlock()
}

func (c *WhatsAppClient) getQR() (string, time.Time, bool) {
	c.qrMu.RLock()
	qr := c.lastQRCode
	exp := c.lastQRExpAt
	c.qrMu.RUnlock()
	if qr == "" {
		return "", time.Time{}, false
	}
	return qr, exp, true
}

const clientShards = 64

type clientShard struct {
	mu sync.RWMutex
	m  map[string]*WhatsAppClient
}

type clientStore struct {
	shards [clientShards]clientShard
}

func newClientStore() *clientStore {
	cs := &clientStore{}
	for i := 0; i < clientShards; i++ {
		cs.shards[i].m = make(map[string]*WhatsAppClient)
	}
	return cs
}

func (cs *clientStore) shard(key string) *clientShard {
	var h uint32 = 2166136261
	for i := 0; i < len(key); i++ {
		h ^= uint32(key[i])
		h *= 16777619
	}
	return &cs.shards[h%clientShards]
}

func (cs *clientStore) Get(key string) (*WhatsAppClient, bool) {
	sh := cs.shard(key)
	sh.mu.RLock()
	v, ok := sh.m[key]
	sh.mu.RUnlock()
	return v, ok
}

func (cs *clientStore) Set(key string, v *WhatsAppClient) {
	sh := cs.shard(key)
	sh.mu.Lock()
	sh.m[key] = v
	sh.mu.Unlock()
}

func (cs *clientStore) Delete(key string) (*WhatsAppClient, bool) {
	sh := cs.shard(key)
	sh.mu.Lock()
	v, ok := sh.m[key]
	if ok {
		delete(sh.m, key)
	}
	sh.mu.Unlock()
	return v, ok
}

func (cs *clientStore) Range(fn func(key string, v *WhatsAppClient)) {
	for i := 0; i < clientShards; i++ {
		sh := &cs.shards[i]
		sh.mu.RLock()
		for k, v := range sh.m {
			fn(k, v)
		}
		sh.mu.RUnlock()
	}
}

type MultiTenantWhatsAppService struct {
	clients *clientStore

	config     *config.Config
	logger     *logger.Logger
	repository *repository.SessionRepository
	container  *sqlstore.Container

	httpClient *http.Client
}

func NewMultiTenantWhatsAppService(cfg *config.Config, db *sql.DB, log *logger.Logger) (*MultiTenantWhatsAppService, error) {
	waLogger := logger.NewWhatsAppLogger("[WhatsApp] ", logger.INFO)

	ctx := context.Background()
	container, err := sqlstore.New(ctx, cfg.Database.Driver, cfg.Database.DSN, waLogger)
	if err != nil {
		return nil, fmt.Errorf("falha ao inicializar banco de dados: %w", err)
	}

	repo := repository.NewSessionRepository(db, log)

	tr := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:          1024,
		MaxIdleConnsPerHost:   256,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
	httpClient := &http.Client{
		Transport: tr,
		Timeout:   2 * time.Minute,
	}

	service := &MultiTenantWhatsAppService{
		clients:    newClientStore(),
		config:     cfg,
		logger:     log,
		repository: repo,
		container:  container,
		httpClient: httpClient,
	}

	if err := service.LoadExistingSessions(); err != nil {
		log.Warnf("Falha ao carregar sessões existentes: %v", err)
	}

	return service, nil
}

func (s *MultiTenantWhatsAppService) LoadExistingSessions() error {
	s.logger.Info("Carregando sessões existentes do banco de dados...")

	sessions, err := s.repository.List()
	if err != nil {
		return err
	}
	s.logger.Infof("Encontradas %d sessões no banco de dados", len(sessions))

	ctx := context.Background()
	devices, devErr := s.container.GetAllDevices(ctx)
	byUser := make(map[string]*store.Device, len(devices))
	if devErr != nil {
		s.logger.Warnf("Falha ao listar devices: %v", devErr)
	} else {
		for _, ds := range devices {
			if ds != nil && ds.ID != nil {
				byUser[ds.ID.User] = ds
			}
		}
	}

	for _, session := range sessions {
		phone := ""
		if session.PhoneNumber != nil {
			phone = *session.PhoneNumber
		}

		if (session.DeviceJID == nil || *session.DeviceJID == "") && phone != "" {
			if ds := byUser[phone]; ds != nil && ds.ID != nil {
				deviceJIDStr := ds.ID.String()
				session.DeviceJID = &deviceJIDStr
				if err := s.repository.UpdateDeviceJID(session.ID, deviceJIDStr); err != nil {
					s.logger.Warnf("Falha ao persistir device_jid: %v", err)
				}
			}
		}

		if phone != "" && (session.Status == models.SessionStatusConnected || session.Status == models.SessionStatusDisconnected) {
			if err := s.reconnectSession(session); err != nil {
				s.logger.Errorf("Falha ao reconectar sessão %s: %v", session.WhatsAppSessionKey, err)
				if updateErr := s.repository.UpdateStatus(session.ID, models.SessionStatusDisconnected, "", ""); updateErr != nil {
					s.logger.Errorf("Falha ao atualizar status após erro de reconexão: %v", updateErr)
				}
			}
		}
	}

	s.logger.Info("Carregamento de sessões concluído")
	return nil
}

func (s *MultiTenantWhatsAppService) RegisterSession(req *models.RegisterSessionRequest, tenantID string) (*models.RegisterSessionResponse, error) {
	if old, ok := s.clients.Delete(req.WhatsAppSessionKey); ok {
		if old.cancelQR != nil {
			old.cancelQR()
		}
		if old.Client != nil {
			old.Client.Disconnect()
		}
	}

	var session *models.WhatsAppSession
	exists, err := s.repository.ExistsBySessionKeyAndTenant(req.WhatsAppSessionKey, tenantID)
	if err != nil {
		return nil, err
	}
	if exists {
		session, err = s.repository.GetBySessionKeyAndTenant(req.WhatsAppSessionKey, tenantID)
		if err != nil {
			return nil, err
		}
		if err := s.repository.ResetSessionForReRegister(session.ID, req.NomePessoa, req.EmailPessoa); err != nil {
			return nil, err
		}
		session.NomePessoa = req.NomePessoa
		session.EmailPessoa = req.EmailPessoa
		session.Status = models.SessionStatusPending
		session.PhoneNumber = nil
		session.DeviceJID = nil
		session.QRCode = nil
		session.QRCodeExpiresAt = nil
		session.LastConnectedAt = nil
		session.UpdatedAt = time.Now()
	} else {
		session = &models.WhatsAppSession{
			ID:                 uuid.New(),
			TenantID:           tenantID,
			WhatsAppSessionKey: req.WhatsAppSessionKey,
			NomePessoa:         req.NomePessoa,
			EmailPessoa:        req.EmailPessoa,
			Status:             models.SessionStatusPending,
			CreatedAt:          time.Now(),
			UpdatedAt:          time.Now(),
		}
		if err := s.repository.Create(session); err != nil {
			return nil, err
		}
	}

	deviceStore := s.container.NewDevice()

	waLogger := logger.NewWhatsAppLogger(fmt.Sprintf("[WA:%s] ", session.WhatsAppSessionKey), logger.INFO)
	client := whatsmeow.NewClient(deviceStore, waLogger)
	s.registerEventHandlers(client, session)

	qrCtx, cancelQR := context.WithCancel(context.Background())
	qrChan, err := client.GetQRChannel(qrCtx)
	if err != nil {
		cancelQR()
		return nil, fmt.Errorf("falha ao obter canal de QR: %w", err)
	}

	waClient := &WhatsAppClient{
		Client:   client,
		Session:  session,
		cancelQR: cancelQR,
	}
	s.clients.Set(session.WhatsAppSessionKey, waClient)

	if err := client.Connect(); err != nil {
		cancelQR()
		s.clients.Delete(session.WhatsAppSessionKey)
		return nil, fmt.Errorf("falha ao conectar: %w", err)
	}

	go s.monitorQRCode(session, waClient, qrChan)

	timeout := time.NewTimer(8 * time.Second)
	ticker := time.NewTicker(200 * time.Millisecond)
	defer timeout.Stop()
	defer ticker.Stop()

	for {
		select {
		case <-timeout.C:
			return nil, fmt.Errorf("timeout aguardando QR code")
		case <-ticker.C:
			if client.Store != nil && client.Store.ID != nil {
				cancelQR()
				phoneNumber := client.Store.ID.User
				deviceJID := client.Store.ID.String()
				_ = s.repository.UpdateStatus(session.ID, models.SessionStatusConnected, phoneNumber, deviceJID)
				return &models.RegisterSessionResponse{
					ID:                 session.ID,
					WhatsAppSessionKey: session.WhatsAppSessionKey,
					QRCodeBase64:       "",
					Status:             models.SessionStatusConnected,
					ExpiresAt:          time.Time{},
				}, nil
			}
			if qr, exp, ok := waClient.getQR(); ok {
				return &models.RegisterSessionResponse{
					ID:                 session.ID,
					WhatsAppSessionKey: session.WhatsAppSessionKey,
					QRCodeBase64:       qr,
					Status:             models.SessionStatusPending,
					ExpiresAt:          exp,
				}, nil
			}
		}
	}
}

func (s *MultiTenantWhatsAppService) monitorQRCode(session *models.WhatsAppSession, waClient *WhatsAppClient, qrChan <-chan whatsmeow.QRChannelItem) {
	for item := range qrChan {
		switch item.Event {
		case "code":
			qrCodePNG, err := qrcode.Encode(item.Code, qrcode.Medium, 256)
			if err != nil {
				s.logger.Errorf("Falha ao gerar QR code PNG: %v", err)
				continue
			}

			qrCodeBase64 := base64.StdEncoding.EncodeToString(qrCodePNG)
			exp := time.Now().Add(60 * time.Second)
			if item.Timeout > 0 {
				exp = time.Now().Add(item.Timeout)
			}

			waClient.setQR(qrCodeBase64, exp)
			if err := s.repository.UpdateQRCode(session.ID, qrCodeBase64, exp); err != nil {
				s.logger.Errorf("Falha ao atualizar QR code no banco: %v", err)
			}

		case "success":
			if waClient.cancelQR != nil {
				waClient.cancelQR()
			}
			return
		case "timeout":
			return
		default:
			if item.Error != nil {
				s.logger.Errorf("Erro no pairing: %v", item.Error)
			}
			return
		}
	}
}

func (s *MultiTenantWhatsAppService) registerEventHandlers(client *whatsmeow.Client, session *models.WhatsAppSession) {
	client.AddEventHandler(func(evt interface{}) {
		switch evt.(type) {
		case *events.Connected:
			phoneNumber := ""
			deviceJID := ""
			if client.Store != nil && client.Store.ID != nil {
				phoneNumber = client.Store.ID.User
				deviceJID = client.Store.ID.String()
			}
			_ = s.repository.UpdateStatus(session.ID, models.SessionStatusConnected, phoneNumber, deviceJID)

		case *events.Disconnected:
			_ = s.repository.UpdateStatus(session.ID, models.SessionStatusDisconnected, "", "")

		case *events.LoggedOut:
			if client.Store != nil {
				_ = client.Store.Delete(context.Background())
			}
			_ = s.repository.MarkLoggedOut(session.ID)
		}
	})
}

func (s *MultiTenantWhatsAppService) reconnectSession(session *models.WhatsAppSession) error {
	if session.PhoneNumber == nil || *session.PhoneNumber == "" {
		return fmt.Errorf("sessão não pode ser reconectada: phone_number ausente")
	}

	ctx := context.Background()

	var deviceStore *store.Device
	var err error

	if session.DeviceJID != nil && *session.DeviceJID != "" {
		if jid, parseErr := types.ParseJID(*session.DeviceJID); parseErr == nil {
			deviceStore, err = s.container.GetDevice(ctx, jid)
			if err != nil {
				s.logger.Warnf("GetDevice por device_jid falhou: %v", err)
			}
		}
	}

	if deviceStore == nil || deviceStore.ID == nil {
		jid, parseErr := types.ParseJID(*session.PhoneNumber + "@s.whatsapp.net")
		if parseErr != nil {
			return fmt.Errorf("falha ao parse JID: %w", parseErr)
		}
		deviceStore, err = s.container.GetDevice(ctx, jid)
		if err != nil {
			s.logger.Warnf("GetDevice por JID falhou: %v", err)
		}
	}

	if deviceStore == nil || deviceStore.ID == nil {
		_ = s.repository.UpdateStatus(session.ID, models.SessionStatusPending, "", "")
		return nil
	}

	waLogger := logger.NewWhatsAppLogger(fmt.Sprintf("[WA:%s] ", session.WhatsAppSessionKey), logger.INFO)
	client := whatsmeow.NewClient(deviceStore, waLogger)
	s.registerEventHandlers(client, session)

	waClient := &WhatsAppClient{Client: client, Session: session}
	s.clients.Set(session.WhatsAppSessionKey, waClient)

	go func() {
		if err := client.Connect(); err != nil {
			s.logger.Errorf("Falha ao reconectar sessão %s: %v", session.WhatsAppSessionKey, err)
			_ = s.repository.UpdateStatus(session.ID, models.SessionStatusDisconnected, "", "")
		}
	}()

	return nil
}

var (
	ErrSessionAlreadyConnected = fmt.Errorf("SESSION_ALREADY_CONNECTED")
)

func (s *MultiTenantWhatsAppService) GetQRCode(sessionKey string, tenantID string) (string, error) {
	session, err := s.repository.GetBySessionKeyAndTenant(sessionKey, tenantID)
	if err != nil {
		return "", fmt.Errorf("sessão não encontrada ou não pertence a este tenant")
	}

	waClient, ok := s.clients.Get(sessionKey)
	if !ok {
		if session.QRCode != nil && session.QRCodeExpiresAt != nil {
			if time.Now().Before(*session.QRCodeExpiresAt) {
				return *session.QRCode, nil
			}
			return "", fmt.Errorf("QR code expirado, gere outro")
		}

		if session.Status == models.SessionStatusConnected {
			return "", ErrSessionAlreadyConnected
		}

		return "", fmt.Errorf("QR code ainda não foi gerado")
	}

	if waClient.Client != nil && waClient.Client.Store != nil && waClient.Client.Store.ID != nil {
		return "", ErrSessionAlreadyConnected
	}

	qr, exp, ok := waClient.getQR()
	if !ok {
		return "", fmt.Errorf("QR code ainda não foi gerado")
	}
	if !exp.IsZero() && time.Now().After(exp) {
		if newQR, _, regenErr := s.refreshQRCode(session, waClient); regenErr == nil {
			return newQR, nil
		}

		return "", fmt.Errorf("QR code expirado, gere outro")
	}
	return qr, nil
}

func (s *MultiTenantWhatsAppService) refreshQRCode(session *models.WhatsAppSession, waClient *WhatsAppClient) (string, time.Time, error) {
	if waClient.cancelQR != nil {
		waClient.cancelQR()
	}

	qrCtx, cancelQR := context.WithCancel(context.Background())
	waClient.cancelQR = cancelQR

	qrChan, err := waClient.Client.GetQRChannel(qrCtx)
	if err != nil {
		cancelQR()
		return "", time.Time{}, err
	}

	if !waClient.Client.IsConnected() {
		if err := waClient.Client.Connect(); err != nil {
			cancelQR()
			return "", time.Time{}, err
		}
	}

	go s.monitorQRCode(session, waClient, qrChan)

	timeout := time.NewTimer(8 * time.Second)
	ticker := time.NewTicker(200 * time.Millisecond)
	defer timeout.Stop()
	defer ticker.Stop()

	for {
		select {
		case <-timeout.C:
			return "", time.Time{}, fmt.Errorf("timeout aguardando QR code")
		case <-ticker.C:
			if qr, exp, ok := waClient.getQR(); ok {
				return qr, exp, nil
			}
		}
	}
}

func (s *MultiTenantWhatsAppService) GetClient(sessionKey string) (*whatsmeow.Client, error) {
	waClient, ok := s.clients.Get(sessionKey)
	if !ok {
		return nil, fmt.Errorf("sessão não encontrada: %s", sessionKey)
	}

	if waClient.Client == nil || waClient.Client.Store == nil || waClient.Client.Store.ID == nil {
		return nil, fmt.Errorf("sessão não está autenticada")
	}
	if !waClient.Client.IsConnected() {
		return nil, fmt.Errorf("sessão não está conectada")
	}
	return waClient.Client, nil
}

func (s *MultiTenantWhatsAppService) SendTextMessage(sessionKey, number, text string) error {
	client, err := s.GetClient(sessionKey)
	if err != nil {
		return err
	}

	jid, err := s.parsePhoneNumber(number)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, err = client.SendMessage(ctx, jid, &waE2E.Message{
		ExtendedTextMessage: &waE2E.ExtendedTextMessage{
			Text: proto.String(text),
		},
	})
	if err != nil {
		return fmt.Errorf("falha ao enviar mensagem: %w", err)
	}
	return nil
}

func (s *MultiTenantWhatsAppService) SendMediaMessage(sessionKey, number, caption, mediaURL, mediaBase64, mimeType string) error {
	client, err := s.GetClient(sessionKey)
	if err != nil {
		return err
	}

	jid, err := s.parsePhoneNumber(number)
	if err != nil {
		return err
	}

	mediaData, contentType, filename, err := s.prepareMedia(mediaURL, mediaBase64, mimeType)
	if err != nil {
		return err
	}

	mediaType := s.determineMediaType(contentType)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	uploaded, err := client.Upload(ctx, mediaData, mediaType)
	if err != nil {
		return fmt.Errorf("falha ao fazer upload da mídia: %w", err)
	}

	msg := s.buildMediaMessage(uploaded, mediaData, contentType, caption, filename)

	_, err = client.SendMessage(ctx, jid, msg)
	if err != nil {
		return fmt.Errorf("falha ao enviar mensagem de mídia: %w", err)
	}
	return nil
}

func (s *MultiTenantWhatsAppService) ListSessions() ([]*models.WhatsAppSession, error) {
	return s.repository.List()
}

func (s *MultiTenantWhatsAppService) ListSessionsByTenant(tenantID string) ([]*models.WhatsAppSession, error) {
	return s.repository.ListByTenant(tenantID)
}

func (s *MultiTenantWhatsAppService) GetSessionByKeyAndTenant(sessionKey string, tenantID string) (*models.WhatsAppSession, error) {
	return s.repository.GetBySessionKeyAndTenant(sessionKey, tenantID)
}

func (s *MultiTenantWhatsAppService) DisconnectSession(sessionKey string, tenantID string) error {
	_, err := s.repository.GetBySessionKeyAndTenant(sessionKey, tenantID)
	if err != nil {
		return fmt.Errorf("sessão não encontrada ou não pertence a este tenant")
	}

	waClient, ok := s.clients.Delete(sessionKey)
	if !ok {
		return fmt.Errorf("sessão não está conectada")
	}

	if waClient.cancelQR != nil {
		waClient.cancelQR()
	}

	if waClient.Client != nil && waClient.Client.IsConnected() {
		waClient.Client.Disconnect()
	}

	if waClient.Session != nil {
		if err := s.repository.UpdateStatus(waClient.Session.ID, models.SessionStatusDisconnected, "", ""); err != nil {
			return err
		}
	}

	return nil
}

func (s *MultiTenantWhatsAppService) DeleteSession(sessionKey string, tenantID string) error {
	_ = s.DisconnectSession(sessionKey, tenantID)

	session, err := s.repository.GetBySessionKeyAndTenant(sessionKey, tenantID)
	if err != nil {
		return fmt.Errorf("sessão não encontrada ou não pertence a este tenant")
	}
	return s.repository.Delete(session.ID)
}

func (s *MultiTenantWhatsAppService) parsePhoneNumber(number string) (types.JID, error) {
	n := strings.TrimSpace(number)
	n = strings.NewReplacer(" ", "", "-", "", "(", "", ")", "").Replace(n)

	if !strings.HasSuffix(n, "@s.whatsapp.net") {
		if !strings.HasPrefix(n, s.config.WhatsApp.DefaultCountry) {
			n = s.config.WhatsApp.DefaultCountry + n
		}
		n = n + "@s.whatsapp.net"
	}

	jid, err := types.ParseJID(n)
	if err != nil {
		return types.JID{}, fmt.Errorf("número de telefone inválido: %w", err)
	}
	return jid, nil
}

func (s *MultiTenantWhatsAppService) prepareMedia(mediaURL, mediaBase64, mimeType string) ([]byte, string, string, error) {
	switch {
	case mediaBase64 != "":
		data, ct, err := s.decodeBase64Media(mediaBase64, mimeType)
		if err != nil {
			return nil, "", "", err
		}
		ext := ""
		if exts, _ := mime.ExtensionsByType(ct); len(exts) > 0 {
			ext = exts[0]
		}
		if ext == "" {
			return data, ct, "media", nil
		}
		return data, ct, "media" + ext, nil

	case mediaURL != "":
		data, ct, err := s.downloadMedia(mediaURL)
		if err != nil {
			return nil, "", "", err
		}
		ext := filepath.Ext(mediaURL)
		if ext == "" {
			if exts, _ := mime.ExtensionsByType(ct); len(exts) > 0 {
				ext = exts[0]
			}
		}
		if ext == "" {
			return data, ct, "media", nil
		}
		return data, ct, "media" + ext, nil

	default:
		return nil, "", "", fmt.Errorf("é necessário fornecer media_url ou media_base64")
	}
}

func (s *MultiTenantWhatsAppService) decodeBase64Media(base64Str, mimeType string) ([]byte, string, error) {
	b := strings.TrimSpace(base64Str)

	if strings.HasPrefix(b, "data:") {
		if idx := strings.IndexByte(b, ','); idx != -1 {
			b = b[idx+1:]
		}
	}
	b = strings.ReplaceAll(b, "\\n", "")
	b = strings.ReplaceAll(b, "\\r", "")
	b = strings.TrimSpace(b)

	data, err := base64.StdEncoding.DecodeString(b)
	if err != nil {
		return nil, "", fmt.Errorf("falha ao decodificar base64: %w", err)
	}

	ct := mimeType
	if ct == "" {
		ct = http.DetectContentType(data)
	}
	return data, ct, nil
}

func (s *MultiTenantWhatsAppService) downloadMedia(url string) ([]byte, string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, "", fmt.Errorf("falha ao criar requisição: %w", err)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("falha ao baixar mídia: %w", err)
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			s.logger.Errorf("falha ao fechar corpo da resposta: %v", err)
		}
	}(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("falha ao baixar mídia: status %d", resp.StatusCode)
	}

	limit := s.config.Server.MaxUploadSize
	if limit <= 0 {
		limit = 25 << 20 // fallback 25MB
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, limit))
	if err != nil {
		return nil, "", fmt.Errorf("falha ao ler mídia: %w", err)
	}

	ct := resp.Header.Get("Content-Type")
	if ct == "" {
		ct = http.DetectContentType(data)
	}
	return data, ct, nil
}

func (s *MultiTenantWhatsAppService) determineMediaType(contentType string) whatsmeow.MediaType {
	switch {
	case strings.HasPrefix(contentType, "image/"):
		return whatsmeow.MediaImage
	case strings.HasPrefix(contentType, "video/"):
		return whatsmeow.MediaVideo
	case strings.HasPrefix(contentType, "audio/"):
		return whatsmeow.MediaAudio
	default:
		return whatsmeow.MediaDocument
	}
}

func (s *MultiTenantWhatsAppService) buildMediaMessage(uploaded whatsmeow.UploadResponse, mediaData []byte, contentType, caption, filename string) *waE2E.Message {
	mt := s.determineMediaType(contentType)
	size := uint64(len(mediaData))

	switch mt {
	case whatsmeow.MediaImage:
		return &waE2E.Message{
			ImageMessage: &waE2E.ImageMessage{
				URL:           proto.String(uploaded.URL),
				DirectPath:    proto.String(uploaded.DirectPath),
				MediaKey:      uploaded.MediaKey,
				Mimetype:      proto.String(contentType),
				FileEncSHA256: uploaded.FileEncSHA256,
				FileSHA256:    uploaded.FileSHA256,
				FileLength:    proto.Uint64(size),
				Caption:       proto.String(caption),
			},
		}
	case whatsmeow.MediaVideo:
		return &waE2E.Message{
			VideoMessage: &waE2E.VideoMessage{
				URL:           proto.String(uploaded.URL),
				DirectPath:    proto.String(uploaded.DirectPath),
				MediaKey:      uploaded.MediaKey,
				Mimetype:      proto.String(contentType),
				FileEncSHA256: uploaded.FileEncSHA256,
				FileSHA256:    uploaded.FileSHA256,
				FileLength:    proto.Uint64(size),
				Caption:       proto.String(caption),
			},
		}
	case whatsmeow.MediaAudio:
		return &waE2E.Message{
			AudioMessage: &waE2E.AudioMessage{
				URL:           proto.String(uploaded.URL),
				DirectPath:    proto.String(uploaded.DirectPath),
				MediaKey:      uploaded.MediaKey,
				Mimetype:      proto.String(contentType),
				FileEncSHA256: uploaded.FileEncSHA256,
				FileSHA256:    uploaded.FileSHA256,
				FileLength:    proto.Uint64(size),
			},
		}
	default:
		return &waE2E.Message{
			DocumentMessage: &waE2E.DocumentMessage{
				URL:           proto.String(uploaded.URL),
				DirectPath:    proto.String(uploaded.DirectPath),
				MediaKey:      uploaded.MediaKey,
				Mimetype:      proto.String(contentType),
				FileEncSHA256: uploaded.FileEncSHA256,
				FileSHA256:    uploaded.FileSHA256,
				FileLength:    proto.Uint64(size),
				FileName:      proto.String(filename),
				Caption:       proto.String(caption),
			},
		}
	}
}

func (s *MultiTenantWhatsAppService) Shutdown() {
	s.logger.Info("Desconectando todas as sessões...")
	s.clients.Range(func(key string, waClient *WhatsAppClient) {
		if waClient != nil && waClient.cancelQR != nil {
			waClient.cancelQR()
		}
		if waClient != nil && waClient.Client != nil && waClient.Client.IsConnected() {
			waClient.Client.Disconnect()
		}
		s.clients.Delete(key)
	})
}
