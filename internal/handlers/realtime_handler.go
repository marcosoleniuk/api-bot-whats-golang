package handlers

import (
	"boot-whatsapp-golang/internal/middleware"
	"boot-whatsapp-golang/internal/models"
	"boot-whatsapp-golang/internal/services"
	"boot-whatsapp-golang/pkg/logger"
	"encoding/json"
	"net/http"
	"strings"
)

type RealtimeHandler struct {
	realtimeService *services.RealtimeService
	log             *logger.Logger
}

func NewRealtimeHandler(rs *services.RealtimeService, log *logger.Logger) *RealtimeHandler {
	return &RealtimeHandler{
		realtimeService: rs,
		log:             log,
	}
}

func (h *RealtimeHandler) Connect(w http.ResponseWriter, r *http.Request) {
	if h.realtimeService == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(models.NewErrorResponse(
			"WebSocket indisponível",
			"WEBSOCKET_UNAVAILABLE",
			nil,
		))
		return
	}

	authScope := strings.TrimSpace(middleware.GetSessionKey(r))
	if authScope == "" {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(models.NewErrorResponse(
			"SESSIONKEY é obrigatório para WebSocket",
			"SESSION_KEY_REQUIRED",
			nil,
		))
		return
	}

	scopes := []string{authScope}

	if sessionScope := strings.TrimSpace(r.URL.Query().Get("whatsapp_session_key")); sessionScope != "" {
		scopes = append(scopes, sessionScope)
	}

	if rawScopes := strings.TrimSpace(r.URL.Query().Get("scopes")); rawScopes != "" {
		for _, value := range strings.Split(rawScopes, ",") {
			if scope := strings.TrimSpace(value); scope != "" {
				scopes = append(scopes, scope)
			}
		}
	}

	if err := h.realtimeService.ServeWS(w, r, scopes); err != nil {
		h.log.Warnf("Falha no handshake websocket: %v", err)
		return
	}
}
