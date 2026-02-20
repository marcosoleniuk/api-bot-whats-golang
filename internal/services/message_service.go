package services

import (
	"boot-whatsapp-golang/internal/models"
	"boot-whatsapp-golang/internal/repository"
	"boot-whatsapp-golang/pkg/media"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type MessageService struct {
	repository *repository.MessageRepository
}

func NewMessageService(repo *repository.MessageRepository) *MessageService {
	return &MessageService{repository: repo}
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

	if len(mediaBytes) > 0 {
		msg.MediaBase64Stored = &mediaBytes
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

				mediaData, err := media.URLToMediaData(*msg.MediaURL)
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
	return media.URLToMediaData(mediaURL)
}
