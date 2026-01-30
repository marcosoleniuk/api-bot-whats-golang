package handlers

import (
	"boot-whatsapp-golang/internal/middleware"
	"boot-whatsapp-golang/internal/models"
	"boot-whatsapp-golang/internal/services"
	"boot-whatsapp-golang/pkg/logger"
	"boot-whatsapp-golang/pkg/validator"
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
)

type SessionHandler struct {
	service *services.MultiTenantWhatsAppService
	logger  *logger.Logger
}

func NewSessionHandler(service *services.MultiTenantWhatsAppService, log *logger.Logger) *SessionHandler {
	return &SessionHandler{service: service, logger: log}
}

func (h *SessionHandler) writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func (h *SessionHandler) errorJSON(
	w http.ResponseWriter,
	status int,
	message, code string,
	details map[string]string,
) {
	h.writeJSON(w, status, models.NewErrorResponse(message, code, details))
}

func (h *SessionHandler) successJSON(w http.ResponseWriter, status int, message string, data any) {
	h.writeJSON(w, status, models.NewSuccessResponse(message, data))
}

func (h *SessionHandler) requireTenantID(w http.ResponseWriter, r *http.Request) (string, bool) {
	tenantID := middleware.GetTenantID(r)
	if tenantID == "" {
		h.logger.Error("TenantID não encontrado no contexto")
		h.errorJSON(w, http.StatusUnauthorized, "Não autorizado", "UNAUTHORIZED", nil)
		return "", false
	}
	return tenantID, true
}

func (h *SessionHandler) pathVar(r *http.Request, key string) string {
	return mux.Vars(r)[key]
}

func (h *SessionHandler) RegisterSession(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := h.requireTenantID(w, r)
	if !ok {
		return
	}

	var req models.RegisterSessionRequest
	if err := validator.ValidateJSON(r, &req); err != nil {
		h.logger.Warnf("JSON inválido na requisição de registro: %v", err)
		h.errorJSON(
			w,
			http.StatusBadRequest,
			"Corpo da requisição inválido",
			"INVALID_JSON",
			map[string]string{"error": err.Error()},
		)
		return
	}

	if req.WhatsAppSessionKey == "" || req.NomePessoa == "" || req.EmailPessoa == "" {
		h.logger.Warn("Campos obrigatórios ausentes na requisição de registro")
		h.errorJSON(
			w,
			http.StatusBadRequest,
			"Campos obrigatórios ausentes",
			"VALIDATION_ERROR",
			map[string]string{
				"whatsappSessionKey": "obrigatório",
				"nomePessoa":         "obrigatório",
				"emailPessoa":        "obrigatório",
			},
		)
		return
	}

	h.logger.Infof("Registrando nova sessão: %s (%s) [Tenant: %s]", req.WhatsAppSessionKey, req.EmailPessoa, tenantID)

	response, err := h.service.RegisterSession(&req, tenantID)
	if err != nil {
		h.logger.Errorf("Falha ao registrar sessão: %v", err)
		h.errorJSON(
			w,
			http.StatusInternalServerError,
			"Falha ao registrar sessão",
			"REGISTRATION_FAILED",
			map[string]string{"error": err.Error()},
		)
		return
	}

	h.successJSON(
		w,
		http.StatusCreated,
		"Sessão registrada com sucesso. Escaneie o QR code para conectar.",
		response,
	)
}

func (h *SessionHandler) GetQRCode(w http.ResponseWriter, r *http.Request) {
	sessionKey := h.pathVar(r, "sessionKey")

	tenantID, ok := h.requireTenantID(w, r)
	if !ok {
		return
	}

	if sessionKey == "" {
		h.errorJSON(w, http.StatusBadRequest, "sessionKey é obrigatório", "VALIDATION_ERROR", nil)
		return
	}

	qrCode, err := h.service.GetQRCode(sessionKey, tenantID)
	if err != nil {
		if err.Error() == "SESSION_ALREADY_CONNECTED" {
			h.successJSON(
				w,
				http.StatusOK,
				"Sessão ativa",
				map[string]interface{}{
					"message":          "Sessão já está conectada ao WhatsApp",
					"session_key":      sessionKey,
					"status":           "connected",
					"qr_code_required": false,
				},
			)
			return
		}

		h.logger.Warnf("Falha ao obter QR code para %s: %v", sessionKey, err)
		h.errorJSON(
			w,
			http.StatusNotFound,
			"Falha ao obter QR code",
			"QRCODE_NOT_FOUND",
			map[string]string{"error": err.Error()},
		)
		return
	}

	h.successJSON(
		w,
		http.StatusOK,
		"QR code obtido com sucesso",
		map[string]interface{}{
			"qr_code_base64": qrCode,
			"session_key":    sessionKey,
		},
	)
}

func (h *SessionHandler) ListSessions(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := h.requireTenantID(w, r)
	if !ok {
		return
	}

	sessions, err := h.service.ListSessionsByTenant(tenantID)
	if err != nil {
		h.logger.Errorf("Falha ao listar sessões do tenant %s: %v", tenantID, err)
		h.errorJSON(
			w,
			http.StatusInternalServerError,
			"Falha ao listar sessões",
			"LIST_FAILED",
			map[string]string{"error": err.Error()},
		)
		return
	}

	type SessionListItem struct {
		ID                 string  `json:"id"`
		WhatsAppSessionKey string  `json:"whatsapp_session_key"`
		NomePessoa         string  `json:"nome_pessoa"`
		EmailPessoa        string  `json:"email_pessoa"`
		PhoneNumber        *string `json:"phone_number,omitempty"`
		Status             string  `json:"status"`
		CreatedAt          string  `json:"created_at"`
		LastConnectedAt    *string `json:"last_connected_at,omitempty"`
	}

	list := make([]SessionListItem, 0, len(sessions))
	for _, s := range sessions {
		item := SessionListItem{
			ID:                 s.ID.String(),
			WhatsAppSessionKey: s.WhatsAppSessionKey,
			NomePessoa:         s.NomePessoa,
			EmailPessoa:        s.EmailPessoa,
			PhoneNumber:        s.PhoneNumber,
			Status:             s.Status,
			CreatedAt:          s.CreatedAt.Format("2006-01-02 15:04:05"),
		}
		if s.LastConnectedAt != nil {
			lastConn := s.LastConnectedAt.Format("2006-01-02 15:04:05")
			item.LastConnectedAt = &lastConn
		}
		list = append(list, item)
	}

	h.successJSON(
		w,
		http.StatusOK,
		"Sessões listadas com sucesso",
		map[string]interface{}{
			"total":    len(list),
			"sessions": list,
		},
	)
}

func (h *SessionHandler) DisconnectSession(w http.ResponseWriter, r *http.Request) {
	sessionKey := h.pathVar(r, "sessionKey")
	if sessionKey == "" {
		h.errorJSON(w, http.StatusBadRequest, "sessionKey é obrigatório", "VALIDATION_ERROR", nil)
		return
	}

	tenantID, ok := h.requireTenantID(w, r)
	if !ok {
		return
	}

	h.logger.Infof("Desconectando sessão: %s [Tenant: %s]", sessionKey, tenantID)

	if err := h.service.DisconnectSession(sessionKey, tenantID); err != nil {
		h.logger.Errorf("Falha ao desconectar sessão %s: %v", sessionKey, err)
		h.errorJSON(
			w,
			http.StatusInternalServerError,
			"Falha ao desconectar sessão",
			"DISCONNECT_FAILED",
			map[string]string{"error": err.Error()},
		)
		return
	}

	h.successJSON(
		w,
		http.StatusOK,
		"Sessão desconectada com sucesso",
		map[string]string{
			"session_key": sessionKey,
			"status":      "disconnected",
		},
	)
}

func (h *SessionHandler) DeleteSession(w http.ResponseWriter, r *http.Request) {
	sessionKey := h.pathVar(r, "sessionKey")
	if sessionKey == "" {
		h.errorJSON(w, http.StatusBadRequest, "sessionKey é obrigatório", "VALIDATION_ERROR", nil)
		return
	}

	tenantID, ok := h.requireTenantID(w, r)
	if !ok {
		return
	}

	h.logger.Infof("Deletando sessão: %s [Tenant: %s]", sessionKey, tenantID)

	if err := h.service.DeleteSession(sessionKey, tenantID); err != nil {
		h.logger.Errorf("Falha ao deletar sessão %s: %v", sessionKey, err)
		h.errorJSON(
			w,
			http.StatusInternalServerError,
			"Falha ao deletar sessão",
			"DELETE_FAILED",
			map[string]string{"error": err.Error()},
		)
		return
	}

	h.successJSON(
		w,
		http.StatusOK,
		"Sessão deletada com sucesso",
		map[string]string{
			"session_key": sessionKey,
		},
	)
}
