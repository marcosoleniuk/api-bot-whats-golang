package handlers

import (
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
	return &SessionHandler{
		service: service,
		logger:  log,
	}
}

func (h *SessionHandler) RegisterSession(w http.ResponseWriter, r *http.Request) {
	var req models.RegisterSessionRequest

	if err := validator.ValidateJSON(r, &req); err != nil {
		h.logger.Warnf("JSON inválido na requisição de registro: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		err := json.NewEncoder(w).Encode(models.NewErrorResponse(
			"Corpo da requisição inválido",
			"INVALID_JSON",
			map[string]string{"error": err.Error()},
		))
		if err != nil {
			return
		}
		return
	}

	if req.WhatsAppSessionKey == "" || req.NomePessoa == "" || req.EmailPessoa == "" {
		h.logger.Warn("Campos obrigatórios ausentes na requisição de registro")
		w.WriteHeader(http.StatusBadRequest)
		err := json.NewEncoder(w).Encode(models.NewErrorResponse(
			"Campos obrigatórios ausentes",
			"VALIDATION_ERROR",
			map[string]string{
				"whatsappSessionKey": "obrigatório",
				"nomePessoa":         "obrigatório",
				"emailPessoa":        "obrigatório",
			},
		))
		if err != nil {
			return
		}
		return
	}

	h.logger.Infof("Registrando nova sessão: %s (%s)", req.WhatsAppSessionKey, req.EmailPessoa)

	response, err := h.service.RegisterSession(&req)
	if err != nil {
		h.logger.Errorf("Falha ao registrar sessão: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		err := json.NewEncoder(w).Encode(models.NewErrorResponse(
			"Falha ao registrar sessão",
			"REGISTRATION_FAILED",
			map[string]string{"error": err.Error()},
		))
		if err != nil {
			return
		}
		return
	}

	w.WriteHeader(http.StatusCreated)
	err = json.NewEncoder(w).Encode(models.NewSuccessResponse(
		"Sessão registrada com sucesso. Escaneie o QR code para conectar.",
		response,
	))
	if err != nil {
		return
	}
}

func (h *SessionHandler) GetQRCode(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	sessionKey := vars["sessionKey"]

	if sessionKey == "" {
		w.WriteHeader(http.StatusBadRequest)
		err := json.NewEncoder(w).Encode(models.NewErrorResponse(
			"sessionKey é obrigatório",
			"VALIDATION_ERROR",
			nil,
		))
		if err != nil {
			return
		}
		return
	}

	qrCode, err := h.service.GetQRCode(sessionKey)
	if err != nil {
		h.logger.Warnf("Falha ao obter QR code para %s: %v", sessionKey, err)
		w.WriteHeader(http.StatusNotFound)
		err := json.NewEncoder(w).Encode(models.NewErrorResponse(
			"Falha ao obter QR code",
			"QRCODE_NOT_FOUND",
			map[string]string{"error": err.Error()},
		))
		if err != nil {
			return
		}
		return
	}

	w.WriteHeader(http.StatusOK)
	err = json.NewEncoder(w).Encode(models.NewSuccessResponse(
		"QR code obtido com sucesso",
		map[string]interface{}{
			"qr_code_base64": qrCode,
			"session_key":    sessionKey,
		},
	))
	if err != nil {
		return
	}
}

func (h *SessionHandler) ListSessions(w http.ResponseWriter, r *http.Request) {
	sessions, err := h.service.ListSessions()
	if err != nil {
		h.logger.Errorf("Falha ao listar sessões: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		err := json.NewEncoder(w).Encode(models.NewErrorResponse(
			"Falha ao listar sessões",
			"LIST_FAILED",
			map[string]string{"error": err.Error()},
		))
		if err != nil {
			return
		}
		return
	}

	type SessionListItem struct {
		ID                 string  `json:"id"`
		WhatsAppSessionKey string  `json:"whatsapp_session_key"`
		NomePessoa         string  `json:"nome_pessoa"`
		EmailPessoa        string  `json:"email_pessoa"`
		PhoneNumber        string  `json:"phone_number,omitempty"`
		Status             string  `json:"status"`
		CreatedAt          string  `json:"created_at"`
		LastConnectedAt    *string `json:"last_connected_at,omitempty"`
	}

	var list []SessionListItem
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

	w.WriteHeader(http.StatusOK)
	err = json.NewEncoder(w).Encode(models.NewSuccessResponse(
		"Sessões listadas com sucesso",
		map[string]interface{}{
			"total":    len(list),
			"sessions": list,
		},
	))
	if err != nil {
		return
	}
}

func (h *SessionHandler) DisconnectSession(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	sessionKey := vars["sessionKey"]

	if sessionKey == "" {
		w.WriteHeader(http.StatusBadRequest)
		err := json.NewEncoder(w).Encode(models.NewErrorResponse(
			"sessionKey é obrigatório",
			"VALIDATION_ERROR",
			nil,
		))
		if err != nil {
			return
		}
		return
	}

	h.logger.Infof("Desconectando sessão: %s", sessionKey)

	if err := h.service.DisconnectSession(sessionKey); err != nil {
		h.logger.Errorf("Falha ao desconectar sessão %s: %v", sessionKey, err)
		w.WriteHeader(http.StatusInternalServerError)
		err := json.NewEncoder(w).Encode(models.NewErrorResponse(
			"Falha ao desconectar sessão",
			"DISCONNECT_FAILED",
			map[string]string{"error": err.Error()},
		))
		if err != nil {
			return
		}
		return
	}

	w.WriteHeader(http.StatusOK)
	err := json.NewEncoder(w).Encode(models.NewSuccessResponse(
		"Sessão desconectada com sucesso",
		map[string]string{
			"session_key": sessionKey,
			"status":      "disconnected",
		},
	))
	if err != nil {
		return
	}
}

func (h *SessionHandler) DeleteSession(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	sessionKey := vars["sessionKey"]

	if sessionKey == "" {
		w.WriteHeader(http.StatusBadRequest)
		err := json.NewEncoder(w).Encode(models.NewErrorResponse(
			"sessionKey é obrigatório",
			"VALIDATION_ERROR",
			nil,
		))
		if err != nil {
			return
		}
		return
	}

	h.logger.Infof("Deletando sessão: %s", sessionKey)

	if err := h.service.DeleteSession(sessionKey); err != nil {
		h.logger.Errorf("Falha ao deletar sessão %s: %v", sessionKey, err)
		w.WriteHeader(http.StatusInternalServerError)
		err := json.NewEncoder(w).Encode(models.NewErrorResponse(
			"Falha ao deletar sessão",
			"DELETE_FAILED",
			map[string]string{"error": err.Error()},
		))
		if err != nil {
			return
		}
		return
	}

	w.WriteHeader(http.StatusOK)
	err := json.NewEncoder(w).Encode(models.NewSuccessResponse(
		"Sessão deletada com sucesso",
		map[string]string{
			"session_key": sessionKey,
		},
	))
	if err != nil {
		return
	}
}
