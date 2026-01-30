package models

import (
	"time"

	"github.com/google/uuid"
)

// WhatsAppSession representa uma sessão/instância de WhatsApp
type WhatsAppSession struct {
	ID                  uuid.UUID  `json:"id" db:"id"`
	WhatsAppSessionKey  string     `json:"whatsapp_session_key" db:"whatsapp_session_key"`
	NomePessoa          string     `json:"nome_pessoa" db:"nome_pessoa"`
	EmailPessoa         string     `json:"email_pessoa" db:"email_pessoa"`
	PhoneNumber         string     `json:"phone_number,omitempty" db:"phone_number"`
	DeviceJID           string     `json:"device_jid,omitempty" db:"device_jid"`
	Status              string     `json:"status" db:"status"` // pending, connected, disconnected, error
	QRCode              string     `json:"qr_code,omitempty" db:"qr_code"`
	QRCodeExpiresAt     *time.Time `json:"qr_code_expires_at,omitempty" db:"qr_code_expires_at"`
	CreatedAt           time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at" db:"updated_at"`
	LastConnectedAt     *time.Time `json:"last_connected_at,omitempty" db:"last_connected_at"`
}

// RegisterSessionRequest é a requisição para registrar uma nova sessão
type RegisterSessionRequest struct {
	WhatsAppSessionKey string `json:"whatsappSessionKey" validate:"required"`
	NomePessoa         string `json:"nomePessoa" validate:"required"`
	EmailPessoa        string `json:"emailPessoa" validate:"required,email"`
}

// RegisterSessionResponse é a resposta do registro de sessão
type RegisterSessionResponse struct {
	ID                 uuid.UUID `json:"id"`
	WhatsAppSessionKey string    `json:"whatsapp_session_key"`
	QRCodeBase64       string    `json:"qr_code_base64"`
	Status             string    `json:"status"`
	ExpiresAt          time.Time `json:"expires_at"`
}

// SessionStatus constants
const (
	SessionStatusPending      = "pending"
	SessionStatusConnected    = "connected"
	SessionStatusDisconnected = "disconnected"
	SessionStatusError        = "error"
)

type MessageRequest struct {
	Number string `json:"number" validate:"required"`
	Text   string `json:"text" validate:"required"`
}

type MediaRequest struct {
	Number      string `json:"number" validate:"required"`
	Caption     string `json:"caption"`
	MediaURL    string `json:"media_url" validate:"required_without=MediaBase64"`
	MediaBase64 string `json:"media_base64" validate:"required_without=MediaURL"`
	MimeType    string `json:"mime_type"`
}

type APIResponse struct {
	Status    string      `json:"status"`
	Message   string      `json:"message"`
	Data      interface{} `json:"data,omitempty"`
	Timestamp time.Time   `json:"timestamp"`
}

type ErrorResponse struct {
	Status    string            `json:"status"`
	Message   string            `json:"message"`
	Code      string            `json:"code"`
	Details   map[string]string `json:"details,omitempty"`
	Timestamp time.Time         `json:"timestamp"`
}

type HealthResponse struct {
	Status    string            `json:"status"`
	Service   string            `json:"service"`
	Version   string            `json:"version"`
	Uptime    string            `json:"uptime"`
	Timestamp time.Time         `json:"timestamp"`
	Checks    map[string]string `json:"checks"`
}

type MessageSent struct {
	MessageID string    `json:"message_id,omitempty"`
	Recipient string    `json:"recipient"`
	Type      string    `json:"type"`
	SentAt    time.Time `json:"sent_at"`
}

func NewSuccessResponse(message string, data interface{}) *APIResponse {
	return &APIResponse{
		Status:    "success",
		Message:   message,
		Data:      data,
		Timestamp: time.Now(),
	}
}

func NewErrorResponse(message, code string, details map[string]string) *ErrorResponse {
	return &ErrorResponse{
		Status:    "error",
		Message:   message,
		Code:      code,
		Details:   details,
		Timestamp: time.Now(),
	}
}
