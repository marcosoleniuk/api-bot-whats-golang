package services

import (
	"boot-whatsapp-golang/internal/config"
	"boot-whatsapp-golang/internal/models"
	"boot-whatsapp-golang/internal/repository"
	"boot-whatsapp-golang/pkg/logger"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
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
	"go.mau.fi/util/random"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waCommon"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"go.mau.fi/whatsmeow/util/gcmutil"
	"go.mau.fi/whatsmeow/util/hkdfutil"
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

	config          *config.Config
	logger          *logger.Logger
	repository      *repository.SessionRepository
	container       *sqlstore.Container
	webhookService  *WebhookService
	messageService  *MessageService
	realtimeService *RealtimeService
	pollRepo        *repository.PollMetadataRepository

	httpClient *http.Client

	pendingEventFallbackMu sync.Mutex
	pendingEventFallback   map[string]pendingEventFallback

	pollMetadataMu sync.RWMutex
	pollMetadata   map[string]pollMetadata
}

type pendingEventFallback struct {
	chat      types.JID
	text      string
	expiresAt time.Time
}

type pollMetadata struct {
	question         string
	options          []string
	optionHashToName map[string]string
	expiresAt        time.Time
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
		clients:              newClientStore(),
		config:               cfg,
		logger:               log,
		repository:           repo,
		container:            container,
		httpClient:           httpClient,
		pendingEventFallback: make(map[string]pendingEventFallback),
		pollMetadata:         make(map[string]pollMetadata),
		pollRepo:             repository.NewPollMetadataRepository(db, log),
	}

	if err := service.LoadExistingSessions(); err != nil {
		log.Warnf("Falha ao carregar sessões existentes: %v", err)
	}

	return service, nil
}

func (s *MultiTenantWhatsAppService) SetWebhookService(ws *WebhookService) {
	s.webhookService = ws
}

func (s *MultiTenantWhatsAppService) SetMessageService(ms *MessageService) {
	s.messageService = ms
}

func (s *MultiTenantWhatsAppService) SetRealtimeService(rs *RealtimeService) {
	s.realtimeService = rs
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
		switch event := evt.(type) {
		case *events.Connected:
			phoneNumber := ""
			deviceJID := ""
			if client.Store != nil && client.Store.ID != nil {
				phoneNumber = client.Store.ID.User
				deviceJID = client.Store.ID.String()
			}
			_ = s.repository.UpdateStatus(session.ID, models.SessionStatusConnected, phoneNumber, deviceJID)
			s.publishRealtimeEvent(session, "session.connected", map[string]interface{}{
				"phone_number": phoneNumber,
				"device_jid":   deviceJID,
				"status":       models.SessionStatusConnected,
			})

		case *events.Disconnected:
			_ = s.repository.UpdateStatus(session.ID, models.SessionStatusDisconnected, "", "")
			s.publishRealtimeEvent(session, "session.disconnected", map[string]interface{}{
				"status": models.SessionStatusDisconnected,
			})

		case *events.LoggedOut:
			if client.Store != nil {
				_ = client.Store.Delete(context.Background())
			}
			_ = s.repository.MarkLoggedOut(session.ID)
			s.publishRealtimeEvent(session, "session.logged_out", map[string]interface{}{
				"status": models.SessionStatusPending,
			})

		case *events.Receipt:
			s.logger.Infof("[%s] Receipt event recebido: Type=%s, %+v", session.WhatsAppSessionKey, event.Type, event)
			receiptMessageIDs := make([]string, 0, len(event.MessageIDs))
			for _, msgID := range event.MessageIDs {
				receiptMessageIDs = append(receiptMessageIDs, fmt.Sprintf("%v", msgID))
			}
			primaryMessageID := ""
			if len(receiptMessageIDs) > 0 {
				primaryMessageID = receiptMessageIDs[0]
			}

			if event.Type == "read" && s.webhookService != nil {
				webhookData := &models.MessageWebhookData{
					MessageID:   primaryMessageID,
					MessageIDs:  receiptMessageIDs,
					ReceiptType: string(event.Type),
					Chat:        event.MessageSource.Chat.String(),
					IsGroup:     proto.Bool(event.MessageSource.IsGroup),
					From:        event.MessageSource.Sender.String(),
					Type:        "receipt",
					Text:        "Message read receipt",
					Timestamp:   time.Now(),
				}
				go s.webhookService.TriggerWebhooks(session.ID, session.WhatsAppSessionKey, "message.read", webhookData, session.WhatsAppSessionKey)
			}
			if s.messageService != nil && len(event.MessageIDs) > 0 {
				for _, msgID := range event.MessageIDs {
					status := "sent"
					if event.Type == "read" {
						status = "read"
					} else if event.Type == "" {
						status = "delivered"
					}
					_ = s.messageService.UpdateMessageStatus(msgID, status)
				}
			}

			if event.Type == "retry" && len(event.MessageIDs) > 0 {
				for _, msgID := range event.MessageIDs {
					s.logger.Warnf(
						"[%s] Retry recebido para mensagem %s (destino não confirmou decodificação do payload nativo em chat direto)",
						session.WhatsAppSessionKey,
						msgID,
					)
				}
			}

			s.publishRealtimeEvent(session, "message.receipt", map[string]interface{}{
				"type":        event.Type,
				"message_id":  primaryMessageID,
				"message_ids": receiptMessageIDs,
				"chat":        event.MessageSource.Chat.String(),
				"sender":      event.MessageSource.Sender.String(),
				"is_from_me":  event.MessageSource.IsFromMe,
				"is_group":    event.MessageSource.IsGroup,
				"timestamp":   event.Timestamp,
			})

		case *events.Message:
			if s.handlePollVoteIfPresent(client, session, event) {
				return
			}

			var messageType string
			var messageContent string
			var mediaURL string
			var mimeType string
			var mediaBytes []byte
			var pollQuestion string
			var pollOptions []string

			if msg := event.Message; msg != nil {
				if pollMsg := extractPollCreationMessage(msg); pollMsg != nil {
					messageType = "poll"
					pollQuestion = strings.TrimSpace(pollMsg.GetName())
					messageContent = pollQuestion
					pollOptions = extractPollOptionNames(pollMsg)
					s.setPollMetadataForSession(session, session.WhatsAppSessionKey, event.Info.Chat, event.Info.ID, pollQuestion, pollOptions)
				} else if msg.GetConversation() != "" {
					messageType = "text"
					messageContent = msg.GetConversation()
				} else if extMsg := msg.GetExtendedTextMessage(); extMsg != nil {
					messageType = "text"
					messageContent = extMsg.GetText()
				} else if imgMsg := msg.GetImageMessage(); imgMsg != nil {
					messageType = "image"
					messageContent = imgMsg.GetCaption()
					if imgMsg.GetURL() != "" {
						mediaURL = imgMsg.GetURL()
					}
					if imgMsg.GetMimetype() != "" {
						mimeType = imgMsg.GetMimetype()
					}
				} else if docMsg := msg.GetDocumentMessage(); docMsg != nil {
					messageType = "document"
					messageContent = docMsg.GetCaption()
					if docMsg.GetURL() != "" {
						mediaURL = docMsg.GetURL()
					}
					if docMsg.GetMimetype() != "" {
						mimeType = docMsg.GetMimetype()
					}
				} else if vidMsg := msg.GetVideoMessage(); vidMsg != nil {
					messageType = "video"
					messageContent = vidMsg.GetCaption()
					if vidMsg.GetURL() != "" {
						mediaURL = vidMsg.GetURL()
					}
					if vidMsg.GetMimetype() != "" {
						mimeType = vidMsg.GetMimetype()
					}
				} else if audMsg := msg.GetAudioMessage(); audMsg != nil {
					messageType = "audio"
					if audMsg.GetURL() != "" {
						mediaURL = audMsg.GetURL()
					}
					if audMsg.GetMimetype() != "" {
						mimeType = audMsg.GetMimetype()
					}
				}
			}

			if messageType == "" {
				return
			}

			s.logger.Infof("[%s] Message event recebido de %s (type=%s)", session.WhatsAppSessionKey, event.Info.Sender, messageType)
			messageID := event.Info.ID
			sender := event.Info.Sender.String()

			if s.messageService != nil {
				var err error
				if mediaURL != "" {
					if downloaded, dlErr := s.downloadInboundMedia(client, event.Message); dlErr != nil {
						s.logger.Warnf("[%s] Falha ao baixar/decriptar mídia %s: %v", session.WhatsAppSessionKey, messageID, dlErr)
					} else {
						mediaBytes = downloaded
						s.logger.Infof("[%s] Mídia decriptada armazenável para %s (size=%d bytes)", session.WhatsAppSessionKey, messageID, len(mediaBytes))
					}

					fmt.Printf("[%s] Salvando mídia imediatamente - URL: %s\n", session.WhatsAppSessionKey, mediaURL)
					_, err = s.messageService.SaveMediaMessage(
						session.ID,
						session.WhatsAppSessionKey,
						messageID,
						"inbound",
						sender,
						messageType,
						mediaURL,
						mediaBytes,
						mimeType,
						messageContent,
					)
				} else {
					_, err = s.messageService.SaveInboundMessage(
						session.ID,
						session.WhatsAppSessionKey,
						messageID,
						sender,
						messageContent,
						messageType,
					)
				}
				if err != nil {
					s.logger.Warnf("[%s] Falha ao salvar mensagem recebida: %v", session.WhatsAppSessionKey, err)
				}

				if s.webhookService != nil {
					webhookData := &models.MessageWebhookData{
						MessageID: messageID,
						Chat:      event.Info.Chat.String(),
						IsGroup:   proto.Bool(event.Info.IsGroup),
						From:      sender,
						Type:      messageType,
						Text:      messageContent,
						Timestamp: time.Now(),
					}
					if messageType == "poll" {
						webhookData.PollID = messageID
						webhookData.PollName = pollQuestion
						webhookData.PollOptions = append([]string(nil), pollOptions...)
					}
					go s.webhookService.TriggerWebhooks(session.ID, session.WhatsAppSessionKey, "message.received", webhookData, session.WhatsAppSessionKey)
				}
			}

			realtimeData := &models.MessageWebhookData{
				MessageID: messageID,
				Chat:      event.Info.Chat.String(),
				IsGroup:   proto.Bool(event.Info.IsGroup),
				From:      sender,
				Type:      messageType,
				Text:      messageContent,
				MediaURL:  mediaURL,
				MimeType:  mimeType,
				Timestamp: time.Now(),
			}
			if messageType == "poll" {
				realtimeData.PollID = messageID
				realtimeData.PollName = pollQuestion
				realtimeData.PollOptions = append([]string(nil), pollOptions...)
			}
			s.publishRealtimeEvent(session, "message.received", realtimeData)

		default:
			s.logger.Infof("[%s] Evento desconhecido recebido: %T", session.WhatsAppSessionKey, evt)
		}
	})
}

func (s *MultiTenantWhatsAppService) handlePollVoteIfPresent(client *whatsmeow.Client, session *models.WhatsAppSession, event *events.Message) bool {
	if client == nil || session == nil || event == nil || event.Message == nil {
		return false
	}

	pollUpdate := event.Message.GetPollUpdateMessage()
	if pollUpdate == nil {
		return false
	}

	pollID := ""
	pollChat := event.Info.Chat
	pollKey := pollUpdate.GetPollCreationMessageKey()
	if pollKey != nil {
		pollID = strings.TrimSpace(pollKey.GetID())
		if remoteJID := strings.TrimSpace(pollKey.GetRemoteJID()); remoteJID != "" {
			if parsed, err := types.ParseJID(remoteJID); err == nil {
				pollChat = parsed
			}
		}
	}

	var selectedOptions []string
	var selectedHashes []string
	var pollName string

	pollVote, err := s.decryptPollVoteWithFallback(client, event, pollID, pollChat)
	if err != nil {
		s.logger.Warnf("[%s] Falha ao decriptar voto de enquete %s: %v", session.WhatsAppSessionKey, event.Info.ID, err)
	} else {
		selectedOptions, selectedHashes, pollName = s.resolvePollVote(session, pollChat, pollID, pollVote.GetSelectedOptions())
	}

	meta, hasMeta := s.getPollMetadata(session.WhatsAppSessionKey, pollChat, pollID)
	if pollName == "" && hasMeta {
		pollName = meta.question
	}

	voteText := "Poll vote update"
	if len(selectedOptions) > 0 {
		voteText = fmt.Sprintf("Poll vote: %s", strings.Join(selectedOptions, ", "))
	} else if len(selectedHashes) > 0 {
		voteText = fmt.Sprintf("Poll vote hashes: %s", strings.Join(selectedHashes, ", "))
	}

	if s.messageService != nil {
		_, saveErr := s.messageService.SaveInboundMessage(
			session.ID,
			session.WhatsAppSessionKey,
			event.Info.ID,
			event.Info.Sender.String(),
			voteText,
			"unknown",
		)
		if saveErr != nil {
			s.logger.Warnf("[%s] Falha ao salvar voto de enquete no banco: %v", session.WhatsAppSessionKey, saveErr)
		}
	}

	webhookData := &models.MessageWebhookData{
		MessageID: event.Info.ID,
		Chat:      event.Info.Chat.String(),
		IsGroup:   proto.Bool(event.Info.IsGroup),
		PollID:    pollID,
		PollName:  pollName,
		PollVotes: append([]string(nil), selectedOptions...),
		PollHashes: append([]string(nil),
			selectedHashes...),
		From:      event.Info.Sender.String(),
		Type:      "poll_vote",
		Text:      voteText,
		Timestamp: time.Now(),
	}
	if hasMeta {
		webhookData.PollOptions = append([]string(nil), meta.options...)
	}

	if s.webhookService != nil {
		go s.webhookService.TriggerWebhooks(session.ID, session.WhatsAppSessionKey, "message.received", webhookData, session.WhatsAppSessionKey)
	}
	s.publishRealtimeEvent(session, "message.received", webhookData)
	return true
}

func (s *MultiTenantWhatsAppService) decryptPollVoteWithFallback(client *whatsmeow.Client, event *events.Message, pollID string, pollChat types.JID) (*waE2E.PollVoteMessage, error) {
	if client == nil || event == nil || event.Message == nil {
		return nil, fmt.Errorf("evento de voto inválido")
	}

	decryptCtx, cancelDecrypt := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelDecrypt()

	pollVote, err := client.DecryptPollVote(decryptCtx, event)
	if err == nil {
		return pollVote, nil
	}

	pollID = strings.TrimSpace(pollID)
	pollUpdate := event.Message.GetPollUpdateMessage()
	if pollID == "" || pollUpdate == nil || pollUpdate.GetVote() == nil {
		return nil, err
	}

	candidateChats := s.buildPollChatCandidates(decryptCtx, client, event, pollChat, pollUpdate.GetPollCreationMessageKey())
	candidateSenders := s.buildPollSenderCandidates(decryptCtx, client, event, pollUpdate.GetPollCreationMessageKey())
	candidateModificationSenders := s.buildPollModificationSenderCandidates(decryptCtx, client, event, pollUpdate.GetPollCreationMessageKey())
	candidatePollIDs := buildPollIDCandidates(pollID, pollUpdate.GetPollCreationMessageKey(), event)

	vote := pollUpdate.GetVote()
	tried := 0
	for _, chatCandidate := range candidateChats {
		for _, senderCandidate := range candidateSenders {
			for _, pollIDCandidate := range candidatePollIDs {
				if decryptCtx.Err() != nil {
					return nil, fmt.Errorf("%w (fallback interrompido: %v)", err, decryptCtx.Err())
				}

				secret, realSender, getSecretErr := client.Store.MsgSecrets.GetMessageSecret(decryptCtx, chatCandidate, senderCandidate, pollIDCandidate)
				if getSecretErr != nil || len(secret) == 0 {
					continue
				}

				originalSenderCandidates := s.buildPollOriginalSenderCandidates(decryptCtx, client, realSender, senderCandidate)
				for _, originalSender := range originalSenderCandidates {
					for _, modificationSender := range candidateModificationSenders {
						tried++
						decryptedVote, decryptErr := decryptPollVoteWithSecret(
							secret,
							originalSender,
							modificationSender,
							pollIDCandidate,
							vote.GetEncPayload(),
							vote.GetEncIV(),
						)
						if decryptErr != nil {
							continue
						}
						return decryptedVote, nil
					}
				}
			}
		}
	}

	if tried == 0 {
		return nil, fmt.Errorf("%w (nenhum segredo compatível encontrado para fallback)", err)
	}
	return nil, fmt.Errorf(
		"%w (fallback testou %d combinação(ões) sem sucesso; chats=%d, senders=%d, mod_senders=%d, poll_ids=%d)",
		err,
		tried,
		len(candidateChats),
		len(candidateSenders),
		len(candidateModificationSenders),
		len(candidatePollIDs),
	)
}

func decryptPollVoteWithSecret(baseSecret []byte, originalSender, modificationSender types.JID, pollID string, encPayload, encIV []byte) (*waE2E.PollVoteMessage, error) {
	if len(baseSecret) == 0 {
		return nil, fmt.Errorf("segredo base vazio")
	}
	if strings.TrimSpace(pollID) == "" {
		return nil, fmt.Errorf("pollID vazio")
	}

	origSenderStr := originalSender.ToNonAD().String()
	modSenderStr := modificationSender.ToNonAD().String()

	useCaseSecret := make([]byte, 0, len(pollID)+len(origSenderStr)+len(modSenderStr)+len("Poll Vote"))
	useCaseSecret = append(useCaseSecret, pollID...)
	useCaseSecret = append(useCaseSecret, origSenderStr...)
	useCaseSecret = append(useCaseSecret, modSenderStr...)
	useCaseSecret = append(useCaseSecret, "Poll Vote"...)

	secretKey := hkdfutil.SHA256(baseSecret, nil, useCaseSecret, 32)
	additionalData := fmt.Appendf(nil, "%s\x00%s", pollID, modSenderStr)

	plaintext, err := gcmutil.Decrypt(secretKey, encIV, encPayload, additionalData)
	if err != nil {
		return nil, err
	}

	var vote waE2E.PollVoteMessage
	if err := proto.Unmarshal(plaintext, &vote); err != nil {
		return nil, fmt.Errorf("falha ao decodificar voto de enquete: %w", err)
	}
	return &vote, nil
}

func (s *MultiTenantWhatsAppService) buildPollChatCandidates(ctx context.Context, client *whatsmeow.Client, event *events.Message, pollChat types.JID, key *waCommon.MessageKey) []types.JID {
	seen := make(map[string]struct{}, 8)
	candidates := make([]types.JID, 0, 8)

	candidates = addJIDCandidate(candidates, seen, event.Info.Chat)
	candidates = addJIDCandidate(candidates, seen, pollChat)
	if key != nil {
		if parsed, parseErr := types.ParseJID(strings.TrimSpace(key.GetRemoteJID())); parseErr == nil {
			candidates = addJIDCandidate(candidates, seen, parsed)
		}
	}

	baseLen := len(candidates)
	for i := 0; i < baseLen; i++ {
		alt, altErr := client.Store.GetAltJID(ctx, candidates[i])
		if altErr == nil {
			candidates = addJIDCandidate(candidates, seen, alt)
		}
	}

	return candidates
}

func (s *MultiTenantWhatsAppService) buildPollSenderCandidates(ctx context.Context, client *whatsmeow.Client, event *events.Message, key *waCommon.MessageKey) []types.JID {
	seen := make(map[string]struct{}, 12)
	candidates := make([]types.JID, 0, 12)

	candidates = addJIDCandidate(candidates, seen, event.Info.Sender)
	candidates = addJIDCandidate(candidates, seen, event.Info.SenderAlt)
	candidates = addJIDCandidate(candidates, seen, event.Info.Chat)
	candidates = addJIDCandidate(candidates, seen, client.Store.GetJID())
	candidates = addJIDCandidate(candidates, seen, client.Store.GetLID())

	if key != nil {
		if parsed, parseErr := types.ParseJID(strings.TrimSpace(key.GetParticipant())); parseErr == nil {
			candidates = addJIDCandidate(candidates, seen, parsed)
		}
		if parsed, parseErr := types.ParseJID(strings.TrimSpace(key.GetRemoteJID())); parseErr == nil {
			candidates = addJIDCandidate(candidates, seen, parsed)
		}
	}

	baseLen := len(candidates)
	for i := 0; i < baseLen; i++ {
		alt, altErr := client.Store.GetAltJID(ctx, candidates[i])
		if altErr == nil {
			candidates = addJIDCandidate(candidates, seen, alt)
		}
	}

	return candidates
}

func (s *MultiTenantWhatsAppService) buildPollModificationSenderCandidates(ctx context.Context, client *whatsmeow.Client, event *events.Message, key *waCommon.MessageKey) []types.JID {
	seen := make(map[string]struct{}, 12)
	candidates := make([]types.JID, 0, 12)

	candidates = addJIDCandidate(candidates, seen, event.Info.Sender)
	candidates = addJIDCandidate(candidates, seen, event.Info.SenderAlt)
	candidates = addJIDCandidate(candidates, seen, event.Info.Chat)

	if key != nil {
		if parsed, parseErr := types.ParseJID(strings.TrimSpace(key.GetParticipant())); parseErr == nil {
			candidates = addJIDCandidate(candidates, seen, parsed)
		}
		if parsed, parseErr := types.ParseJID(strings.TrimSpace(key.GetRemoteJID())); parseErr == nil {
			candidates = addJIDCandidate(candidates, seen, parsed)
		}
	}

	baseLen := len(candidates)
	for i := 0; i < baseLen; i++ {
		alt, altErr := client.Store.GetAltJID(ctx, candidates[i])
		if altErr == nil {
			candidates = addJIDCandidate(candidates, seen, alt)
		}
	}

	return candidates
}

func (s *MultiTenantWhatsAppService) buildPollOriginalSenderCandidates(ctx context.Context, client *whatsmeow.Client, realSender, requestedSender types.JID) []types.JID {
	seen := make(map[string]struct{}, 8)
	candidates := make([]types.JID, 0, 8)

	candidates = addJIDCandidate(candidates, seen, realSender)
	candidates = addJIDCandidate(candidates, seen, requestedSender)

	baseLen := len(candidates)
	for i := 0; i < baseLen; i++ {
		alt, altErr := client.Store.GetAltJID(ctx, candidates[i])
		if altErr == nil {
			candidates = addJIDCandidate(candidates, seen, alt)
		}
	}

	return candidates
}

func buildPollIDCandidates(primaryPollID string, key *waCommon.MessageKey, event *events.Message) []string {
	seen := make(map[string]struct{}, 4)
	candidates := make([]string, 0, 4)

	add := func(value string) {
		v := strings.TrimSpace(value)
		if v == "" {
			return
		}
		if _, exists := seen[v]; exists {
			return
		}
		seen[v] = struct{}{}
		candidates = append(candidates, v)
	}

	add(primaryPollID)
	if key != nil {
		add(key.GetID())
	}
	if event != nil {
		add(string(event.Info.MsgMetaInfo.TargetID))
		add(event.Info.ID)
	}

	return candidates
}

func addJIDCandidate(candidates []types.JID, seen map[string]struct{}, jid types.JID) []types.JID {
	if jid.IsEmpty() {
		return candidates
	}
	normalized := jid.ToNonAD()
	key := strings.TrimSpace(normalized.String())
	if key == "" {
		return candidates
	}
	if _, exists := seen[key]; exists {
		return candidates
	}
	seen[key] = struct{}{}
	return append(candidates, normalized)
}

func (s *MultiTenantWhatsAppService) publishRealtimeEvent(session *models.WhatsAppSession, eventType string, data interface{}) {
	if s.realtimeService == nil || session == nil {
		return
	}

	sessionKey := strings.TrimSpace(session.WhatsAppSessionKey)
	tenantID := strings.TrimSpace(session.TenantID)

	s.realtimeService.Publish(
		eventType,
		sessionKey,
		tenantID,
		data,
		sessionKey,
		tenantID,
	)
}

func (s *MultiTenantWhatsAppService) downloadInboundMedia(client *whatsmeow.Client, msg *waE2E.Message) ([]byte, error) {
	if client == nil || msg == nil {
		return nil, fmt.Errorf("cliente ou mensagem inválida")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	data, err := client.DownloadAny(ctx, msg)
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("mídia vazia")
	}

	return data, nil
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

func (s *MultiTenantWhatsAppService) SendTextMessage(sessionKey, number, text string) (string, error) {
	client, err := s.GetClient(sessionKey)
	if err != nil {
		return "", err
	}

	jid, err := s.parsePhoneNumber(number)
	if err != nil {
		return "", err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := client.SendMessage(ctx, jid, &waE2E.Message{
		ExtendedTextMessage: &waE2E.ExtendedTextMessage{
			Text: proto.String(text),
		},
	})
	if err != nil {
		return "", fmt.Errorf("falha ao enviar mensagem: %w", err)
	}

	messageID := resp.ID
	if messageID == "" {
		messageID = "unknown"
	}

	return messageID, nil
}

func (s *MultiTenantWhatsAppService) SendMediaMessage(sessionKey, number, caption, mediaURL, mediaBase64, mimeType string) (string, string, string, []byte, error) {
	client, err := s.GetClient(sessionKey)
	if err != nil {
		return "", "", "", nil, err
	}

	jid, err := s.parsePhoneNumber(number)
	if err != nil {
		return "", "", "", nil, err
	}

	mediaData, contentType, filename, err := s.prepareMedia(mediaURL, mediaBase64, mimeType)
	if err != nil {
		return "", "", "", nil, err
	}

	mediaType := s.determineMediaType(contentType)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	uploaded, err := client.Upload(ctx, mediaData, mediaType)
	if err != nil {
		return "", "", "", nil, fmt.Errorf("falha ao fazer upload da mídia: %w", err)
	}

	msg := s.buildMediaMessage(uploaded, mediaData, contentType, caption, filename)

	resp, err := client.SendMessage(ctx, jid, msg)
	if err != nil {
		return "", "", "", nil, fmt.Errorf("falha ao enviar mensagem de mídia: %w", err)
	}

	messageID := resp.ID
	if messageID == "" {
		messageID = "unknown"
	}

	return messageID, contentType, mediaTypeToMessageType(mediaType), mediaData, nil
}

func (s *MultiTenantWhatsAppService) SendAudioMessage(sessionKey, number, mediaURL, mediaBase64, mimeType string, ptt bool) (string, string, []byte, error) {
	client, err := s.GetClient(sessionKey)
	if err != nil {
		return "", "", nil, err
	}

	jid, err := s.parsePhoneNumber(number)
	if err != nil {
		return "", "", nil, err
	}

	prepareStartedAt := time.Now()
	mediaData, contentType, _, err := s.prepareMedia(mediaURL, mediaBase64, mimeType)
	if err != nil {
		return "", "", nil, err
	}
	contentType = normalizeExpectedContentType(contentType, mimeType, "audio/")
	if contentType == "" {
		contentType = "audio/mpeg"
	}
	s.logger.Infof("[%s] Áudio preparado (bytes=%d, mime=%s) em %v", sessionKey, len(mediaData), contentType, time.Since(prepareStartedAt))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	uploadStartedAt := time.Now()
	uploaded, err := client.Upload(ctx, mediaData, whatsmeow.MediaAudio)
	if err != nil {
		return "", "", nil, fmt.Errorf("falha ao fazer upload do áudio: %w", err)
	}
	s.logger.Infof("[%s] Upload de áudio concluído em %v", sessionKey, time.Since(uploadStartedAt))

	msg := &waE2E.Message{
		AudioMessage: &waE2E.AudioMessage{
			URL:           proto.String(uploaded.URL),
			DirectPath:    proto.String(uploaded.DirectPath),
			MediaKey:      uploaded.MediaKey,
			Mimetype:      proto.String(contentType),
			FileEncSHA256: uploaded.FileEncSHA256,
			FileSHA256:    uploaded.FileSHA256,
			FileLength:    proto.Uint64(uint64(len(mediaData))),
			PTT:           proto.Bool(ptt),
		},
	}

	sendStartedAt := time.Now()
	resp, err := client.SendMessage(ctx, jid, msg)
	if err != nil {
		return "", "", nil, fmt.Errorf("falha ao enviar áudio: %w", err)
	}
	s.logger.Infof("[%s] SendMessage de áudio concluído em %v (id=%s)", sessionKey, time.Since(sendStartedAt), resp.ID)

	messageID := resp.ID
	if messageID == "" {
		messageID = "unknown"
	}

	return messageID, contentType, mediaData, nil
}

func (s *MultiTenantWhatsAppService) SendVideoMessage(sessionKey, number, caption, mediaURL, mediaBase64, mimeType string, gifPlayback bool) (string, string, []byte, error) {
	client, err := s.GetClient(sessionKey)
	if err != nil {
		return "", "", nil, err
	}

	jid, err := s.parsePhoneNumber(number)
	if err != nil {
		return "", "", nil, err
	}

	mediaData, contentType, _, err := s.prepareMedia(mediaURL, mediaBase64, mimeType)
	if err != nil {
		return "", "", nil, err
	}
	contentType = normalizeExpectedContentType(contentType, mimeType, "video/")
	if contentType == "" {
		contentType = "video/mp4"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	uploaded, err := client.Upload(ctx, mediaData, whatsmeow.MediaVideo)
	if err != nil {
		return "", "", nil, fmt.Errorf("falha ao fazer upload do vídeo: %w", err)
	}

	msg := &waE2E.Message{
		VideoMessage: &waE2E.VideoMessage{
			URL:           proto.String(uploaded.URL),
			DirectPath:    proto.String(uploaded.DirectPath),
			MediaKey:      uploaded.MediaKey,
			Mimetype:      proto.String(contentType),
			FileEncSHA256: uploaded.FileEncSHA256,
			FileSHA256:    uploaded.FileSHA256,
			FileLength:    proto.Uint64(uint64(len(mediaData))),
			Caption:       proto.String(caption),
			GifPlayback:   proto.Bool(gifPlayback),
		},
	}

	resp, err := client.SendMessage(ctx, jid, msg)
	if err != nil {
		return "", "", nil, fmt.Errorf("falha ao enviar vídeo: %w", err)
	}

	messageID := resp.ID
	if messageID == "" {
		messageID = "unknown"
	}

	return messageID, contentType, mediaData, nil
}

func (s *MultiTenantWhatsAppService) SendPollMessage(sessionKey, number, question string, options []string, selectableOptionsCount int) (string, error) {
	client, err := s.GetClient(sessionKey)
	if err != nil {
		return "", err
	}

	jid, err := s.parsePhoneNumber(number)
	if err != nil {
		return "", err
	}

	msg := client.BuildPollCreation(question, options, selectableOptionsCount)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := client.SendMessage(ctx, jid, msg)
	if err != nil {
		return "", fmt.Errorf("falha ao enviar enquete: %w", err)
	}

	messageID := resp.ID
	if messageID == "" {
		messageID = "unknown"
	}
	s.setPollMetadata(sessionKey, jid, messageID, question, options)

	return messageID, nil
}

func (s *MultiTenantWhatsAppService) SendEventMessage(
	sessionKey, number, name, description, joinLink string,
	startTime, endTime int64,
	extraGuestsAllowed, isScheduleCall, hasReminder bool,
	reminderOffsetSec int64,
	callType string,
	forceStructured bool,
) (string, error) {
	client, err := s.GetClient(sessionKey)
	if err != nil {
		return "", err
	}

	jid, err := s.parsePhoneNumber(number)
	if err != nil {
		return "", err
	}

	isDirectChat := jid.Server == types.DefaultUserServer
	sendMessageWithTimeout := func(timeout time.Duration, msg *waE2E.Message) (whatsmeow.SendResponse, error) {
		sendCtx, sendCancel := context.WithTimeout(context.Background(), timeout)
		defer sendCancel()
		return client.SendMessage(sendCtx, jid, msg)
	}

	if isDirectChat && !forceStructured {
		fallbackText := buildEventFallbackText(name, description, joinLink, startTime, endTime)
		msg := &waE2E.Message{
			ExtendedTextMessage: &waE2E.ExtendedTextMessage{
				Text: proto.String(fallbackText),
			},
		}

		resp, err := sendMessageWithTimeout(30*time.Second, msg)
		if err != nil {
			return "", fmt.Errorf("falha ao enviar fallback textual de evento: %w", err)
		}

		s.logger.Warnf("[%s] Evento enviado como texto por compatibilidade em chat direto", sessionKey)
		s.logger.Infof(
			"[%s] Evento fallback enviado para %s (id=%s, server_ts=%s, server_id=%d)",
			sessionKey,
			number,
			resp.ID,
			resp.Timestamp.Format(time.RFC3339),
			resp.ServerID,
		)

		messageID := resp.ID
		if messageID == "" {
			messageID = "unknown"
		}
		return messageID, nil
	}

	finalJoinLink := strings.TrimSpace(joinLink)
	normalizedCallType := strings.ToLower(strings.TrimSpace(callType))

	if normalizedCallType == "" && isDirectChat && forceStructured && finalJoinLink != "" && !isWhatsAppCallLink(finalJoinLink) {
		return "", fmt.Errorf("join_link externo não gera cartão nativo em chat direto; informe call_type ('video' ou 'voice') sem join_link, ou use link call.whatsapp.com")
	}

	if normalizedCallType != "" {
		var mediaType whatsmeow.CallLinkType
		var callLinkPrefix string
		switch normalizedCallType {
		case "video":
			mediaType = whatsmeow.CallLinkTypeVideo
			callLinkPrefix = whatsmeow.CallLinkVideoPrefix
		case "voice":
			mediaType = whatsmeow.CallLinkTypeAudio
			callLinkPrefix = whatsmeow.CallLinkAudioPrefix
		default:
			return "", fmt.Errorf("call_type inválido: %s", callType)
		}

		if finalJoinLink == "" || !isWhatsAppCallLink(finalJoinLink) {
			linkCtx, linkCancel := context.WithTimeout(context.Background(), 25*time.Second)
			token, linkErr := client.CreateCallLink(linkCtx, mediaType, time.Unix(startTime, 0))
			linkCancel()
			if linkErr != nil {
				if isDirectChat && forceStructured {
					return "", fmt.Errorf("falha ao criar link oficial de chamada do WhatsApp: %w", linkErr)
				}
				s.logger.Warnf("[%s] Falha ao criar link oficial de chamada (%v), mantendo join_link informado", sessionKey, linkErr)
			} else {
				finalJoinLink = callLinkPrefix + token
			}
		}
		isScheduleCall = true
	}

	s.logger.Infof(
		"[%s] Preparando envio de evento (to=%s, server=%s, force_structured=%t, call_type=%s, is_schedule_call=%t, has_join_link=%t, has_reminder=%t, reminder_offset_sec=%d)",
		sessionKey,
		number,
		jid.Server,
		forceStructured,
		normalizedCallType,
		isScheduleCall,
		finalJoinLink != "",
		hasReminder,
		reminderOffsetSec,
	)

	if isDirectChat && forceStructured {
		s.logger.Warnf("[%s] Forçando EventMessage nativo em chat direto; requer cliente WhatsApp compatível no destino", sessionKey)
	}

	eventMsg := &waE2E.EventMessage{
		Name:       proto.String(name),
		StartTime:  proto.Int64(startTime),
		IsCanceled: proto.Bool(false),
	}
	if description != "" {
		eventMsg.Description = proto.String(description)
	}
	if finalJoinLink != "" {
		eventMsg.JoinLink = proto.String(finalJoinLink)
	}
	if endTime > 0 {
		eventMsg.EndTime = proto.Int64(endTime)
	}
	if extraGuestsAllowed {
		eventMsg.ExtraGuestsAllowed = proto.Bool(true)
	}
	if isScheduleCall {
		eventMsg.IsScheduleCall = proto.Bool(true)
	}
	if hasReminder {
		eventMsg.HasReminder = proto.Bool(true)
	}
	if hasReminder && reminderOffsetSec > 0 {
		eventMsg.ReminderOffsetSec = proto.Int64(reminderOffsetSec)
	}

	msg := &waE2E.Message{
		EventMessage: eventMsg,
		MessageContextInfo: &waE2E.MessageContextInfo{
			MessageSecret: random.Bytes(32),
		},
	}

	resp, err := sendMessageWithTimeout(45*time.Second, msg)
	if err != nil {
		return "", fmt.Errorf("falha ao enviar evento: %w", err)
	}
	s.logger.Infof(
		"[%s] Evento enviado para %s (id=%s, server_ts=%s, server_id=%d, start_time=%d)",
		sessionKey,
		number,
		resp.ID,
		resp.Timestamp.Format(time.RFC3339),
		resp.ServerID,
		startTime,
	)
	messageID := resp.ID
	if messageID == "" {
		messageID = "unknown"
	}

	return messageID, nil
}

func isWhatsAppCallLink(link string) bool {
	normalized := strings.ToLower(strings.TrimSpace(link))
	return strings.HasPrefix(normalized, strings.ToLower(whatsmeow.CallLinkAudioPrefix)) ||
		strings.HasPrefix(normalized, strings.ToLower(whatsmeow.CallLinkVideoPrefix))
}

func inferCallTypeForDirectEvent(joinLink string, isScheduleCall, hasReminder bool) (string, string) {
	normalized := strings.ToLower(strings.TrimSpace(joinLink))
	if strings.HasPrefix(normalized, strings.ToLower(whatsmeow.CallLinkVideoPrefix)) {
		return "video", "join_link_video"
	}
	if strings.HasPrefix(normalized, strings.ToLower(whatsmeow.CallLinkAudioPrefix)) {
		return "voice", "join_link_voice"
	}
	if strings.TrimSpace(joinLink) != "" {
		return "video", "join_link_present_default_video"
	}
	if isScheduleCall {
		return "video", "is_schedule_call_default_video"
	}
	if hasReminder {
		return "video", "has_reminder_default_video"
	}
	return "", ""
}

func (s *MultiTenantWhatsAppService) setPendingEventFallback(messageID string, chat types.JID, text string) {
	id := strings.TrimSpace(messageID)
	if id == "" || strings.TrimSpace(text) == "" {
		return
	}

	now := time.Now()
	exp := now.Add(10 * time.Minute)

	s.pendingEventFallbackMu.Lock()
	for k, v := range s.pendingEventFallback {
		if v.expiresAt.Before(now) {
			delete(s.pendingEventFallback, k)
		}
	}
	s.pendingEventFallback[id] = pendingEventFallback{
		chat:      chat,
		text:      text,
		expiresAt: exp,
	}
	s.pendingEventFallbackMu.Unlock()
}

func (s *MultiTenantWhatsAppService) popPendingEventFallback(messageID string) (pendingEventFallback, bool) {
	id := strings.TrimSpace(messageID)
	if id == "" {
		return pendingEventFallback{}, false
	}

	s.pendingEventFallbackMu.Lock()
	defer s.pendingEventFallbackMu.Unlock()

	pending, ok := s.pendingEventFallback[id]
	if !ok {
		return pendingEventFallback{}, false
	}
	delete(s.pendingEventFallback, id)
	if pending.expiresAt.Before(time.Now()) {
		return pendingEventFallback{}, false
	}
	return pending, true
}

func extractPollCreationMessage(msg *waE2E.Message) *waE2E.PollCreationMessage {
	if msg == nil {
		return nil
	}
	if poll := msg.GetPollCreationMessage(); poll != nil {
		return poll
	}
	if poll := msg.GetPollCreationMessageV2(); poll != nil {
		return poll
	}
	if poll := msg.GetPollCreationMessageV3(); poll != nil {
		return poll
	}
	if poll := msg.GetPollCreationMessageV5(); poll != nil {
		return poll
	}
	if fp := msg.GetPollCreationMessageV4(); fp != nil && fp.GetMessage() != nil {
		return extractPollCreationMessage(fp.GetMessage())
	}
	if fp := msg.GetPollCreationMessageV6(); fp != nil && fp.GetMessage() != nil {
		return extractPollCreationMessage(fp.GetMessage())
	}
	return nil
}

func extractPollOptionNames(poll *waE2E.PollCreationMessage) []string {
	if poll == nil {
		return nil
	}
	options := make([]string, 0, len(poll.GetOptions()))
	for _, opt := range poll.GetOptions() {
		name := strings.TrimSpace(opt.GetOptionName())
		if name == "" {
			continue
		}
		options = append(options, name)
	}
	return options
}

func buildPollHashIndex(options []string) map[string]string {
	index := make(map[string]string, len(options))
	for _, option := range options {
		trimmed := strings.TrimSpace(option)
		if trimmed == "" {
			continue
		}
		hash := sha256.Sum256([]byte(trimmed))
		index[hex.EncodeToString(hash[:])] = trimmed
	}
	return index
}

func pollMetadataKey(sessionKey string, chat types.JID, messageID string) string {
	keySession := strings.TrimSpace(sessionKey)
	keyChat := strings.TrimSpace(chat.String())
	keyMessageID := strings.TrimSpace(messageID)
	if keySession == "" || keyChat == "" || keyMessageID == "" || strings.EqualFold(keyMessageID, "unknown") {
		return ""
	}
	return keySession + "|" + keyChat + "|" + keyMessageID
}

func normalizePollOptions(options []string) []string {
	trimmedOptions := make([]string, 0, len(options))
	for _, option := range options {
		trimmed := strings.TrimSpace(option)
		if trimmed == "" {
			continue
		}
		trimmedOptions = append(trimmedOptions, trimmed)
	}
	return trimmedOptions
}

func (s *MultiTenantWhatsAppService) setPollMetadata(sessionKey string, chat types.JID, messageID, question string, options []string) {
	var session *models.WhatsAppSession
	if s.repository != nil {
		if loaded, err := s.repository.GetBySessionKey(strings.TrimSpace(sessionKey)); err == nil {
			session = loaded
		}
	}
	s.setPollMetadataForSession(session, sessionKey, chat, messageID, question, options)
}

func (s *MultiTenantWhatsAppService) setPollMetadataForSession(session *models.WhatsAppSession, sessionKey string, chat types.JID, messageID, question string, options []string) {
	if strings.TrimSpace(sessionKey) == "" && session != nil {
		sessionKey = session.WhatsAppSessionKey
	}
	key := pollMetadataKey(sessionKey, chat, messageID)
	if key == "" {
		return
	}

	trimmedOptions := normalizePollOptions(options)
	trimmedQuestion := strings.TrimSpace(question)

	now := time.Now()
	exp := now.Add(7 * 24 * time.Hour)

	s.pollMetadataMu.Lock()
	for k, v := range s.pollMetadata {
		if v.expiresAt.Before(now) {
			delete(s.pollMetadata, k)
		}
	}
	s.pollMetadata[key] = pollMetadata{
		question:         trimmedQuestion,
		options:          append([]string(nil), trimmedOptions...),
		optionHashToName: buildPollHashIndex(trimmedOptions),
		expiresAt:        exp,
	}
	s.pollMetadataMu.Unlock()

	if session == nil || s.pollRepo == nil {
		return
	}
	meta := &models.PollMetadata{
		SessionID:     session.ID,
		TenantID:      strings.TrimSpace(session.TenantID),
		ChatJID:       strings.TrimSpace(chat.String()),
		PollMessageID: strings.TrimSpace(messageID),
		Question:      trimmedQuestion,
		Options:       append([]string(nil), trimmedOptions...),
	}
	if upsertErr := s.pollRepo.Upsert(meta); upsertErr != nil {
		s.logger.Warnf("[%s] Falha ao persistir metadados da enquete %s: %v", session.WhatsAppSessionKey, messageID, upsertErr)
	}
}

func (s *MultiTenantWhatsAppService) getPollMetadata(sessionKey string, chat types.JID, messageID string) (pollMetadata, bool) {
	key := pollMetadataKey(sessionKey, chat, messageID)
	if key == "" {
		return pollMetadata{}, false
	}

	s.pollMetadataMu.RLock()
	meta, ok := s.pollMetadata[key]
	s.pollMetadataMu.RUnlock()
	if !ok {
		return pollMetadata{}, false
	}
	if meta.expiresAt.Before(time.Now()) {
		s.pollMetadataMu.Lock()
		delete(s.pollMetadata, key)
		s.pollMetadataMu.Unlock()
		return pollMetadata{}, false
	}

	meta.options = append([]string(nil), meta.options...)
	meta.optionHashToName = cloneStringMap(meta.optionHashToName)
	return meta, true
}

func cloneStringMap(input map[string]string) map[string]string {
	if len(input) == 0 {
		return map[string]string{}
	}
	out := make(map[string]string, len(input))
	for k, v := range input {
		out[k] = v
	}
	return out
}

func (s *MultiTenantWhatsAppService) resolvePollVote(session *models.WhatsAppSession, chat types.JID, pollID string, selectedOptionHashes [][]byte) ([]string, []string, string) {
	selectedOptions := make([]string, 0, len(selectedOptionHashes))
	selectedHashes := make([]string, 0, len(selectedOptionHashes))

	sessionKey := ""
	if session != nil {
		sessionKey = session.WhatsAppSessionKey
	}
	meta, hasMeta := s.getPollMetadata(sessionKey, chat, pollID)
	if !hasMeta && session != nil && s.pollRepo != nil {
		persistedMeta, err := s.pollRepo.GetBySessionChatAndPoll(session.ID, chat.String(), pollID)
		if err != nil {
			s.logger.Warnf("[%s] Falha ao buscar metadados persistidos da enquete %s: %v", session.WhatsAppSessionKey, pollID, err)
		}
		if persistedMeta == nil {
			persistedMeta, err = s.pollRepo.GetBySessionAndPoll(session.ID, pollID)
			if err != nil {
				s.logger.Warnf("[%s] Falha ao buscar metadados persistidos da enquete %s por sessão: %v", session.WhatsAppSessionKey, pollID, err)
			}
		}
		if persistedMeta != nil {
			s.setPollMetadataForSession(
				session,
				session.WhatsAppSessionKey,
				chat,
				persistedMeta.PollMessageID,
				persistedMeta.Question,
				persistedMeta.Options,
			)
			meta, hasMeta = s.getPollMetadata(session.WhatsAppSessionKey, chat, pollID)
		}
	}

	pollName := ""
	if hasMeta {
		pollName = meta.question
	}

	for _, optionHash := range selectedOptionHashes {
		if len(optionHash) == 0 {
			continue
		}
		hashHex := hex.EncodeToString(optionHash)
		selectedHashes = append(selectedHashes, hashHex)
		if hasMeta {
			if optionName, ok := meta.optionHashToName[hashHex]; ok {
				selectedOptions = append(selectedOptions, optionName)
			}
		}
	}

	return selectedOptions, selectedHashes, pollName
}

func buildEventFallbackText(name, description, joinLink string, startTime, endTime int64) string {
	lines := []string{
		fmt.Sprintf("Evento: %s", strings.TrimSpace(name)),
		fmt.Sprintf("Inicio: %s", time.Unix(startTime, 0).In(time.Local).Format("02/01/2006 15:04")),
	}
	if endTime > 0 {
		lines = append(lines, fmt.Sprintf("Fim: %s", time.Unix(endTime, 0).In(time.Local).Format("02/01/2006 15:04")))
	}
	if strings.TrimSpace(description) != "" {
		lines = append(lines, fmt.Sprintf("Descricao: %s", strings.TrimSpace(description)))
	}
	if strings.TrimSpace(joinLink) != "" {
		lines = append(lines, fmt.Sprintf("Link: %s", strings.TrimSpace(joinLink)))
	}
	return strings.Join(lines, "\n")
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

	if strings.Contains(n, "@") {
		jid, err := types.ParseJID(n)
		if err != nil {
			return types.JID{}, fmt.Errorf("jid inválido: %w", err)
		}
		return jid, nil
	}

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
		limit = 50 << 20 // 50 MB
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

func normalizeExpectedContentType(detected, provided, expectedPrefix string) string {
	candidate := strings.TrimSpace(detected)
	if candidate == "" || strings.EqualFold(candidate, "application/octet-stream") {
		candidate = strings.TrimSpace(provided)
	}
	if candidate == "" {
		return ""
	}
	if strings.HasPrefix(strings.ToLower(candidate), strings.ToLower(expectedPrefix)) {
		return candidate
	}
	if strings.HasPrefix(strings.ToLower(provided), strings.ToLower(expectedPrefix)) {
		return strings.TrimSpace(provided)
	}
	return ""
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

func mediaTypeToMessageType(mediaType whatsmeow.MediaType) string {
	switch mediaType {
	case whatsmeow.MediaImage:
		return "image"
	case whatsmeow.MediaVideo:
		return "video"
	case whatsmeow.MediaAudio:
		return "audio"
	default:
		return "document"
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
