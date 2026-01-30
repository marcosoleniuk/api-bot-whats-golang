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
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	qrcode "github.com/skip2/go-qrcode"
	"go.mau.fi/whatsmeow"
	waProto "go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"
)

type WhatsAppClient struct {
	Client         *whatsmeow.Client
	Session        *models.WhatsAppSession
	QRChan         <-chan whatsmeow.QRChannelItem
	CancelQR       context.CancelFunc
	LastQRCode     string
	LastQRCodeTime time.Time
}

type MultiTenantWhatsAppService struct {
	clients    map[string]*WhatsAppClient
	mu         sync.RWMutex
	config     *config.Config
	logger     *logger.Logger
	repository *repository.SessionRepository
	container  *sqlstore.Container
}

func NewMultiTenantWhatsAppService(cfg *config.Config, db *sql.DB, log *logger.Logger) (*MultiTenantWhatsAppService, error) {
	waLogger := logger.NewWhatsAppLogger("[WhatsApp] ", logger.INFO)

	ctx := context.Background()
	container, err := sqlstore.New(ctx, cfg.Database.Driver, cfg.Database.DSN, waLogger)
	if err != nil {
		return nil, fmt.Errorf("falha ao inicializar banco de dados: %w", err)
	}

	repo := repository.NewSessionRepository(db, log)

	service := &MultiTenantWhatsAppService{
		clients:    make(map[string]*WhatsAppClient),
		config:     cfg,
		logger:     log,
		repository: repo,
		container:  container,
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

	for _, session := range sessions {
		s.logger.Infof("Processando sessão: %s (Status: %s, Phone: %s)",
			session.WhatsAppSessionKey, session.Status, session.PhoneNumber)

		if session.DeviceJID == "" && session.PhoneNumber != "" {
			devices, listErr := s.container.GetAllDevices(context.Background())
			if listErr != nil {
				s.logger.Warnf("Falha ao listar devices para backfill: %v", listErr)
			} else {
				for _, ds := range devices {
					if ds != nil && ds.ID != nil && ds.ID.User == session.PhoneNumber {
						session.DeviceJID = ds.ID.String()
						if err := s.repository.UpdateDeviceJID(session.ID, session.DeviceJID); err != nil {
							s.logger.Warnf("Falha ao persistir device_jid: %v", err)
						}
						break
					}
				}
			}
		}

		if session.PhoneNumber != "" &&
			(session.Status == models.SessionStatusConnected || session.Status == models.SessionStatusDisconnected) {
			s.logger.Infof("Reconectando sessão: %s", session.WhatsAppSessionKey)
			if err := s.reconnectSession(session); err != nil {
				s.logger.Errorf("Falha ao reconectar sessão %s: %v", session.WhatsAppSessionKey, err)
				if updateErr := s.repository.UpdateStatus(session.ID, models.SessionStatusDisconnected, "", ""); updateErr != nil {
					s.logger.Errorf("Falha ao atualizar status após erro de reconexão: %v", updateErr)
				}
			}
		} else {
			s.logger.Infof("Sessão %s não será reconectada (Status: %s, Phone: %s)",
				session.WhatsAppSessionKey, session.Status, session.PhoneNumber)
		}
	}

	s.logger.Info("Carregamento de sessões concluído")
	return nil
}

func (s *MultiTenantWhatsAppService) RegisterSession(req *models.RegisterSessionRequest) (*models.RegisterSessionResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	session := &models.WhatsAppSession{
		ID:                 uuid.New(),
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

	if err := client.Connect(); err != nil {
		cancelQR()
		return nil, fmt.Errorf("falha ao conectar: %w", err)
	}

	whatsappClient := &WhatsAppClient{
		Client:         client,
		Session:        session,
		QRChan:         qrChan,
		CancelQR:       cancelQR,
		LastQRCodeTime: time.Now(),
	}

	s.clients[session.WhatsAppSessionKey] = whatsappClient

	go s.monitorQRCode(whatsappClient)

	timeout := time.After(10 * time.Second)
	select {
	case <-timeout:
		return nil, fmt.Errorf("timeout aguardando QR code")
	case <-time.After(500 * time.Millisecond):
		if whatsappClient.LastQRCode == "" {
			time.Sleep(1 * time.Second)
		}
	}

	qrCodeBase64 := whatsappClient.LastQRCode
	if qrCodeBase64 == "" {
		return nil, fmt.Errorf("QR code não gerado")
	}

	expiresAt := time.Now().Add(60 * time.Second)

	return &models.RegisterSessionResponse{
		ID:                 session.ID,
		WhatsAppSessionKey: session.WhatsAppSessionKey,
		QRCodeBase64:       qrCodeBase64,
		Status:             session.Status,
		ExpiresAt:          expiresAt,
	}, nil
}

func (s *MultiTenantWhatsAppService) monitorQRCode(waClient *WhatsAppClient) {
	for evt := range waClient.QRChan {
		if evt.Event == "code" {
			s.logger.Infof("Novo QR code gerado para sessão: %s", waClient.Session.WhatsAppSessionKey)

			qrCodePNG, err := qrcode.Encode(evt.Code, qrcode.Medium, 256)
			if err != nil {
				s.logger.Errorf("Falha ao gerar QR code PNG: %v", err)
				continue
			}

			qrCodeBase64 := base64.StdEncoding.EncodeToString(qrCodePNG)
			waClient.LastQRCode = qrCodeBase64
			waClient.LastQRCodeTime = time.Now()

			expiresAt := time.Now().Add(60 * time.Second)
			if err := s.repository.UpdateQRCode(waClient.Session.ID, qrCodeBase64, expiresAt); err != nil {
				s.logger.Errorf("Falha ao atualizar QR code no banco: %v", err)
			}
		} else {
			s.logger.Infof("Evento de login para %s: %s", waClient.Session.WhatsAppSessionKey, evt.Event)
		}
	}
}

func (s *MultiTenantWhatsAppService) registerEventHandlers(client *whatsmeow.Client, session *models.WhatsAppSession) {
	client.AddEventHandler(func(evt interface{}) {
		switch evt.(type) {
		case *events.Connected:
			s.logger.Infof("Sessão %s conectada ao WhatsApp", session.WhatsAppSessionKey)

			phoneNumber := ""
			deviceJID := ""
			if client.Store.ID != nil {
				phoneNumber = client.Store.ID.User
				deviceJID = client.Store.ID.String()
			}

			if err := s.repository.UpdateStatus(session.ID, models.SessionStatusConnected, phoneNumber, deviceJID); err != nil {
				s.logger.Errorf("Falha ao atualizar status: %v", err)
			}

		case *events.Disconnected:
			s.logger.Warnf("Sessão %s desconectada do WhatsApp", session.WhatsAppSessionKey)
			if err := s.repository.UpdateStatus(session.ID, models.SessionStatusDisconnected, "", ""); err != nil {
				s.logger.Errorf("Falha ao atualizar status: %v", err)
			}

		case *events.LoggedOut:
			s.logger.Warnf("Sessão %s fez logout", session.WhatsAppSessionKey)
			if client.Store != nil {
				if err := client.Store.Delete(context.Background()); err != nil {
					s.logger.Errorf("Falha ao apagar store local após logout: %v", err)
				}
			}
			if err := s.repository.MarkLoggedOut(session.ID); err != nil {
				s.logger.Errorf("Falha ao atualizar status após logout: %v", err)
			}
		}
	})
}

func (s *MultiTenantWhatsAppService) reconnectSession(session *models.WhatsAppSession) error {
	if session.PhoneNumber == "" {
		return fmt.Errorf("sessão não pode ser reconectada: phone_number ausente")
	}

	s.logger.Infof("Iniciando reconexão da sessão: %s (Phone: %s)", session.WhatsAppSessionKey, session.PhoneNumber)

	ctx := context.Background()

	var deviceStore *store.Device
	var err error

	if session.DeviceJID != "" {
		deviceJID, parseErr := types.ParseJID(session.DeviceJID)
		if parseErr != nil {
			s.logger.Warnf("Falha ao parse device_jid salvo (%s): %v", session.DeviceJID, parseErr)
		} else {
			s.logger.Infof("Buscando device pelo device_jid salvo: %s", deviceJID.String())
			deviceStore, err = s.container.GetDevice(ctx, deviceJID)
			if err != nil {
				s.logger.Warnf("GetDevice por device_jid falhou: %v", err)
			}
		}
	}

	if deviceStore == nil || deviceStore.ID == nil {
		jid, parseErr := types.ParseJID(session.PhoneNumber + "@s.whatsapp.net")
		if parseErr != nil {
			return fmt.Errorf("falha ao parse JID: %w", parseErr)
		}

		s.logger.Infof("Buscando device para JID: %s", jid.String())
		deviceStore, err = s.container.GetDevice(ctx, jid)
		if err != nil {
			s.logger.Warnf("GetDevice por JID falhou: %v", err)
		}
	}

	if deviceStore == nil || deviceStore.ID == nil {
		s.logger.Warnf("Device store não encontrado por JID. Tentando localizar por usuário %s", session.PhoneNumber)
		devices, listErr := s.container.GetAllDevices(ctx)
		if listErr != nil {
			s.logger.Warnf("Falha ao listar devices: %v", listErr)
		} else {
			for _, ds := range devices {
				if ds != nil && ds.ID != nil && ds.ID.User == session.PhoneNumber {
					deviceStore = ds
					break
				}
			}
		}
	}

	if deviceStore == nil || deviceStore.ID == nil {
		s.logger.Warnf("Device store não encontrado para sessão %s. Criando novo device (necessário novo QR code)", session.WhatsAppSessionKey)
		deviceStore = s.container.NewDevice()

		if err := s.repository.UpdateStatus(session.ID, models.SessionStatusPending, "", ""); err != nil {
			s.logger.Errorf("Falha ao atualizar status para pending: %v", err)
		}
	} else {
		s.logger.Infof("Device store encontrado para sessão %s (JID: %v)", session.WhatsAppSessionKey, deviceStore.ID)
	}

	waLogger := logger.NewWhatsAppLogger(fmt.Sprintf("[WA:%s] ", session.WhatsAppSessionKey), logger.INFO)
	client := whatsmeow.NewClient(deviceStore, waLogger)

	whatsappClient := &WhatsAppClient{
		Client:  client,
		Session: session,
		QRChan:  make(chan whatsmeow.QRChannelItem, 5),
	}

	s.mu.Lock()
	s.clients[session.WhatsAppSessionKey] = whatsappClient
	s.mu.Unlock()

	s.logger.Infof("Sessão %s adicionada ao map de clientes", session.WhatsAppSessionKey)

	s.registerEventHandlers(client, session)

	if client.Store.ID != nil {
		go func() {
			s.logger.Infof("Tentando conectar sessão %s...", session.WhatsAppSessionKey)
			if err := client.Connect(); err != nil {
				s.logger.Errorf("Falha ao reconectar sessão %s: %v", session.WhatsAppSessionKey, err)
				if updateErr := s.repository.UpdateStatus(session.ID, models.SessionStatusDisconnected, "", ""); updateErr != nil {
					s.logger.Errorf("Falha ao atualizar status após erro de reconexão: %v", updateErr)
				}
			} else {
				s.logger.Infof("Sessão %s reconectada com sucesso!", session.WhatsAppSessionKey)
			}
		}()
	} else {
		s.logger.Infof("Sessão %s não possui credenciais salvas. QR code necessário.", session.WhatsAppSessionKey)
	}

	return nil
}

func (s *MultiTenantWhatsAppService) GetQRCode(sessionKey string) (string, error) {
	s.mu.RLock()
	waClient, exists := s.clients[sessionKey]
	s.mu.RUnlock()

	if !exists {
		return "", fmt.Errorf("sessão não encontrada")
	}

	if waClient.Client.IsConnected() {
		return "", fmt.Errorf("sessão já está conectada")
	}

	if waClient.LastQRCode == "" {
		return "", fmt.Errorf("QR code ainda não foi gerado")
	}

	if time.Since(waClient.LastQRCodeTime) > 60*time.Second {
		return "", fmt.Errorf("QR code expirado, aguarde um novo")
	}

	return waClient.LastQRCode, nil
}

func (s *MultiTenantWhatsAppService) GetClient(sessionKey string) (*whatsmeow.Client, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	waClient, exists := s.clients[sessionKey]
	if !exists {
		return nil, fmt.Errorf("sessão não encontrada: %s", sessionKey)
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

	_, err = client.SendMessage(ctx, jid, &waProto.Message{
		ExtendedTextMessage: &waProto.ExtendedTextMessage{
			Text: proto.String(text),
		},
	})

	if err != nil {
		return fmt.Errorf("falha ao enviar mensagem: %w", err)
	}

	s.logger.Infof("[%s] Mensagem de texto enviada para %s", sessionKey, number)
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

	s.logger.Infof("[%s] Fazendo upload da mídia: tipo=%s, tamanho=%d bytes", sessionKey, contentType, len(mediaData))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	uploaded, err := client.Upload(ctx, mediaData, mediaType)
	if err != nil {
		return fmt.Errorf("falha ao fazer upload da mídia: %w", err)
	}

	s.logger.Infof("[%s] Mídia enviada com sucesso: URL=%s", sessionKey, uploaded.URL)

	message := s.buildMediaMessage(uploaded, mediaData, contentType, caption, filename)

	_, err = client.SendMessage(ctx, jid, message)
	if err != nil {
		return fmt.Errorf("falha ao enviar mensagem de mídia: %w", err)
	}

	s.logger.Infof("[%s] Mensagem de mídia enviada para %s", sessionKey, number)
	return nil
}

func (s *MultiTenantWhatsAppService) ListSessions() ([]*models.WhatsAppSession, error) {
	return s.repository.List()
}

func (s *MultiTenantWhatsAppService) DisconnectSession(sessionKey string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	waClient, exists := s.clients[sessionKey]
	if !exists {
		return fmt.Errorf("sessão não encontrada")
	}

	if waClient.CancelQR != nil {
		waClient.CancelQR()
	}

	if waClient.Client.IsConnected() {
		waClient.Client.Disconnect()
	}

	delete(s.clients, sessionKey)

	if err := s.repository.UpdateStatus(waClient.Session.ID, models.SessionStatusDisconnected, "", ""); err != nil {
		return err
	}

	s.logger.Infof("Sessão desconectada: %s", sessionKey)
	return nil
}

func (s *MultiTenantWhatsAppService) DeleteSession(sessionKey string) error {
	if err := s.DisconnectSession(sessionKey); err != nil {
		s.logger.Warnf("Erro ao desconectar sessão antes de deletar: %v", err)
	}

	session, err := s.repository.GetBySessionKey(sessionKey)
	if err != nil {
		return err
	}

	return s.repository.Delete(session.ID)
}

func (s *MultiTenantWhatsAppService) parsePhoneNumber(number string) (types.JID, error) {
	number = strings.TrimSpace(number)
	number = strings.ReplaceAll(number, " ", "")
	number = strings.ReplaceAll(number, "-", "")
	number = strings.ReplaceAll(number, "(", "")
	number = strings.ReplaceAll(number, ")", "")

	if !strings.HasSuffix(number, "@s.whatsapp.net") {
		if !strings.HasPrefix(number, s.config.WhatsApp.DefaultCountry) {
			number = s.config.WhatsApp.DefaultCountry + number
		}
		number = number + "@s.whatsapp.net"
	}

	jid, err := types.ParseJID(number)
	if err != nil {
		return types.JID{}, fmt.Errorf("número de telefone inválido: %w", err)
	}

	return jid, nil
}

func (s *MultiTenantWhatsAppService) prepareMedia(mediaURL, mediaBase64, mimeType string) ([]byte, string, string, error) {
	var mediaData []byte
	var contentType string
	var filename string
	var err error

	if mediaBase64 != "" {
		mediaData, contentType, err = s.decodeBase64Media(mediaBase64, mimeType)
		if err != nil {
			return nil, "", "", err
		}

		ext, _ := mime.ExtensionsByType(contentType)
		if len(ext) > 0 {
			filename = "media" + ext[0]
		} else {
			filename = "media"
		}
		filename = "media" + ext[0]

	} else if mediaURL != "" {
		mediaData, contentType, err = s.downloadMedia(mediaURL)
		if err != nil {
			return nil, "", "", err
		}

		ext := filepath.Ext(mediaURL)
		if ext == "" {
			exts, _ := mime.ExtensionsByType(contentType)
			if len(exts) > 0 {
				ext = exts[0]
			}
		}
		filename = "media" + ext

	} else {
		return nil, "", "", fmt.Errorf("é necessário fornecer media_url ou media_base64")
	}

	return mediaData, contentType, filename, nil
}

func (s *MultiTenantWhatsAppService) decodeBase64Media(base64Str, mimeType string) ([]byte, string, error) {
	base64Str = strings.TrimSpace(base64Str)

	if strings.HasPrefix(base64Str, "data:") {
		if idx := strings.Index(base64Str, ","); idx != -1 {
			base64Str = base64Str[idx+1:]
		}
	}

	base64Str = strings.ReplaceAll(base64Str, "\n", "")
	base64Str = strings.ReplaceAll(base64Str, "\r", "")
	base64Str = strings.TrimSpace(base64Str)

	mediaData, err := base64.StdEncoding.DecodeString(base64Str)
	if err != nil {
		return nil, "", fmt.Errorf("falha ao decodificar base64: %w", err)
	}

	contentType := mimeType
	if contentType == "" {
		contentType = http.DetectContentType(mediaData)
	}

	return mediaData, contentType, nil
}

func (s *MultiTenantWhatsAppService) downloadMedia(url string) ([]byte, string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, "", fmt.Errorf("falha ao criar requisição: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
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

	mediaData, err := io.ReadAll(io.LimitReader(resp.Body, s.config.Server.MaxUploadSize))
	if err != nil {
		return nil, "", fmt.Errorf("falha ao ler mídia: %w", err)
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = http.DetectContentType(mediaData)
	}

	return mediaData, contentType, nil
}

func (s *MultiTenantWhatsAppService) determineMediaType(contentType string) whatsmeow.MediaType {
	if strings.HasPrefix(contentType, "image/") {
		return whatsmeow.MediaImage
	} else if strings.HasPrefix(contentType, "video/") {
		return whatsmeow.MediaVideo
	} else if strings.HasPrefix(contentType, "audio/") {
		return whatsmeow.MediaAudio
	}
	return whatsmeow.MediaDocument
}

func (s *MultiTenantWhatsAppService) buildMediaMessage(uploaded whatsmeow.UploadResponse, mediaData []byte, contentType, caption, filename string) *waProto.Message {
	mediaType := s.determineMediaType(contentType)

	switch mediaType {
	case whatsmeow.MediaImage:
		return &waProto.Message{
			ImageMessage: &waProto.ImageMessage{
				URL:           proto.String(uploaded.URL),
				DirectPath:    proto.String(uploaded.DirectPath),
				MediaKey:      uploaded.MediaKey,
				Mimetype:      proto.String(contentType),
				FileEncSHA256: uploaded.FileEncSHA256,
				FileSHA256:    uploaded.FileSHA256,
				FileLength:    proto.Uint64(uint64(len(mediaData))),
				Caption:       proto.String(caption),
			},
		}
	case whatsmeow.MediaVideo:
		return &waProto.Message{
			VideoMessage: &waProto.VideoMessage{
				URL:           proto.String(uploaded.URL),
				DirectPath:    proto.String(uploaded.DirectPath),
				MediaKey:      uploaded.MediaKey,
				Mimetype:      proto.String(contentType),
				FileEncSHA256: uploaded.FileEncSHA256,
				FileSHA256:    uploaded.FileSHA256,
				FileLength:    proto.Uint64(uint64(len(mediaData))),
				Caption:       proto.String(caption),
			},
		}
	case whatsmeow.MediaAudio:
		return &waProto.Message{
			AudioMessage: &waProto.AudioMessage{
				URL:           proto.String(uploaded.URL),
				DirectPath:    proto.String(uploaded.DirectPath),
				MediaKey:      uploaded.MediaKey,
				Mimetype:      proto.String(contentType),
				FileEncSHA256: uploaded.FileEncSHA256,
				FileSHA256:    uploaded.FileSHA256,
				FileLength:    proto.Uint64(uint64(len(mediaData))),
			},
		}
	default:
		return &waProto.Message{
			DocumentMessage: &waProto.DocumentMessage{
				URL:           proto.String(uploaded.URL),
				DirectPath:    proto.String(uploaded.DirectPath),
				MediaKey:      uploaded.MediaKey,
				Mimetype:      proto.String(contentType),
				FileEncSHA256: uploaded.FileEncSHA256,
				FileSHA256:    uploaded.FileSHA256,
				FileLength:    proto.Uint64(uint64(len(mediaData))),
				FileName:      proto.String(filename),
				Caption:       proto.String(caption),
			},
		}
	}
}

func (s *MultiTenantWhatsAppService) Shutdown() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.logger.Info("Desconectando todas as sessões...")
	for key, waClient := range s.clients {
		if waClient.CancelQR != nil {
			waClient.CancelQR()
		}
		if waClient.Client != nil && waClient.Client.IsConnected() {
			waClient.Client.Disconnect()
		}
		s.logger.Infof("Sessão desconectada: %s", key)
	}
	s.clients = make(map[string]*WhatsAppClient)
}
