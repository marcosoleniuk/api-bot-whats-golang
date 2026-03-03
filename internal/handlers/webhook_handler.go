package handlers

import (
	"boot-whatsapp-golang/internal/middleware"
	"boot-whatsapp-golang/internal/models"
	"boot-whatsapp-golang/internal/repository"
	"boot-whatsapp-golang/internal/services"
	"boot-whatsapp-golang/pkg/logger"
	"boot-whatsapp-golang/pkg/validator"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

type WebhookHandler struct {
	webhookService    *services.WebhookService
	sessionRepository *repository.SessionRepository
	logger            *logger.Logger
}

func NewWebhookHandler(
	webhookService *services.WebhookService,
	sessionRepo *repository.SessionRepository,
	log *logger.Logger,
) *WebhookHandler {
	return &WebhookHandler{
		webhookService:    webhookService,
		sessionRepository: sessionRepo,
		logger:            log,
	}
}

func (h *WebhookHandler) writeJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func (h *WebhookHandler) writeError(
	w http.ResponseWriter,
	status int,
	message string,
	code string,
	details any,
) {
	var detailsMap map[string]string
	if details != nil {
		if dm, ok := details.(map[string]string); ok {
			detailsMap = dm
		}
	}
	h.writeJSON(w, status, models.NewErrorResponse(message, code, detailsMap))
}

func (h *WebhookHandler) parseUUIDParam(
	w http.ResponseWriter,
	r *http.Request,
	param string,
) (uuid.UUID, bool) {

	idStr := mux.Vars(r)[param]
	id, err := uuid.Parse(idStr)
	if err != nil {
		h.logger.Warnf("ID inválido: %s", idStr)
		h.writeError(
			w,
			http.StatusBadRequest,
			"ID inválido",
			"INVALID_ID",
			map[string]string{param: idStr},
		)
		return uuid.Nil, false
	}
	return id, true
}

func (h *WebhookHandler) getSessionContext(
	w http.ResponseWriter,
	r *http.Request,
) (sessionID uuid.UUID, tenantID string, sessionKey string, ok bool) {

	sessionKey = r.Header.Get("X-WhatsApp-Session-Key")
	if sessionKey == "" {
		h.writeError(
			w,
			http.StatusBadRequest,
			"Header X-WhatsApp-Session-Key é obrigatório",
			"MISSING_SESSION_KEY",
			nil,
		)
		return uuid.Nil, "", "", false
	}

	tenantID = middleware.GetTenantID(r)
	if tenantID == "" {
		h.writeError(
			w,
			http.StatusUnauthorized,
			"Não autorizado",
			"UNAUTHORIZED",
			nil,
		)
		return uuid.Nil, "", "", false
	}

	id, err := h.sessionRepository.GetIDBySessionKey(sessionKey)
	if err != nil {
		h.writeError(
			w,
			http.StatusNotFound,
			"Sessão não encontrada",
			"SESSION_NOT_FOUND",
			map[string]string{"session_key": sessionKey},
		)
		return uuid.Nil, "", "", false
	}

	return id, tenantID, sessionKey, true
}

func (h *WebhookHandler) RegisterWebhook(w http.ResponseWriter, r *http.Request) {

	sessionID, tenantID, sessionKey, ok := h.getSessionContext(w, r)
	if !ok {
		return
	}

	var req models.RegisterWebhookRequest
	if err := validator.ValidateJSON(r, &req); err != nil {
		h.writeError(
			w,
			http.StatusBadRequest,
			"Corpo da requisição inválido",
			"INVALID_JSON",
			map[string]string{"error": err.Error()},
		)
		return
	}

	if req.URL == "" {
		h.writeError(w, http.StatusBadRequest, "URL é obrigatória", "MISSING_URL", nil)
		return
	}

	if len(req.Events) == 0 {
		h.writeError(w, http.StatusBadRequest, "Pelo menos um evento é obrigatório", "MISSING_EVENTS", nil)
		return
	}

	webhook, err := h.webhookService.RegisterWebhook(sessionID, tenantID, &req)
	if err != nil {
		h.logger.Errorf("[%s] Falha ao registrar webhook: %v", sessionKey, err)
		h.writeError(w, http.StatusInternalServerError, "Falha ao registrar webhook", "REGISTRATION_FAILED", err.Error())
		return
	}

	h.writeJSON(w, http.StatusCreated,
		models.NewSuccessResponse("Webhook registrado com sucesso", webhook))
}

func (h *WebhookHandler) ListWebhooks(w http.ResponseWriter, r *http.Request) {

	sessionID, tenantID, _, ok := h.getSessionContext(w, r)
	if !ok {
		return
	}

	webhooks, err := h.webhookService.GetWebhooks(sessionID, tenantID)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "Falha ao listar webhooks", "LIST_FAILED", err.Error())
		return
	}

	if webhooks == nil {
		webhooks = []*models.Webhook{}
	}

	h.writeJSON(w, http.StatusOK,
		models.NewSuccessResponse("Webhooks listados com sucesso", webhooks))
}

func (h *WebhookHandler) GetWebhook(w http.ResponseWriter, r *http.Request) {

	webhookID, ok := h.parseUUIDParam(w, r, "webhookId")
	if !ok {
		return
	}

	webhook, err := h.webhookService.GetWebhookByID(webhookID)
	if err != nil {
		h.writeError(w, http.StatusNotFound, "Webhook não encontrado", "NOT_FOUND", nil)
		return
	}

	h.writeJSON(w, http.StatusOK,
		models.NewSuccessResponse("Webhook obtido com sucesso", webhook))
}

func (h *WebhookHandler) UpdateWebhook(w http.ResponseWriter, r *http.Request) {

	webhookID, ok := h.parseUUIDParam(w, r, "webhookId")
	if !ok {
		return
	}

	var req models.RegisterWebhookRequest
	if err := validator.ValidateJSON(r, &req); err != nil {
		h.writeError(w, http.StatusBadRequest, "Corpo inválido", "INVALID_JSON", err.Error())
		return
	}

	webhook, err := h.webhookService.UpdateWebhook(webhookID, &req)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "Falha ao atualizar webhook", "UPDATE_FAILED", err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK,
		models.NewSuccessResponse("Webhook atualizado com sucesso", webhook))
}

func (h *WebhookHandler) DeleteWebhook(w http.ResponseWriter, r *http.Request) {

	webhookID, ok := h.parseUUIDParam(w, r, "webhookId")
	if !ok {
		return
	}

	if err := h.webhookService.DeleteWebhook(webhookID); err != nil {
		h.writeError(w, http.StatusInternalServerError, "Falha ao deletar webhook", "DELETE_FAILED", err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK,
		models.NewSuccessResponse("Webhook deletado com sucesso", nil))
}

func (h *WebhookHandler) GetWebhookLogs(w http.ResponseWriter, r *http.Request) {

	webhookID, ok := h.parseUUIDParam(w, r, "webhookId")
	if !ok {
		return
	}

	limit := 50
	if l, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && l > 0 {
		limit = l
	}

	logs, err := h.webhookService.GetWebhookLogs(webhookID, limit)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "Falha ao buscar logs", "LOGS_FAILED", err.Error())
		return
	}

	if logs == nil {
		logs = []*models.WebhookLog{}
	}

	h.writeJSON(w, http.StatusOK,
		models.NewSuccessResponse("Logs obtidos com sucesso", logs))
}
