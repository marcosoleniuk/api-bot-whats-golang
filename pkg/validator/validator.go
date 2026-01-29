package validator

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
)

// ValidatePhoneNumber validates a phone number
func ValidatePhoneNumber(number string) error {
	if number == "" {
		return fmt.Errorf("phone number is required")
	}
	
	// Remove common formatting characters
	phoneRegex := regexp.MustCompile(`^[0-9]{10,15}$`)
	if !phoneRegex.MatchString(number) {
		return fmt.Errorf("invalid phone number format")
	}
	
	return nil
}

// ValidateJSON validates and decodes JSON from request body
func ValidateJSON(r *http.Request, v interface{}) error {
	if r.Body == nil {
		return fmt.Errorf("empty request body")
	}
	
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	
	if err := decoder.Decode(v); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	
	return nil
}

// ValidateMimeType validates a MIME type
func ValidateMimeType(mimeType string) bool {
	validTypes := []string{
		"image/jpeg", "image/jpg", "image/png", "image/gif", "image/webp",
		"video/mp4", "video/mpeg", "video/quicktime",
		"audio/mpeg", "audio/ogg", "audio/wav",
		"application/pdf", "application/msword",
		"application/vnd.openxmlformats-officedocument.wordprocessingml.document",
	}
	
	for _, valid := range validTypes {
		if mimeType == valid {
			return true
		}
	}
	
	return false
}
