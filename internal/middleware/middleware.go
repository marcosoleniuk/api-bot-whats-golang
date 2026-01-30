package middleware

import (
	"boot-whatsapp-golang/internal/config"
	"boot-whatsapp-golang/internal/models"
	"boot-whatsapp-golang/pkg/logger"
	"context"
	"encoding/json"
	"net/http"
	"time"
)

type contextKey string

const TenantIDKey contextKey = "tenant_id"

func AuthMiddleware(cfg *config.Config, log *logger.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			apiToken := r.Header.Get("apitoken")
			sessionKey := r.Header.Get("SESSIONKEY")

			if apiToken != cfg.Auth.APIToken {
				log.Warnf("Tentativa de acesso não autorizado de %s - Token inválido",
					r.RemoteAddr)

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				err := json.NewEncoder(w).Encode(models.NewErrorResponse(
					"Credenciais de autenticação inválidas",
					"AUTH_INVALID",
					nil,
				))
				if err != nil {
					return
				}
				return
			}

			if sessionKey == "" {
				log.Warnf("Tentativa de acesso sem SESSION_KEY de %s", r.RemoteAddr)

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				err := json.NewEncoder(w).Encode(models.NewErrorResponse(
					"SESSION_KEY é obrigatório",
					"SESSION_KEY_REQUIRED",
					nil,
				))
				if err != nil {
					return
				}
				return
			}

			ctx := context.WithValue(r.Context(), TenantIDKey, sessionKey)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func GetTenantID(r *http.Request) string {
	if tenantID, ok := r.Context().Value(TenantIDKey).(string); ok {
		return tenantID
	}
	return ""
}

func RecoveryMiddleware(log *logger.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					log.Errorf("Panic recuperado: %v", err)

					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusInternalServerError)
					err := json.NewEncoder(w).Encode(models.NewErrorResponse(
						"Erro interno do servidor",
						"INTERNAL_ERROR",
						nil,
					))
					if err != nil {
						return
					}
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

func LoggingMiddleware(log *logger.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

			next.ServeHTTP(rw, r)

			duration := time.Since(start)
			log.Infof("%s %s %d %v", r.Method, r.URL.Path, rw.statusCode, duration)
		})
	}
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func CORSMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, apitoken, SESSIONKEY")

			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func ContentTypeMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			next.ServeHTTP(w, r)
		})
	}
}
