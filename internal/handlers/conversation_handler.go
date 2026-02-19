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

//
// ==========================
// 🔹 UTILIDADES REUTILIZÁVEIS
// ==========================
//

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

//
// ==========================
// 🔹 CONVERSA
// ==========================
//

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

//
// ==========================
// 🔹 CONTATOS
// ==========================
//

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

//
// ==========================
// 🔹 ESTATÍSTICAS
// ==========================
//

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

//
// ==========================
// 🔹 MÍDIA
// ==========================
//

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

	// Se mídia já armazenada em base64
	if message.MediaBase64Stored != nil && len(*message.MediaBase64Stored) > 0 {
		stored := *message.MediaBase64Stored
		h.streamMedia(w, stored, resolveMediaMime(stored, message.MessageType, stringValue(message.MimeType)))
		return
	}

	// Download externo
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

//
// ==========================
// 🔹 AUXILIARES
// ==========================
//

func addMediaDownloadURLs(messages []*models.Message) []map[string]interface{} {
	result := make([]map[string]interface{}, 0, len(messages))

	for _, msg := range messages {
		msgMap := messageToMap(msg)

		if media.IsMediaMessage(msg.MessageType) && msg.ID != uuid.Nil {
			// Determinar extensão baseada no MIME type
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

//package handlers
//
//import (
//	"boot-whatsapp-golang/internal/middleware"
//	"boot-whatsapp-golang/internal/models"
//	"boot-whatsapp-golang/internal/repository"
//	"boot-whatsapp-golang/internal/services"
//	"boot-whatsapp-golang/pkg/logger"
//	"boot-whatsapp-golang/pkg/media"
//	"encoding/base64"
//	"encoding/json"
//	"fmt"
//	"net/http"
//	"strconv"
//
//	"github.com/google/uuid"
//	"github.com/gorilla/mux"
//)
//
//type ConversationHandler struct {
//	messageService   *services.MessageService
//	sessionRepository *repository.SessionRepository
//	log            *logger.Logger
//}
//
//func NewConversationHandler(messageService *services.MessageService, sessionRepository *repository.SessionRepository, log *logger.Logger) *ConversationHandler {
//	return &ConversationHandler{
//		messageService:   messageService,
//		sessionRepository: sessionRepository,
//		log:            log,
//	}
//}
//
//// GetConversation recupera histórico de conversas
//// GET /api/v1/conversations/{sessionID}?contact={number}&limit=50&offset=0&include_media=false
//func (h *ConversationHandler) GetConversation(w http.ResponseWriter, r *http.Request) {
//	sessionIDStr := mux.Vars(r)["sessionID"]
//	sessionID, err := uuid.Parse(sessionIDStr)
//	if err != nil {
//		http.Error(w, "Invalid session ID", http.StatusBadRequest)
//		return
//	}
//
//	// Extrair parâmetros query
//	contactNumber := r.URL.Query().Get("contact")
//	limitStr := r.URL.Query().Get("limit")
//	offsetStr := r.URL.Query().Get("offset")
//	includeMediaStr := r.URL.Query().Get("include_media")
//
//	limit := 50
//	offset := 0
//	includeMedia := false
//
//	if limitStr != "" {
//		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
//			limit = l
//		}
//	}
//
//	if offsetStr != "" {
//		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
//			offset = o
//		}
//	}
//
//	if includeMediaStr == "true" {
//		includeMedia = true
//	}
//
//	messages, total, err := h.messageService.GetConversation(sessionID, contactNumber, limit, offset, includeMedia)
//	if err != nil {
//		h.log.Error("Erro ao recuperar conversa: %v", err)
//		http.Error(w, "Erro ao recuperar conversa", http.StatusInternalServerError)
//		return
//	}
//
//	// Adicionar URLs de download para mensagens com mídia
//	messagesWithURLs := addMediaDownloadURLs(messages)
//
//	response := map[string]interface{}{
//		"messages": messagesWithURLs,
//		"pagination": map[string]interface{}{
//			"total":  total,
//			"limit":  limit,
//			"offset": offset,
//		},
//	}
//
//	w.Header().Set("Content-Type", "application/json")
//	json.NewEncoder(w).Encode(response)
//}
//
//// addMediaDownloadURLs adiciona URLs de download para mensagens com mídia
//func addMediaDownloadURLs(messages []*models.Message) []map[string]interface{} {
//	result := make([]map[string]interface{}, len(messages))
//	for i, msg := range messages {
//		// Converter message para map
//		msgMap := messageToMap(msg)
//		// Adicionar URL de download se houver mídia
//		if media.IsMediaMessage(msg.MessageType) && msg.ID != uuid.Nil {
//			// Determinar extensão baseada no MIME type
//			mimeType := "application/octet-stream"
//			if msg.MimeType != nil && *msg.MimeType != "" {
//				mimeType = *msg.MimeType
//			}
//			ext := getExtensionFromMimeType(mimeType)
//
//			msgMap["media_download_url"] = fmt.Sprintf("/api/v1/messages/%s/media/%s.%s", msg.ID.String(), msg.MessageType, ext)
//		}
//		result[i] = msgMap
//	}
//	return result
//}
//
//// messageToMap converte um Message struct para um map com os campos
//func messageToMap(msg *models.Message) map[string]interface{} {
//	return map[string]interface{}{
//		"id":                    msg.ID,
//		"session_id":            msg.SessionID,
//		"tenant_id":             msg.TenantID,
//		"whatsapp_message_id":   msg.WhatsAppMessageID,
//		"direction":             msg.Direction,
//		"sender":                msg.Sender,
//		"recipient":             msg.Recipient,
//		"message_type":          msg.MessageType,
//		"content":               msg.Content,
//		"media_url":             msg.MediaURL,
//		"mime_type":             msg.MimeType,
//		"media_base64":          msg.MediaBase64,
//		"status":                msg.Status,
//		"created_at":            msg.CreatedAt,
//		"updated_at":            msg.UpdatedAt,
//	}
//}
//
//// GetContacts retorna lista de contatos
//// GET /api/v1/conversations/{sessionID}/contacts
//func (h *ConversationHandler) GetContacts(w http.ResponseWriter, r *http.Request) {
//	sessionIDStr := mux.Vars(r)["sessionID"]
//	sessionID, err := uuid.Parse(sessionIDStr)
//	if err != nil {
//		http.Error(w, "Invalid session ID", http.StatusBadRequest)
//		return
//	}
//
//	contacts, err := h.messageService.GetContacts(sessionID)
//	if err != nil {
//		h.log.Error("Erro ao recuperar contatos: %v", err)
//		http.Error(w, "Erro ao recuperar contatos", http.StatusInternalServerError)
//		return
//	}
//
//	if contacts == nil {
//		contacts = []string{}
//	}
//
//	response := models.NewSuccessResponse("Contatos recuperados", map[string]interface{}{
//		"contacts": contacts,
//		"count":    len(contacts),
//	})
//
//	w.Header().Set("Content-Type", "application/json")
//	json.NewEncoder(w).Encode(response)
//}
//
//// GetMessageStats retorna estatísticas de mensagens
//// GET /api/v1/conversations/{sessionID}/stats
//func (h *ConversationHandler) GetMessageStats(w http.ResponseWriter, r *http.Request) {
//	sessionIDStr := mux.Vars(r)["sessionID"]
//	sessionID, err := uuid.Parse(sessionIDStr)
//	if err != nil {
//		http.Error(w, "Invalid session ID", http.StatusBadRequest)
//		return
//	}
//
//	stats, err := h.messageService.GetMessageStats(sessionID)
//	if err != nil {
//		h.log.Error("Erro ao recuperar estatísticas: %v", err)
//		http.Error(w, "Erro ao recuperar estatísticas", http.StatusInternalServerError)
//		return
//	}
//
//	response := models.NewSuccessResponse("Estatísticas de mensagens", stats)
//
//	w.Header().Set("Content-Type", "application/json")
//	json.NewEncoder(w).Encode(response)
//}
//
//// GetConversationWithAuth recupera conversa com autenticação
//// GET /api/v1/messages/history?contact={number}&limit=50&offset=0&include_media=false
//func (h *ConversationHandler) GetConversationWithAuth(w http.ResponseWriter, r *http.Request) {
//	// Extrair sessionKey do contexto (adicionado pelo middleware)
//	sessionKey := middleware.GetSessionKey(r)
//	if sessionKey == "" {
//		http.Error(w, "Unauthorized", http.StatusUnauthorized)
//		return
//	}
//
//	// Recuperar sessão pelo whatsapp_session_key
//	session, err := h.sessionRepository.GetBySessionKey(sessionKey)
//	if err != nil || session == nil {
//		h.log.Warnf("Sessão não encontrada para key: %s", sessionKey)
//		http.Error(w, "Session not found", http.StatusNotFound)
//		return
//	}
//
//	// Extrair parâmetros query
//	contactNumber := r.URL.Query().Get("contact")
//	limitStr := r.URL.Query().Get("limit")
//	offsetStr := r.URL.Query().Get("offset")
//	includeMediaStr := r.URL.Query().Get("include_media")
//
//	limit := 50
//	offset := 0
//	includeMedia := false
//
//	if limitStr != "" {
//		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
//			limit = l
//		}
//	}
//
//	if offsetStr != "" {
//		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
//			offset = o
//		}
//	}
//
//	if includeMediaStr == "true" {
//		includeMedia = true
//	}
//
//	messages, total, err := h.messageService.GetConversation(session.ID, contactNumber, limit, offset, includeMedia)
//	if err != nil {
//		h.log.Error("Erro ao recuperar conversa: %v", err)
//		http.Error(w, "Erro ao recuperar conversa", http.StatusInternalServerError)
//		return
//	}
//
//	// Adicionar URLs de download para mensagens com mídia
//	messagesWithURLs := addMediaDownloadURLs(messages)
//
//	response := models.NewSuccessResponse("Histórico de mensagens recuperado", map[string]interface{}{
//		"messages": messagesWithURLs,
//		"pagination": map[string]interface{}{
//			"total":  total,
//			"limit":  limit,
//			"offset": offset,
//		},
//	})
//
//	w.Header().Set("Content-Type", "application/json")
//	json.NewEncoder(w).Encode(response)
//}
//
//// GetContactsWithAuth retorna contatos com autenticação
//// GET /api/v1/messages/contacts
//func (h *ConversationHandler) GetContactsWithAuth(w http.ResponseWriter, r *http.Request) {
//	// Extrair sessionKey do contexto
//	sessionKey := middleware.GetSessionKey(r)
//	if sessionKey == "" {
//		http.Error(w, "Unauthorized", http.StatusUnauthorized)
//		return
//	}
//
//	// Recuperar sessão pelo whatsapp_session_key
//	session, err := h.sessionRepository.GetBySessionKey(sessionKey)
//	if err != nil || session == nil {
//		h.log.Warnf("Sessão não encontrada para key: %s", sessionKey)
//		http.Error(w, "Session not found", http.StatusNotFound)
//		return
//	}
//
//	contacts, err := h.messageService.GetContacts(session.ID)
//	if err != nil {
//		h.log.Error("Erro ao recuperar contatos: %v", err)
//		http.Error(w, "Erro ao recuperar contatos", http.StatusInternalServerError)
//		return
//	}
//
//	if contacts == nil {
//		contacts = []string{}
//	}
//
//	response := models.NewSuccessResponse("Contatos recuperados", map[string]interface{}{
//		"contacts": contacts,
//		"count":    len(contacts),
//	})
//
//	w.Header().Set("Content-Type", "application/json")
//	json.NewEncoder(w).Encode(response)
//}
//
//// GetStatsWithAuth retorna estatísticas com autenticação
//// GET /api/v1/messages/stats
//func (h *ConversationHandler) GetStatsWithAuth(w http.ResponseWriter, r *http.Request) {
//	// Extrair sessionKey do contexto
//	sessionKey := middleware.GetSessionKey(r)
//	if sessionKey == "" {
//		http.Error(w, "Unauthorized", http.StatusUnauthorized)
//		return
//	}
//
//	// Recuperar sessão pelo whatsapp_session_key
//	session, err := h.sessionRepository.GetBySessionKey(sessionKey)
//	if err != nil || session == nil {
//		h.log.Warnf("Sessão não encontrada para key: %s", sessionKey)
//		http.Error(w, "Session not found", http.StatusNotFound)
//		return
//	}
//
//	stats, err := h.messageService.GetMessageStats(session.ID)
//	if err != nil {
//		h.log.Error("Erro ao recuperar estatísticas: %v", err)
//		http.Error(w, "Erro ao recuperar estatísticas", http.StatusInternalServerError)
//		return
//	}
//
//	response := models.NewSuccessResponse("Estatísticas de mensagens", stats)
//
//	w.Header().Set("Content-Type", "application/json")
//	json.NewEncoder(w).Encode(response)
//}
//
//// GetMessageMedia retorna a mídia de uma mensagem em base64
//// GET /api/v1/messages/{messageID}/media
//func (h *ConversationHandler) GetMessageMedia(w http.ResponseWriter, r *http.Request) {
//	messageIDStr := mux.Vars(r)["messageID"]
//	messageID, err := uuid.Parse(messageIDStr)
//	if err != nil {
//		http.Error(w, "Invalid message ID", http.StatusBadRequest)
//		return
//	}
//
//	// Recuperar mensagem do banco
//	message, err := h.messageService.GetMessageByID(messageID)
//	if err != nil || message == nil {
//		h.log.Warnf("Mensagem não encontrada: %s", messageIDStr)
//		http.Error(w, "Message not found", http.StatusNotFound)
//		return
//	}
//
//	// Verificar se é uma mensagem com mídia
//	if message.MediaURL == nil {
//		http.Error(w, "No media found for this message", http.StatusNotFound)
//		return
//	}
//
//	// Tentar recuperar do banco se estiver armazenado
//	if message.MediaBase64Stored != nil && *message.MediaBase64Stored != "" {
//		// Retornar dados em base64 com Content-Type apropriado
//		mimeType := "application/octet-stream"
//		if message.MimeType != nil && *message.MimeType != "" {
//			mimeType = *message.MimeType
//		}
//
//		w.Header().Set("Content-Type", mimeType)
//		w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=media.%s", getExtensionFromMimeType(mimeType)))
//		w.Write([]byte(*message.MediaBase64Stored))
//		return
//	}
//
//	// Fallback: fazer download da URL
//	if message.MediaURL != nil && *message.MediaURL != "" {
//		mediaData, err := h.messageService.DownloadMedia(*message.MediaURL)
//		if err != nil {
//			h.log.Warnf("Erro ao fazer download de mídia: %v", err)
//			http.Error(w, "Failed to download media", http.StatusInternalServerError)
//			return
//		}
//
//		// Retornar arquivo binário diretamente
//		mimeType := mediaData.MimeType
//		if mimeType == "" {
//			mimeType = "application/octet-stream"
//		}
//
//		w.Header().Set("Content-Type", mimeType)
//		w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=media.%s", getExtensionFromMimeType(mimeType)))
//		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(mediaData.Base64)))
//
//		// Decodificar base64 para enviar binário
//		decodedData, err := base64.StdEncoding.DecodeString(mediaData.Base64)
//		if err != nil {
//			http.Error(w, "Failed to decode media", http.StatusInternalServerError)
//			return
//		}
//		w.Write(decodedData)
//		return
//	}
//
//	http.Error(w, "No media found for this message", http.StatusNotFound)
//}
//
//// getExtensionFromMimeType converte MIME type para extensão de arquivo
//func getExtensionFromMimeType(mimeType string) string {
//	switch mimeType {
//	case "image/jpeg":
//		return "jpg"
//	case "image/png":
//		return "png"
//	case "image/gif":
//		return "gif"
//	case "image/webp":
//		return "webp"
//	case "video/mp4":
//		return "mp4"
//	case "audio/mpeg":
//		return "mp3"
//	case "application/pdf":
//		return "pdf"
//	default:
//		return "bin"
//	}
//}
