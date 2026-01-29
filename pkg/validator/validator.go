package validator

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
)

func ValidatePhoneNumber(number string) error {
	if number == "" {
		return fmt.Errorf("número de telefone é obrigatório")
	}

	phoneRegex := regexp.MustCompile(`^[0-9]{10,15}$`)
	if !phoneRegex.MatchString(number) {
		return fmt.Errorf("formato de número de telefone inválido")
	}

	return nil
}

func ValidateJSON(r *http.Request, v interface{}) error {
	if r.Body == nil {
		return fmt.Errorf("corpo da requisição vazio")
	}

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(v); err != nil {
		return fmt.Errorf("JSON inválido: %w", err)
	}

	return nil
}

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
