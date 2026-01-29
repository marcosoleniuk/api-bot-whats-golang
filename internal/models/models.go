package models

import "time"

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
