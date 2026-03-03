package media

import (
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type DataDrive struct {
	Base64   string
	MimeType string
}

func URLToMediaData(url string) (*DataDrive, error) {
	client := &http.Client{
		Timeout: 15 * time.Second,
	}

	var resp *http.Response
	var err error

	for attempt := 1; attempt <= 2; attempt++ {
		resp, err = client.Get(url)
		if err == nil && resp.StatusCode == http.StatusOK {
			break
		}
		if err != nil {
			fmt.Printf("Tentativa %d: Erro ao baixar URL: %v\n", attempt, err)
		} else {
			fmt.Printf("Tentativa %d: Status code %d para URL: %s\n", attempt, resp.StatusCode, url)
		}

		if resp != nil {
			err := resp.Body.Close()
			if err != nil {
				return nil, err
			}
		}

		if attempt == 1 {
			time.Sleep(1 * time.Second)
		}
	}

	if err != nil {
		return nil, fmt.Errorf("falha ao baixar mídia após 2 tentativas: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		defer func(Body io.ReadCloser) {
			err := Body.Close()
			if err != nil {
				fmt.Printf("Erro ao fechar response body: %v\n", err)
			}
		}(resp.Body)
		return nil, fmt.Errorf("status code inválido: %d", resp.StatusCode)
	}

	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			fmt.Printf("Erro ao fechar response body: %v\n", err)
		}
	}(resp.Body)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("erro ao ler response: %w", err)
	}

	encoded := base64.StdEncoding.EncodeToString(body)

	mimeType := resp.Header.Get("Content-Type")

	if idx := strings.Index(mimeType, ";"); idx != -1 {
		mimeType = mimeType[:idx]
	}

	if mimeType == "" || mimeType == "application/octet-stream" {
		mimeType = detectMimeType(body)
	}

	return &DataDrive{
		Base64:   encoded,
		MimeType: mimeType,
	}, nil
}

func detectMimeType(data []byte) string {
	if len(data) < 4 {
		return http.DetectContentType(data)
	}

	if data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF {
		return "image/jpeg"
	}
	if len(data) >= 8 && data[0] == 0x89 && data[1] == 0x50 && data[2] == 0x4E && data[3] == 0x47 {
		return "image/png"
	}
	if data[0] == 0x47 && data[1] == 0x49 && data[2] == 0x46 {
		return "image/gif"
	}
	if len(data) >= 12 && data[0] == 0x52 && data[1] == 0x49 && data[2] == 0x46 && data[3] == 0x46 &&
		data[8] == 0x57 && data[9] == 0x45 && data[10] == 0x42 && data[11] == 0x50 {
		return "image/webp"
	}
	if len(data) >= 12 && ((data[4] == 0x66 && data[5] == 0x74 && data[6] == 0x79 && data[7] == 0x70) ||
		(data[4] == 0x6D && data[5] == 0x64 && data[6] == 0x61 && data[7] == 0x74)) {
		return "video/mp4"
	}
	if data[0] == 0x49 && data[1] == 0x44 && data[2] == 0x33 {
		return "audio/mpeg"
	}
	if data[0] == 0xFF && data[1] == 0xFB {
		return "audio/mpeg"
	}
	if data[0] == 0x25 && data[1] == 0x50 && data[2] == 0x44 && data[3] == 0x46 {
		return "application/pdf"
	}

	detected := http.DetectContentType(data)
	if idx := strings.Index(detected, ";"); idx != -1 {
		detected = detected[:idx]
	}
	if detected == "" {
		return "application/octet-stream"
	}
	return detected
}
func IsMediaMessage(messageType string) bool {
	switch messageType {
	case "image", "video", "document", "audio":
		return true
	default:
		return false
	}
}
