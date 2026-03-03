package models

import (
	"time"

	"github.com/google/uuid"
)

type WhatsAppSession struct {
	ID                 uuid.UUID  `json:"id" db:"id"`
	TenantID           string     `json:"tenant_id" db:"tenant_id"`
	WhatsAppSessionKey string     `json:"whatsapp_session_key" db:"whatsapp_session_key"`
	NomePessoa         string     `json:"nome_pessoa" db:"nome_pessoa"`
	EmailPessoa        string     `json:"email_pessoa" db:"email_pessoa"`
	PhoneNumber        *string    `json:"phone_number,omitempty" db:"phone_number"`
	DeviceJID          *string    `json:"device_jid,omitempty" db:"device_jid"`
	Status             string     `json:"status" db:"status"`
	QRCode             *string    `json:"qr_code,omitempty" db:"qr_code"`
	QRCodeExpiresAt    *time.Time `json:"qr_code_expires_at,omitempty" db:"qr_code_expires_at"`
	CreatedAt          time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at" db:"updated_at"`
	LastConnectedAt    *time.Time `json:"last_connected_at,omitempty" db:"last_connected_at"`
}

type RegisterSessionRequest struct {
	WhatsAppSessionKey string `json:"whatsappSessionKey" validate:"required"`
	NomePessoa         string `json:"nomePessoa" validate:"required"`
	EmailPessoa        string `json:"emailPessoa" validate:"required,email"`
}

type RegisterSessionResponse struct {
	ID                 uuid.UUID `json:"id"`
	WhatsAppSessionKey string    `json:"whatsapp_session_key"`
	QRCodeBase64       string    `json:"qr_code_base64"`
	Status             string    `json:"status"`
	ExpiresAt          time.Time `json:"expires_at"`
}

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

type AudioRequest struct {
	Number      string `json:"number" validate:"required"`
	MediaURL    string `json:"media_url" validate:"required_without=MediaBase64"`
	MediaBase64 string `json:"media_base64" validate:"required_without=MediaURL"`
	MimeType    string `json:"mime_type"`
	PTT         bool   `json:"ptt"`
}

type VideoRequest struct {
	Number      string `json:"number" validate:"required"`
	Caption     string `json:"caption"`
	MediaURL    string `json:"media_url" validate:"required_without=MediaBase64"`
	MediaBase64 string `json:"media_base64" validate:"required_without=MediaURL"`
	MimeType    string `json:"mime_type"`
	GIFPlayback bool   `json:"gif_playback"`
}

type PollRequest struct {
	Number                 string   `json:"number" validate:"required"`
	Question               string   `json:"question" validate:"required"`
	Options                []string `json:"options" validate:"required"`
	SelectableOptionsCount int      `json:"selectable_options_count"`
}

type EventRequest struct {
	Number             string `json:"number" validate:"required"`
	Name               string `json:"name" validate:"required"`
	Description        string `json:"description"`
	JoinLink           string `json:"join_link"`
	StartTime          string `json:"start_time" validate:"required"`
	EndTime            string `json:"end_time"`
	CallType           string `json:"call_type"` // voice|video
	ForceStructured    *bool  `json:"force_structured,omitempty"`
	ExtraGuestsAllowed bool   `json:"extra_guests_allowed"`
	IsScheduleCall     bool   `json:"is_schedule_call"`
	HasReminder        bool   `json:"has_reminder"`
	ReminderOffsetSec  int64  `json:"reminder_offset_sec"`
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

type Webhook struct {
	ID              uuid.UUID         `json:"id" db:"id"`
	SessionID       uuid.UUID         `json:"session_id" db:"session_id"`
	TenantID        string            `json:"tenant_id" db:"tenant_id"`
	URL             string            `json:"url" db:"url"`
	Events          []string          `json:"events" db:"events"`
	Headers         map[string]string `json:"headers,omitempty" db:"headers"`
	Active          bool              `json:"active" db:"active"`
	CreatedAt       time.Time         `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time         `json:"updated_at" db:"updated_at"`
	LastTriggeredAt *time.Time        `json:"last_triggered_at,omitempty" db:"last_triggered_at"`
}

type RegisterWebhookRequest struct {
	URL     string            `json:"url" validate:"required,http_url"`
	Events  []string          `json:"events" validate:"required"`
	Headers map[string]string `json:"headers,omitempty"`
}

type WebhookPayload struct {
	Event      string      `json:"event"`
	SessionKey string      `json:"session_key"`
	TenantID   string      `json:"tenant_id"`
	Timestamp  time.Time   `json:"timestamp"`
	Data       interface{} `json:"data"`
}

type MessageWebhookData struct {
	MessageID   string    `json:"message_id,omitempty"`
	MessageIDs  []string  `json:"message_ids,omitempty"`
	ReceiptType string    `json:"receipt_type,omitempty"`
	Chat        string    `json:"chat,omitempty"`
	IsGroup     *bool     `json:"is_group,omitempty"`
	PollID      string    `json:"poll_id,omitempty"`
	PollName    string    `json:"poll_name,omitempty"`
	PollOptions []string  `json:"poll_options,omitempty"`
	PollVotes   []string  `json:"poll_votes,omitempty"`
	PollHashes  []string  `json:"poll_hashes,omitempty"`
	From        string    `json:"from,omitempty"`
	To          string    `json:"to,omitempty"`
	Type        string    `json:"type"`
	Text        string    `json:"text,omitempty"`
	Caption     string    `json:"caption,omitempty"`
	MediaURL    string    `json:"media_url,omitempty"`
	MimeType    string    `json:"mime_type,omitempty"`
	Timestamp   time.Time `json:"timestamp"`
}

type WebhookLog struct {
	ID              uuid.UUID `json:"id" db:"id"`
	WebhookID       uuid.UUID `json:"webhook_id" db:"webhook_id"`
	EventType       string    `json:"event_type" db:"event_type"`
	Payload         string    `json:"payload" db:"payload"`
	HTTPStatus      *int      `json:"http_status,omitempty" db:"http_status"`
	ResponseBody    *string   `json:"response_body,omitempty" db:"response_body"`
	ErrorMessage    *string   `json:"error_message,omitempty" db:"error_message"`
	ExecutionTimeMS *int      `json:"execution_time_ms,omitempty" db:"execution_time_ms"`
	TriggeredAt     time.Time `json:"triggered_at" db:"triggered_at"`
}

type Message struct {
	ID                uuid.UUID `json:"id" db:"id"`
	SessionID         uuid.UUID `json:"session_id" db:"session_id"`
	TenantID          string    `json:"tenant_id" db:"tenant_id"`
	WhatsAppMessageID *string   `json:"whatsapp_message_id,omitempty" db:"whatsapp_message_id"`
	Direction         string    `json:"direction" db:"direction"`
	Sender            *string   `json:"sender,omitempty" db:"sender"`
	Recipient         *string   `json:"recipient,omitempty" db:"recipient"`
	MessageType       string    `json:"message_type" db:"message_type"`
	Content           *string   `json:"content,omitempty" db:"content"`
	MediaURL          *string   `json:"media_url,omitempty" db:"media_url"`
	MimeType          *string   `json:"mime_type,omitempty" db:"mime_type"`
	MediaBase64Stored *[]byte   `json:"media_base64_stored,omitempty" db:"media_base64"`
	MediaBase64       *string   `json:"media_base64,omitempty" db:"-"`
	Status            string    `json:"status" db:"status"`
	CreatedAt         time.Time `json:"created_at" db:"created_at"`
	UpdatedAt         time.Time `json:"updated_at" db:"updated_at"`
}

type GetConversationRequest struct {
	ContactNumber string `json:"contact_number,omitempty"`
	Limit         int    `json:"limit,omitempty"`
	Offset        int    `json:"offset,omitempty"`
}

type MessageResponse struct {
	MessageID string    `json:"message_id"`
	Status    string    `json:"status"`
	SentAt    time.Time `json:"sent_at"`
}

type PollMetadata struct {
	ID            uuid.UUID `json:"id" db:"id"`
	SessionID     uuid.UUID `json:"session_id" db:"session_id"`
	TenantID      string    `json:"tenant_id" db:"tenant_id"`
	ChatJID       string    `json:"chat_jid" db:"chat_jid"`
	PollMessageID string    `json:"poll_message_id" db:"poll_message_id"`
	Question      string    `json:"question" db:"question"`
	Options       []string  `json:"options" db:"options"`
	CreatedAt     time.Time `json:"created_at" db:"created_at"`
	UpdatedAt     time.Time `json:"updated_at" db:"updated_at"`
}
