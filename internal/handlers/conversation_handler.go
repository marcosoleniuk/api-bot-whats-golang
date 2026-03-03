package handlers

import (
	"boot-whatsapp-golang/internal/middleware"
	"boot-whatsapp-golang/internal/models"
	"boot-whatsapp-golang/internal/repository"
	"boot-whatsapp-golang/internal/services"
	"boot-whatsapp-golang/pkg/logger"
	"boot-whatsapp-golang/pkg/media"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

type ConversationHandler struct {
	messageService    *services.MessageService
	sessionRepository *repository.SessionRepository
	log               *logger.Logger
}

func NewConversationHandler(
	messageService *services.MessageService,
	sessionRepository *repository.SessionRepository,
	log *logger.Logger,
) *ConversationHandler {
	return &ConversationHandler{
		messageService:    messageService,
		sessionRepository: sessionRepository,
		log:               log,
	}
}

type Pagination struct {
	Total  int `json:"total"`
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}

type ConversationResponse struct {
	Messages   []map[string]interface{} `json:"messages"`
	Pagination Pagination               `json:"pagination"`
}


func (h *ConversationHandler) writeJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(payload); err != nil {
		h.log.Error("Erro ao codificar JSON: %v", err)
	}
}

func (h *ConversationHandler) writeError(w http.ResponseWriter, status int, msg string) {
	http.Error(w, msg, status)
}

func (h *ConversationHandler) resolveSessionID(r *http.Request, requireAuth bool) (uuid.UUID, error) {
	if requireAuth {
		sessionKey := middleware.GetSessionKey(r)
		if sessionKey == "" {
			return uuid.Nil, fmt.Errorf("unauthorized")
		}

		session, err := h.sessionRepository.GetBySessionKey(sessionKey)
		if err != nil || session == nil {
			return uuid.Nil, fmt.Errorf("session not found")
		}
		return session.ID, nil
	}

	sessionIDStr := mux.Vars(r)["sessionID"]
	return uuid.Parse(sessionIDStr)
}

func parsePaginationParams(r *http.Request) (limit, offset int, includeMedia bool) {
	limit = 50
	offset = 0
	includeMedia = false

	if l, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && l > 0 {
		limit = l
	}

	if o, err := strconv.Atoi(r.URL.Query().Get("offset")); err == nil && o >= 0 {
		offset = o
	}

	includeMedia = r.URL.Query().Get("include_media") == "true"

	return
}

func (h *ConversationHandler) handleConversation(w http.ResponseWriter, r *http.Request, requireAuth bool) {
	sessionID, err := h.resolveSessionID(r, requireAuth)
	if err != nil {
		h.writeError(w, http.StatusUnauthorized, err.Error())
		return
	}

	contact := r.URL.Query().Get("contact")
	limit, offset, includeMedia := parsePaginationParams(r)

	messages, total, err := h.messageService.GetConversation(sessionID, contact, limit, offset, includeMedia)
	if err != nil {
		h.log.Error("Erro ao recuperar conversa: %v", err)
		h.writeError(w, http.StatusInternalServerError, "Erro ao recuperar conversa")
		return
	}

	response := ConversationResponse{
		Messages: addMediaDownloadURLs(messages),
		Pagination: Pagination{
			Total:  total,
			Limit:  limit,
			Offset: offset,
		},
	}

	h.writeJSON(w, http.StatusOK, response)
}

func (h *ConversationHandler) GetConversation(w http.ResponseWriter, r *http.Request) {
	h.handleConversation(w, r, false)
}

func (h *ConversationHandler) GetConversationWithAuth(w http.ResponseWriter, r *http.Request) {
	h.handleConversation(w, r, true)
}

func (h *ConversationHandler) handleContacts(w http.ResponseWriter, r *http.Request, requireAuth bool) {
	sessionID, err := h.resolveSessionID(r, requireAuth)
	if err != nil {
		h.writeError(w, http.StatusUnauthorized, err.Error())
		return
	}

	contacts, err := h.messageService.GetContacts(sessionID)
	if err != nil {
		h.log.Error("Erro ao recuperar contatos: %v", err)
		h.writeError(w, http.StatusInternalServerError, "Erro ao recuperar contatos")
		return
	}

	if contacts == nil {
		contacts = []string{}
	}

	h.writeJSON(w, http.StatusOK, models.NewSuccessResponse("Contatos recuperados", map[string]interface{}{
		"contacts": contacts,
		"count":    len(contacts),
	}))
}

func (h *ConversationHandler) GetContacts(w http.ResponseWriter, r *http.Request) {
	h.handleContacts(w, r, false)
}

func (h *ConversationHandler) GetContactsWithAuth(w http.ResponseWriter, r *http.Request) {
	h.handleContacts(w, r, true)
}

func (h *ConversationHandler) handleStats(w http.ResponseWriter, r *http.Request, requireAuth bool) {
	sessionID, err := h.resolveSessionID(r, requireAuth)
	if err != nil {
		h.writeError(w, http.StatusUnauthorized, err.Error())
		return
	}

	stats, err := h.messageService.GetMessageStats(sessionID)
	if err != nil {
		h.log.Error("Erro ao recuperar estatísticas: %v", err)
		h.writeError(w, http.StatusInternalServerError, "Erro ao recuperar estatísticas")
		return
	}

	h.writeJSON(w, http.StatusOK, models.NewSuccessResponse("Estatísticas de mensagens", stats))
}

func (h *ConversationHandler) GetMessageStats(w http.ResponseWriter, r *http.Request) {
	h.handleStats(w, r, false)
}

func (h *ConversationHandler) GetStatsWithAuth(w http.ResponseWriter, r *http.Request) {
	h.handleStats(w, r, true)
}

func (h *ConversationHandler) GetMessageMedia(w http.ResponseWriter, r *http.Request) {
	messageIDStr := mux.Vars(r)["messageID"]
	messageID, err := uuid.Parse(messageIDStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "Invalid message ID")
		return
	}

	message, err := h.messageService.GetMessageByID(messageID)
	if err != nil || message == nil {
		h.writeError(w, http.StatusNotFound, "Message not found")
		return
	}

	if message.MediaBase64Stored != nil && len(*message.MediaBase64Stored) > 0 {
		stored := *message.MediaBase64Stored
		h.streamMedia(w, stored, resolveMediaMime(stored, message.MessageType, stringValue(message.MimeType)))
		return
	}

	if message.MediaURL != nil && *message.MediaURL != "" {
		mediaData, err := h.messageService.DownloadMedia(*message.MediaURL)
		if err != nil {
			h.writeError(w, http.StatusInternalServerError, "Failed to download media")
			return
		}

		decoded, err := base64.StdEncoding.DecodeString(mediaData.Base64)
		if err != nil {
			h.writeError(w, http.StatusInternalServerError, "Failed to decode media")
			return
		}

		h.streamMedia(
			w,
			decoded,
			resolveMediaMime(decoded, message.MessageType, stringValue(message.MimeType), mediaData.MimeType),
		)
		return
	}

	h.writeError(w, http.StatusNotFound, "No media found")
}

func (h *ConversationHandler) streamMedia(w http.ResponseWriter, data []byte, mime string) {
	w.Header().Set("Content-Type", mime)
	w.Header().Set("Content-Disposition",
		fmt.Sprintf("inline; filename=media.%s", getExtensionFromMimeType(mime)),
	)
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	_, _ = w.Write(data)
}

func resolveMediaMime(data []byte, messageType string, candidates ...string) string {
	for _, candidate := range candidates {
		if normalized := normalizeMimeType(candidate); normalized != "" && normalized != "application/octet-stream" {
			return normalized
		}
	}

	if detected := normalizeMimeType(http.DetectContentType(data)); detected != "" && detected != "application/octet-stream" {
		return detected
	}

	switch strings.ToLower(strings.TrimSpace(messageType)) {
	case "image":
		return "image/jpeg"
	case "video":
		return "video/mp4"
	case "document":
		return "application/pdf"
	case "audio":
		return "audio/mpeg"
	default:
		return "application/octet-stream"
	}
}

func normalizeMimeType(mimeType string) string {
	mimeType = strings.ToLower(strings.TrimSpace(mimeType))
	if idx := strings.Index(mimeType, ";"); idx != -1 {
		mimeType = strings.TrimSpace(mimeType[:idx])
	}

	switch mimeType {
	case "image/jpg":
		return "image/jpeg"
	default:
		return mimeType
	}
}

func stringValue(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}


func addMediaDownloadURLs(messages []*models.Message) []map[string]interface{} {
	result := make([]map[string]interface{}, 0, len(messages))

	for _, msg := range messages {
		msgMap := messageToMap(msg)

		if media.IsMediaMessage(msg.MessageType) && msg.ID != uuid.Nil {
			mimeType := "application/octet-stream"
			if msg.MimeType != nil && *msg.MimeType != "" {
				mimeType = *msg.MimeType
			}
			ext := getExtensionFromMimeType(mimeType)

			msgMap["media_download_url"] =
				fmt.Sprintf("/api/v1/messages/%s/media/%s.%s", msg.ID.String(), msg.MessageType, ext)
		}

		result = append(result, msgMap)
	}

	return result
}

func messageToMap(msg *models.Message) map[string]interface{} {
	return map[string]interface{}{
		"id":                  msg.ID,
		"session_id":          msg.SessionID,
		"tenant_id":           msg.TenantID,
		"whatsapp_message_id": msg.WhatsAppMessageID,
		"direction":           msg.Direction,
		"sender":              msg.Sender,
		"recipient":           msg.Recipient,
		"message_type":        msg.MessageType,
		"content":             msg.Content,
		"media_url":           msg.MediaURL,
		"mime_type":           msg.MimeType,
		"media_base64":        msg.MediaBase64,
		"status":              msg.Status,
		"created_at":          msg.CreatedAt,
		"updated_at":          msg.UpdatedAt,
	}
}

func getExtensionFromMimeType(mimeType string) string {
	switch normalizeMimeType(mimeType) {
	case "image/jpeg":
		return "jpg"
	case "image/png":
		return "png"
	case "image/gif":
		return "gif"
	case "image/webp":
		return "webp"
	case "image/heic":
		return "heic"
	case "image/heif":
		return "heif"
	case "video/mp4":
		return "mp4"
	case "audio/mpeg":
		return "mp3"
	case "audio/ogg":
		return "ogg"
	case "audio/opus":
		return "opus"
	case "application/pdf":
		return "pdf"
	default:
		return "bin"
	}
}