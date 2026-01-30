package services

import (
	"boot-whatsapp-golang/internal/config"
	"boot-whatsapp-golang/pkg/logger"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mdp/qrterminal/v3"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	_ "go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"google.golang.org/protobuf/proto"
)

type WhatsAppService struct {
	client *whatsmeow.Client
	config *config.Config
	logger *logger.Logger
}

func (s *WhatsAppService) Connect() error {
	if s.client.IsConnected() {
		s.logger.Info("Já conectado ao WhatsApp")
		return nil
	}

	if s.client.Store.ID == nil {
		qrChan, _ := s.client.GetQRChannel(context.Background())

		if err := s.client.Connect(); err != nil {
			return fmt.Errorf("falha ao conectar: %w", err)
		}

		s.logger.Info("Aguardando leitura do QR Code...")

		for evt := range qrChan {
			if evt.Event == "code" {
				if s.config.WhatsApp.QRCodeGenerate {
					s.logger.Info("Escaneie o QR Code abaixo:")
					qrterminal.GenerateHalfBlock(evt.Code, qrterminal.L, os.Stdout)
				} else {
					s.logger.Infof("QR Code: %s", evt.Code)
				}
			} else {
				s.logger.Infof("Evento de login: %s", evt.Event)
			}
		}

		s.logger.Info("Conectado ao WhatsApp com sucesso")
	} else {
		if err := s.client.Connect(); err != nil {
			return fmt.Errorf("falha ao reconectar: %w", err)
		}
		s.logger.Info("Reconectado ao WhatsApp com sessão existente")
	}

	return nil
}

func (s *WhatsAppService) Disconnect() {
	if s.client != nil && s.client.IsConnected() {
		s.client.Disconnect()
		s.logger.Info("Desconectado do WhatsApp")
	}
}

func (s *WhatsAppService) IsConnected() bool {
	return s.client != nil && s.client.IsConnected()
}

func (s *WhatsAppService) SendTextMessage(number, text string) error {
	jid, err := s.parsePhoneNumber(number)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, err = s.client.SendMessage(ctx, jid, &waE2E.Message{
		ExtendedTextMessage: &waE2E.ExtendedTextMessage{
			Text: proto.String(text),
		},
	})

	if err != nil {
		return fmt.Errorf("falha ao enviar mensagem: %w", err)
	}

	s.logger.Infof("Mensagem de texto enviada para %s", number)
	return nil
}

func (s *WhatsAppService) SendMediaMessage(number, caption, mediaURL, mediaBase64, mimeType string) error {
	jid, err := s.parsePhoneNumber(number)
	if err != nil {
		return err
	}

	mediaData, contentType, filename, err := s.prepareMediaData(mediaURL, mediaBase64, mimeType)
	if err != nil {
		return err
	}

	mediaType := s.determineMediaType(contentType)

	s.logger.Infof("Fazendo upload da mídia: tipo=%s, tamanho=%d bytes", contentType, len(mediaData))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	uploaded, err := s.client.Upload(ctx, mediaData, mediaType)
	if err != nil {
		return fmt.Errorf("falha ao fazer upload da mídia: %w", err)
	}

	s.logger.Infof("Mídia enviada com sucesso: URL=%s", uploaded.URL)

	message := s.buildMediaMessage(uploaded, mediaData, contentType, caption, filename)

	_, err = s.client.SendMessage(ctx, jid, message)
	if err != nil {
		return fmt.Errorf("falha ao enviar mensagem de mídia: %w", err)
	}

	s.logger.Infof("Mensagem de mídia enviada para %s", number)
	return nil
}

func (s *WhatsAppService) parsePhoneNumber(number string) (types.JID, error) {
	number = strings.TrimSpace(number)
	number = strings.ReplaceAll(number, " ", "")
	number = strings.ReplaceAll(number, "-", "")
	number = strings.ReplaceAll(number, "(", "")
	number = strings.ReplaceAll(number, ")", "")

	if !strings.HasSuffix(number, "@s.whatsapp.net") {
		if !strings.HasPrefix(number, s.config.WhatsApp.DefaultCountry) {
			number = s.config.WhatsApp.DefaultCountry + number
		}
		number = number + "@s.whatsapp.net"
	}

	jid, err := types.ParseJID(number)
	if err != nil {
		return types.JID{}, fmt.Errorf("número de telefone inválido: %w", err)
	}

	return jid, nil
}

func (s *WhatsAppService) prepareMediaData(mediaURL, mediaBase64, mimeType string) ([]byte, string, string, error) {
	var mediaData []byte
	var contentType string
	var filename string
	var err error

	if mediaBase64 != "" {
		mediaData, contentType, err = s.decodeBase64Media(mediaBase64, mimeType)
		if err != nil {
			return nil, "", "", err
		}

		exts, _ := mime.ExtensionsByType(contentType)
		ext := ".bin"
		if len(exts) > 0 {
			ext = exts[0]
		}
		filename = "media" + ext

	} else if mediaURL != "" {
		mediaData, contentType, err = s.downloadMedia(mediaURL)
		if err != nil {
			return nil, "", "", err
		}

		ext := filepath.Ext(mediaURL)
		if ext == "" {
			exts, _ := mime.ExtensionsByType(contentType)
			if len(exts) > 0 {
				ext = exts[0]
			}
		}
		filename = "media" + ext

	} else {
		return nil, "", "", fmt.Errorf("é necessário fornecer media_url ou media_base64")
	}

	return mediaData, contentType, filename, nil
}

func (s *WhatsAppService) decodeBase64Media(base64Str, mimeType string) ([]byte, string, error) {
	base64Str = strings.TrimSpace(base64Str)

	if strings.HasPrefix(base64Str, "data:") {
		if idx := strings.Index(base64Str, ","); idx != -1 {
			base64Str = base64Str[idx+1:]
		}
	}

	base64Str = strings.ReplaceAll(base64Str, "\n", "")
	base64Str = strings.ReplaceAll(base64Str, "\r", "")
	base64Str = strings.TrimSpace(base64Str)

	mediaData, err := base64.StdEncoding.DecodeString(base64Str)
	if err != nil {
		return nil, "", fmt.Errorf("falha ao decodificar base64: %w", err)
	}

	contentType := mimeType
	if contentType == "" {
		contentType = http.DetectContentType(mediaData)
	}

	return mediaData, contentType, nil
}

func (s *WhatsAppService) downloadMedia(url string) ([]byte, string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, "", fmt.Errorf("falha ao criar requisição: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("falha ao baixar mídia: %w", err)
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			s.logger.Errorf("falha ao fechar corpo da resposta: %v", err)
		}
	}(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("falha ao baixar mídia: status %d", resp.StatusCode)
	}

	mediaData, err := io.ReadAll(io.LimitReader(resp.Body, s.config.Server.MaxUploadSize))
	if err != nil {
		return nil, "", fmt.Errorf("falha ao ler mídia: %w", err)
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = http.DetectContentType(mediaData)
	}

	return mediaData, contentType, nil
}

func (s *WhatsAppService) determineMediaType(contentType string) whatsmeow.MediaType {
	switch {
	case strings.HasPrefix(contentType, "image/"):
		return whatsmeow.MediaImage
	case strings.HasPrefix(contentType, "video/"):
		return whatsmeow.MediaVideo
	case strings.HasPrefix(contentType, "audio/"):
		return whatsmeow.MediaAudio
	default:
		return whatsmeow.MediaDocument
	}
}

func (s *WhatsAppService) buildMediaMessage(uploaded whatsmeow.UploadResponse, mediaData []byte, contentType, caption, filename string) *waE2E.Message {
	fileLength := proto.Uint64(uint64(len(mediaData)))
	mimetype := proto.String(contentType)

	switch {
	case strings.HasPrefix(contentType, "image/"):
		return &waE2E.Message{
			ImageMessage: &waE2E.ImageMessage{
				Caption:       proto.String(caption),
				URL:           proto.String(uploaded.URL),
				DirectPath:    proto.String(uploaded.DirectPath),
				MediaKey:      uploaded.MediaKey,
				Mimetype:      mimetype,
				FileEncSHA256: uploaded.FileEncSHA256,
				FileSHA256:    uploaded.FileSHA256,
				FileLength:    fileLength,
			},
		}
	case strings.HasPrefix(contentType, "video/"):
		return &waE2E.Message{
			VideoMessage: &waE2E.VideoMessage{
				Caption:       proto.String(caption),
				URL:           proto.String(uploaded.URL),
				DirectPath:    proto.String(uploaded.DirectPath),
				MediaKey:      uploaded.MediaKey,
				Mimetype:      mimetype,
				FileEncSHA256: uploaded.FileEncSHA256,
				FileSHA256:    uploaded.FileSHA256,
				FileLength:    fileLength,
			},
		}
	case strings.HasPrefix(contentType, "audio/"):
		return &waE2E.Message{
			AudioMessage: &waE2E.AudioMessage{
				URL:           proto.String(uploaded.URL),
				DirectPath:    proto.String(uploaded.DirectPath),
				MediaKey:      uploaded.MediaKey,
				Mimetype:      mimetype,
				FileEncSHA256: uploaded.FileEncSHA256,
				FileSHA256:    uploaded.FileSHA256,
				FileLength:    fileLength,
			},
		}
	default:
		return &waE2E.Message{
			DocumentMessage: &waE2E.DocumentMessage{
				Caption:       proto.String(caption),
				URL:           proto.String(uploaded.URL),
				DirectPath:    proto.String(uploaded.DirectPath),
				MediaKey:      uploaded.MediaKey,
				Mimetype:      mimetype,
				FileEncSHA256: uploaded.FileEncSHA256,
				FileSHA256:    uploaded.FileSHA256,
				FileLength:    fileLength,
				FileName:      proto.String(filename),
			},
		}
	}
}
