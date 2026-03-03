package services

import (
	"boot-whatsapp-golang/internal/models"
	"boot-whatsapp-golang/internal/repository"
	"boot-whatsapp-golang/pkg/logger"
	"boot-whatsapp-golang/pkg/media"
	"context"
	"encoding/base64"
	"fmt"
	"io/fs"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

type MessageService struct {
	repository       *repository.MessageRepository
	mediaStoragePath string
	log              *logger.Logger
	retentionDays    int
	cleanupInterval  time.Duration
	cleanupStateMu   sync.Mutex
	cleanupRunning   bool
}

func NewMessageService(repo *repository.MessageRepository, log *logger.Logger) *MessageService {
	mediaStoragePath := strings.TrimSpace(os.Getenv("MEDIA_STORAGE_PATH"))
	if mediaStoragePath == "" {
		mediaStoragePath = filepath.Join("storage", "media")
	}
	if absPath, err := filepath.Abs(mediaStoragePath); err == nil {
		mediaStoragePath = absPath
	}

	if err := os.MkdirAll(mediaStoragePath, 0o755); err != nil {
		fmt.Printf("[WARN] Falha ao criar pasta de mídia %s: %v\n", mediaStoragePath, err)
	}

	retentionDays := getEnvInt("MEDIA_RETENTION_DAYS", 7)
	cleanupInterval := getEnvDuration("MEDIA_CLEANUP_INTERVAL", 24*time.Hour)

	return &MessageService{
		repository:       repo,
		mediaStoragePath: mediaStoragePath,
		log:              log,
		retentionDays:    retentionDays,
		cleanupInterval:  cleanupInterval,
	}
}

func (s *MessageService) SaveOutboundMessage(
	sessionID uuid.UUID,
	tenantID string,
	messageID string,
	recipient string,
	content string,
	messageType string,
) (*models.Message, error) {
	msg := &models.Message{
		ID:                uuid.New(),
		SessionID:         sessionID,
		TenantID:          tenantID,
		WhatsAppMessageID: &messageID,
		Direction:         "outbound",
		Recipient:         &recipient,
		MessageType:       messageType,
		Content:           &content,
		Status:            "sent",
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
	}

	err := s.repository.SaveMessage(msg)
	if err != nil {
		return nil, err
	}

	return msg, nil
}

func (s *MessageService) SaveInboundMessage(
	sessionID uuid.UUID,
	tenantID string,
	messageID string,
	sender string,
	content string,
	messageType string,
) (*models.Message, error) {
	msg := &models.Message{
		ID:                uuid.New(),
		SessionID:         sessionID,
		TenantID:          tenantID,
		WhatsAppMessageID: &messageID,
		Direction:         "inbound",
		Sender:            &sender,
		MessageType:       messageType,
		Content:           &content,
		Status:            "received",
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
	}

	err := s.repository.SaveMessage(msg)
	if err != nil {
		return nil, err
	}

	return msg, nil
}

func (s *MessageService) SaveMediaMessage(
	sessionID uuid.UUID,
	tenantID string,
	messageID string,
	direction string,
	contact string,
	messageType string,
	mediaURL string,
	mediaBytes []byte,
	mimeType string,
	caption string,
) (*models.Message, error) {
	status := "sent"
	if direction == "inbound" {
		status = "received"
	}

	msg := &models.Message{
		ID:                uuid.New(),
		SessionID:         sessionID,
		TenantID:          tenantID,
		WhatsAppMessageID: &messageID,
		Direction:         direction,
		MessageType:       messageType,
		MediaURL:          &mediaURL,
		Status:            status,
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
	}

	storedMediaURL := strings.TrimSpace(mediaURL)
	if len(mediaBytes) > 0 {
		if localMediaURL, err := s.storeMediaLocally(sessionID, messageID, messageType, mimeType, mediaBytes); err != nil {
			fmt.Printf("[WARN] Falha ao salvar mídia localmente (%s): %v\n", messageID, err)
		} else {
			storedMediaURL = localMediaURL
		}
	}

	if storedMediaURL != "" {
		msg.MediaURL = &storedMediaURL
	}

	if mimeType != "" {
		msg.MimeType = &mimeType
	}

	if caption != "" {
		msg.Content = &caption
	}

	if direction == "outbound" {
		msg.Recipient = &contact
	} else {
		msg.Sender = &contact
	}

	err := s.repository.SaveMessage(msg)
	if err != nil {
		return nil, err
	}

	return msg, nil
}

func (s *MessageService) UpdateMessageStatus(messageID string, status string) error {
	return s.repository.UpdateMessageStatus(messageID, status)
}

func (s *MessageService) GetConversation(sessionID uuid.UUID, contactNumber string, limit int, offset int, includeMedia bool) ([]*models.Message, int, error) {
	messages, err := s.repository.GetConversation(sessionID, contactNumber, limit, offset)
	if err != nil {
		return nil, 0, err
	}

	for _, msg := range messages {
		if includeMedia && media.IsMediaMessage(msg.MessageType) {
			if msg.MediaBase64Stored != nil && len(*msg.MediaBase64Stored) > 0 {
				fmt.Printf("[DEBUG] Usando mídia armazenada no banco para %s (size=%d bytes)\n", msg.ID, len(*msg.MediaBase64Stored))

				mimeType := "image/jpeg"
				if msg.MimeType != nil && *msg.MimeType != "" {
					mimeType = *msg.MimeType
				}

				encoded := base64.StdEncoding.EncodeToString(*msg.MediaBase64Stored)
				msg.MediaBase64 = new(fmt.Sprintf("data:%s;base64,%s", mimeType, encoded))
			} else if msg.MediaURL != nil && *msg.MediaURL != "" {
				fmt.Printf("[DEBUG] Tentando fazer download on-demand de %s\n", msg.ID)

				mediaData, err := s.DownloadMedia(*msg.MediaURL)
				if err != nil {
					fmt.Printf("[ERROR] Erro ao carregar mídia base64 para %s: %v\n", msg.ID, err)
					continue
				}

				fmt.Printf("[DEBUG] Mídia baixada com sucesso - size=%d bytes, mimeType=%s\n", len(mediaData.Base64), mediaData.MimeType)

				mimeType := mediaData.MimeType

				if mimeType == "" || mimeType == "application/octet-stream" {
					fmt.Printf("[WARN] MIME type inválido, usando fallback para %s\n", msg.MessageType)
					switch msg.MessageType {
					case "image":
						mimeType = "image/jpeg"
					case "video":
						mimeType = "video/mp4"
					case "document":
						mimeType = "application/pdf"
					case "audio":
						mimeType = "audio/mpeg"
					default:
						mimeType = "application/octet-stream"
					}
				}

				dataURI := fmt.Sprintf("data:%s;base64,%s", mimeType, mediaData.Base64)
				msg.MediaBase64 = &dataURI
				fmt.Printf("[DEBUG] Data URI criado - tamanho total: %d bytes\n", len(dataURI))
			}
		}
	}

	count, err := s.repository.GetConversationCount(sessionID, contactNumber)
	if err != nil {
		return nil, 0, err
	}

	return messages, count, nil
}

func (s *MessageService) GetContacts(sessionID uuid.UUID) ([]string, error) {
	return s.repository.GetContactNumbers(sessionID)
}

func (s *MessageService) GetMessageStats(sessionID uuid.UUID) (map[string]interface{}, error) {
	inboundMsgs, err := s.repository.GetMessagesByStatus(sessionID, "received")
	if err != nil {
		return nil, fmt.Errorf("erro ao contar mensagens recebidas: %w", err)
	}

	outboundMsgs, err := s.repository.GetMessagesByStatus(sessionID, "sent")
	if err != nil {
		return nil, fmt.Errorf("erro ao contar mensagens enviadas: %w", err)
	}

	total, err := s.repository.GetConversationCount(sessionID, "")
	if err != nil {
		return nil, fmt.Errorf("erro ao contar total de mensagens: %w", err)
	}

	contacts, err := s.repository.GetContactNumbers(sessionID)
	if err != nil {
		return nil, fmt.Errorf("erro ao listar contatos: %w", err)
	}

	stats := map[string]interface{}{
		"total_messages": total,
		"inbound":        len(inboundMsgs),
		"outbound":       len(outboundMsgs),
		"contact_count":  len(contacts),
	}

	return stats, nil
}

func (s *MessageService) GetMessageByID(messageID uuid.UUID) (*models.Message, error) {
	message, err := s.repository.GetMessageByID(messageID)
	if err != nil {
		return nil, err
	}
	return message, nil
}

func (s *MessageService) DownloadMedia(mediaURL string) (*media.DataDrive, error) {
	if isLocalMediaURL(mediaURL) {
		return s.loadLocalMedia(mediaURL)
	}
	return media.URLToMediaData(mediaURL)
}

func (s *MessageService) StartMediaCleanupJob(ctx context.Context) {
	if s.retentionDays <= 0 {
		s.logInfo("Limpeza de mídia desabilitada: MEDIA_RETENTION_DAYS <= 0")
		return
	}
	if s.cleanupInterval <= 0 {
		s.logInfo("Limpeza de mídia desabilitada: MEDIA_CLEANUP_INTERVAL <= 0")
		return
	}

	s.logInfo(
		"Job de limpeza de mídia iniciado (path=%s, retenção=%d dias, intervalo=%s)",
		s.mediaStoragePath,
		s.retentionDays,
		s.cleanupInterval,
	)

	go s.runMediaCleanupOnce()

	go func() {
		ticker := time.NewTicker(s.cleanupInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				s.logInfo("Job de limpeza de mídia encerrado")
				return
			case <-ticker.C:
				s.runMediaCleanupOnce()
			}
		}
	}()
}

func isLocalMediaURL(mediaURL string) bool {
	return strings.HasPrefix(strings.TrimSpace(mediaURL), "local://")
}

func (s *MessageService) runMediaCleanupOnce() {
	s.cleanupStateMu.Lock()
	if s.cleanupRunning {
		s.cleanupStateMu.Unlock()
		s.logInfo("Job de limpeza já em execução; pulando ciclo")
		return
	}
	s.cleanupRunning = true
	s.cleanupStateMu.Unlock()

	defer func() {
		s.cleanupStateMu.Lock()
		s.cleanupRunning = false
		s.cleanupStateMu.Unlock()
	}()

	if isDangerousCleanupRoot(s.mediaStoragePath) {
		s.logWarn("Path de limpeza inválido e perigoso: %s", s.mediaStoragePath)
		return
	}

	root := filepath.Clean(s.mediaStoragePath)
	cutoff := time.Now().AddDate(0, 0, -s.retentionDays)
	start := time.Now()
	deletedFiles := 0
	var deletedBytes int64
	dirs := make([]string, 0, 128)

	walkErr := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			s.logWarn("Erro ao acessar %s: %v", path, err)
			return nil
		}

		if d.IsDir() {
			dirs = append(dirs, path)
			return nil
		}

		info, infoErr := d.Info()
		if infoErr != nil {
			s.logWarn("Erro ao obter info de %s: %v", path, infoErr)
			return nil
		}

		if !info.Mode().IsRegular() {
			return nil
		}
		if info.ModTime().After(cutoff) {
			return nil
		}

		size := info.Size()
		if removeErr := os.Remove(path); removeErr != nil {
			s.logWarn("Falha ao remover arquivo antigo %s: %v", path, removeErr)
			return nil
		}
		deletedFiles++
		deletedBytes += size
		return nil
	})
	if walkErr != nil {
		s.logWarn("Erro durante varredura de limpeza de mídia: %v", walkErr)
	}

	sort.Slice(dirs, func(i, j int) bool {
		return len(dirs[i]) > len(dirs[j])
	})
	for _, dir := range dirs {
		if dir == root {
			continue
		}
		entries, err := os.ReadDir(dir)
		if err == nil && len(entries) == 0 {
			_ = os.Remove(dir)
		}
	}

	s.logInfo(
		"Limpeza de mídia concluída em %v (removidos=%d arquivos, bytes=%d, cutoff=%s)",
		time.Since(start),
		deletedFiles,
		deletedBytes,
		cutoff.Format(time.RFC3339),
	)
}

func isDangerousCleanupRoot(path string) bool {
	clean := filepath.Clean(strings.TrimSpace(path))
	if clean == "" || clean == "." {
		return true
	}
	if clean == string(os.PathSeparator) {
		return true
	}

	volume := filepath.VolumeName(clean)
	if volume != "" {
		root := volume + string(os.PathSeparator)
		if strings.EqualFold(clean, root) {
			return true
		}
	}

	return false
}

func getEnvInt(key string, defaultValue int) int {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return defaultValue
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return defaultValue
	}
	return v
}

func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return defaultValue
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		return defaultValue
	}
	return d
}

func (s *MessageService) logInfo(format string, args ...interface{}) {
	if s.log != nil {
		s.log.Infof(format, args...)
		return
	}
	fmt.Printf("[INFO] "+format+"\n", args...)
}

func (s *MessageService) logWarn(format string, args ...interface{}) {
	if s.log != nil {
		s.log.Warnf(format, args...)
		return
	}
	fmt.Printf("[WARN] "+format+"\n", args...)
}

func (s *MessageService) storeMediaLocally(sessionID uuid.UUID, messageID, messageType, mimeType string, mediaBytes []byte) (string, error) {
	if len(mediaBytes) == 0 {
		return "", fmt.Errorf("arquivo de mídia vazio")
	}
	if strings.TrimSpace(s.mediaStoragePath) == "" {
		return "", fmt.Errorf("MEDIA_STORAGE_PATH não configurado")
	}

	targetDir := filepath.Join(
		s.mediaStoragePath,
		sessionID.String(),
		time.Now().Format("2006-01-02"),
	)
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return "", fmt.Errorf("falha ao criar diretório: %w", err)
	}

	ext := detectMediaExtension(mimeType, messageType, mediaBytes)
	filename := fmt.Sprintf("%d_%s_%s%s",
		time.Now().UnixMilli(),
		sanitizeFilePart(strings.TrimSpace(messageType)),
		sanitizeFilePart(strings.TrimSpace(messageID)),
		ext,
	)

	fullPath := filepath.Join(targetDir, filename)
	if err := os.WriteFile(fullPath, mediaBytes, 0o644); err != nil {
		return "", fmt.Errorf("falha ao escrever arquivo: %w", err)
	}

	relativePath, err := filepath.Rel(s.mediaStoragePath, fullPath)
	if err != nil {
		return "", fmt.Errorf("falha ao calcular caminho relativo: %w", err)
	}

	return "local://" + filepath.ToSlash(relativePath), nil
}

func (s *MessageService) loadLocalMedia(mediaURL string) (*media.DataDrive, error) {
	if strings.TrimSpace(s.mediaStoragePath) == "" {
		return nil, fmt.Errorf("MEDIA_STORAGE_PATH não configurado")
	}

	relative := strings.TrimSpace(strings.TrimPrefix(mediaURL, "local://"))
	relative = filepath.Clean(filepath.FromSlash(relative))
	if relative == "." || strings.HasPrefix(relative, "..") || filepath.IsAbs(relative) {
		return nil, fmt.Errorf("referência local inválida")
	}

	fullPath := filepath.Join(s.mediaStoragePath, relative)
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, fmt.Errorf("falha ao ler arquivo local: %w", err)
	}

	mimeType := strings.TrimSpace(mime.TypeByExtension(filepath.Ext(fullPath)))
	if mimeType == "" || mimeType == "application/octet-stream" {
		mimeType = strings.TrimSpace(http.DetectContentType(data))
	}
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}

	return &media.DataDrive{
		Base64:   base64.StdEncoding.EncodeToString(data),
		MimeType: mimeType,
	}, nil
}

func sanitizeFilePart(value string) string {
	if value == "" {
		return "media"
	}

	var b strings.Builder
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-' || r == '_':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}

	s := strings.Trim(b.String(), "_")
	if s == "" {
		return "media"
	}
	return s
}

func detectMediaExtension(mimeType, messageType string, mediaBytes []byte) string {
	normalized := strings.TrimSpace(strings.ToLower(mimeType))
	if idx := strings.Index(normalized, ";"); idx >= 0 {
		normalized = strings.TrimSpace(normalized[:idx])
	}

	if normalized == "" || normalized == "application/octet-stream" {
		normalized = strings.TrimSpace(strings.ToLower(http.DetectContentType(mediaBytes)))
		if idx := strings.Index(normalized, ";"); idx >= 0 {
			normalized = strings.TrimSpace(normalized[:idx])
		}
	}

	if exts, err := mime.ExtensionsByType(normalized); err == nil && len(exts) > 0 {
		return exts[0]
	}

	switch strings.ToLower(strings.TrimSpace(messageType)) {
	case "image":
		return ".jpg"
	case "video":
		return ".mp4"
	case "audio":
		return ".mp3"
	case "document":
		return ".bin"
	default:
		return ".bin"
	}
}
