package models

import "time"

// MessageRequest represents a text message request
type MessageRequest struct {
	Number string `json:"number" validate:"required"`
	Text   string `json:"text" validate:"required"`
}

// MediaRequest represents a media message request
type MediaRequest struct {
	Number      string `json:"number" validate:"required"`
	Caption     string `json:"caption"`
	MediaURL    string `json:"media_url" validate:"required_without=MediaBase64"`
	MediaBase64 string `json:"media_base64" validate:"required_without=MediaURL"`
	MimeType    string `json:"mime_type"`
}

// APIResponse represents a standard API response
type APIResponse struct {
	Status    string      `json:"status"`
	Message   string      `json:"message"`
	Data      interface{} `json:"data,omitempty"`
	Timestamp time.Time   `json:"timestamp"`
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Status    string            `json:"status"`
	Message   string            `json:"message"`
	Code      string            `json:"code"`
	Details   map[string]string `json:"details,omitempty"`
	Timestamp time.Time         `json:"timestamp"`
}

// HealthResponse represents a health check response
type HealthResponse struct {
	Status    string            `json:"status"`
	Service   string            `json:"service"`
	Version   string            `json:"version"`
	Uptime    string            `json:"uptime"`
	Timestamp time.Time         `json:"timestamp"`
	Checks    map[string]string `json:"checks"`
}

// MessageSent represents a successful message send response
type MessageSent struct {
	MessageID string    `json:"message_id,omitempty"`
	Recipient string    `json:"recipient"`
	Type      string    `json:"type"`
	SentAt    time.Time `json:"sent_at"`
}

// NewSuccessResponse creates a new success response
func NewSuccessResponse(message string, data interface{}) *APIResponse {
	return &APIResponse{
		Status:    "success",
		Message:   message,
		Data:      data,
		Timestamp: time.Now(),
	}
}

// NewErrorResponse creates a new error response
func NewErrorResponse(message, code string, details map[string]string) *ErrorResponse {
	return &ErrorResponse{
		Status:    "error",
		Message:   message,
		Code:      code,
		Details:   details,
		Timestamp: time.Now(),
	}
}
