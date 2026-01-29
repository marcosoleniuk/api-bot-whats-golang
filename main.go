package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/mdp/qrterminal/v3"
	_ "github.com/mdp/qrterminal/v3"
	"go.mau.fi/whatsmeow"
	waProto "go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	waLog "go.mau.fi/whatsmeow/util/log"
	"google.golang.org/protobuf/proto"
)

const (
	API_TOKEN       = "eMFCeuCLQYW02hTp9q6QecSP7RnTFdWb"
	SESSION_KEY     = "moleniuk"
	API_ENDPOINT    = "/sendText"
	MEDIA_ENDPOINT  = "/sendMedia"
	HTTP_PORT       = "8080"
	MAX_UPLOAD_SIZE = 50 << 20
)

type MessageRequest struct {
	Number string `json:"number"`
	Text   string `json:"text"`
}

type MediaRequest struct {
	Number      string `json:"number"`
	Caption     string `json:"caption"`
	MediaURL    string `json:"media_url"`
	MediaBase64 string `json:"media_base64"`
	MimeType    string `json:"mime_type"`
}

type APIResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

type customLogger struct{ prefix string }

func (l *customLogger) Sub(module string) waLog.Logger {
	return &customLogger{prefix: l.prefix + "[" + module + "] "}
}
func (l *customLogger) Warnf(format string, args ...any) {
	log.Printf(l.prefix+"[WARN] "+format, args...)
}
func (l *customLogger) Errorf(format string, args ...any) {
	log.Printf(l.prefix+"[ERROR] "+format, args...)
}
func (l *customLogger) Infof(format string, args ...any) {
	log.Printf(l.prefix+"[INFO] "+format, args...)
}
func (l *customLogger) Debugf(format string, args ...any) {
	log.Printf(l.prefix+"[DEBUG] "+format, args...)
}

type WhatsAppClient struct {
	client *whatsmeow.Client
	logger waLog.Logger
}

func NewWhatsAppClient() (*WhatsAppClient, error) {
	waLogger := &customLogger{prefix: "[BOT] "}
	ctx := context.Background()
	container, err := sqlstore.New(ctx, "sqlite3", "file:bot.db?_foreign_keys=on", waLogger)
	if err != nil {
		return nil, fmt.Errorf("erro ao iniciar banco: %v", err)
	}

	deviceStore, err := container.GetFirstDevice(ctx)
	if err != nil {
		return nil, fmt.Errorf("erro ao obter dispositivo: %v", err)
	}

	client := whatsmeow.NewClient(deviceStore, waLogger)
	return &WhatsAppClient{client: client, logger: waLogger}, nil
}

func (wc *WhatsAppClient) Connect() error {
	if wc.client.IsConnected() {
		return nil
	}

	if wc.client.Store.ID == nil {
		qrChan, _ := wc.client.GetQRChannel(context.Background())
		if err := wc.client.Connect(); err != nil {
			return fmt.Errorf("erro ao conectar: %v", err)
		}
		for evt := range qrChan {
			if evt.Event == "code" {
				wc.logger.Infof("Escaneie o QR Code exibido no terminal")
				qrterminal.GenerateHalfBlock(evt.Code, qrterminal.L, os.Stdout)
			} else {
				wc.logger.Infof("Evento de login: %s", evt.Event)
			}
		}
	} else {
		if err := wc.client.Connect(); err != nil {
			return fmt.Errorf("erro ao conectar com sessão existente: %v", err)
		}
	}
	return nil
}

func (wc *WhatsAppClient) SendMessage(number, text string) error {
	if !strings.HasSuffix(number, "@s.whatsapp.net") {
		number = "55" + number + "@s.whatsapp.net"
	}

	jid, err := types.ParseJID(number)
	if err != nil {
		return fmt.Errorf("número inválido: %v", err)
	}

	_, err = wc.client.SendMessage(context.Background(), jid, &waProto.Message{
		ExtendedTextMessage: &waProto.ExtendedTextMessage{
			Text: &text,
		},
	})
	if err != nil {
		return fmt.Errorf("falha ao enviar mensagem: %v", err)
	}

	return nil
}

func (wc *WhatsAppClient) SendMediaMessage(number, caption, mediaURL, mediaBase64, mimeType string) error {
	if !strings.HasSuffix(number, "@s.whatsapp.net") {
		number = "55" + number + "@s.whatsapp.net"
	}

	jid, err := types.ParseJID(number)
	if err != nil {
		return fmt.Errorf("número inválido: %v", err)
	}

	var mediaData []byte
	var contentType string
	var filename string

	if mediaBase64 != "" {
		s := strings.TrimSpace(mediaBase64)
		if strings.HasPrefix(s, "data:") {
			if idx := strings.Index(s, ","); idx != -1 {
				s = s[idx+1:]
			}
		}
		s = strings.ReplaceAll(s, "\n", "")
		s = strings.ReplaceAll(s, "\r", "")
		s = strings.TrimSpace(s)

		mediaData, err = base64.StdEncoding.DecodeString(s)
		if err != nil {
			return fmt.Errorf("erro ao decodificar base64: %v", err)
		}

		if mimeType != "" {
			contentType = mimeType
		} else {
			contentType = http.DetectContentType(mediaData)
		}

		exts, _ := mime.ExtensionsByType(contentType)
		ext := ".bin"
		if len(exts) > 0 {
			ext = exts[0]
		}
		filename = "media" + ext

	} else if mediaURL != "" {
		resp, err := http.Get(mediaURL)
		if err != nil {
			return fmt.Errorf("erro ao baixar mídia: %v", err)
		}
		defer func(Body io.ReadCloser) {
			err := Body.Close()
			if err != nil {
				wc.logger.Errorf("Erro ao fechar corpo da resposta: %v", err)
			}
		}(resp.Body)

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("falha ao baixar mídia: status %d", resp.StatusCode)
		}

		mediaData, err = io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("erro ao ler mídia: %v", err)
		}

		contentType = resp.Header.Get("Content-Type")
		if contentType == "" {
			contentType = http.DetectContentType(mediaData)
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
		return fmt.Errorf("é necessário fornecer media_url ou media_base64")
	}

	var mediaType whatsmeow.MediaType
	if strings.HasPrefix(contentType, "image/") {
		mediaType = whatsmeow.MediaImage
	} else if strings.HasPrefix(contentType, "video/") {
		mediaType = whatsmeow.MediaVideo
	} else if strings.HasPrefix(contentType, "audio/") {
		mediaType = whatsmeow.MediaAudio
	} else {
		mediaType = whatsmeow.MediaDocument
	}

	wc.logger.Infof("Fazendo upload da mídia: contentType=%s, size=%d, mediaType=%v", contentType, len(mediaData), mediaType)

	uploaded, err := wc.client.Upload(context.Background(), mediaData, mediaType)
	if err != nil {
		wc.logger.Errorf("Upload falhou: contentType=%s, size=%d, mediaType=%v, erro=%v", contentType, len(mediaData), mediaType, err)
		return fmt.Errorf("erro ao fazer upload da mídia: %v", err)
	}

	wc.logger.Infof("Upload completo: URL=%s DirectPath=%s Size=%d", uploaded.URL, uploaded.DirectPath, len(mediaData))

	mimetype := contentType

	var msg *waProto.Message

	if strings.HasPrefix(contentType, "image/") {
		msg = &waProto.Message{
			ImageMessage: &waProto.ImageMessage{
				Caption:       proto.String(caption),
				URL:           proto.String(uploaded.URL),
				DirectPath:    proto.String(uploaded.DirectPath),
				MediaKey:      uploaded.MediaKey,
				Mimetype:      proto.String(mimetype),
				FileEncSHA256: uploaded.FileEncSHA256,
				FileSHA256:    uploaded.FileSHA256,
				FileLength:    proto.Uint64(uint64(len(mediaData))),
			},
		}
	} else if strings.HasPrefix(contentType, "video/") {
		msg = &waProto.Message{
			VideoMessage: &waProto.VideoMessage{
				Caption:       proto.String(caption),
				URL:           proto.String(uploaded.URL),
				DirectPath:    proto.String(uploaded.DirectPath),
				MediaKey:      uploaded.MediaKey,
				Mimetype:      proto.String(mimetype),
				FileEncSHA256: uploaded.FileEncSHA256,
				FileSHA256:    uploaded.FileSHA256,
				FileLength:    proto.Uint64(uint64(len(mediaData))),
			},
		}
	} else if strings.HasPrefix(contentType, "audio/") {
		msg = &waProto.Message{
			AudioMessage: &waProto.AudioMessage{
				URL:           proto.String(uploaded.URL),
				DirectPath:    proto.String(uploaded.DirectPath),
				MediaKey:      uploaded.MediaKey,
				Mimetype:      proto.String(mimetype),
				FileEncSHA256: uploaded.FileEncSHA256,
				FileSHA256:    uploaded.FileSHA256,
				FileLength:    proto.Uint64(uint64(len(mediaData))),
			},
		}
	} else {
		msg = &waProto.Message{
			DocumentMessage: &waProto.DocumentMessage{
				Caption:       proto.String(caption),
				URL:           proto.String(uploaded.URL),
				DirectPath:    proto.String(uploaded.DirectPath),
				MediaKey:      uploaded.MediaKey,
				Mimetype:      proto.String(mimetype),
				FileEncSHA256: uploaded.FileEncSHA256,
				FileSHA256:    uploaded.FileSHA256,
				FileLength:    proto.Uint64(uint64(len(mediaData))),
				FileName:      proto.String(filename),
			},
		}
	}

	_, err = wc.client.SendMessage(context.Background(), jid, msg)
	if err != nil {
		return fmt.Errorf("falha ao enviar mensagem de mídia: %v", err)
	}

	return nil
}

func (wc *WhatsAppClient) handleSendText(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		wc.logger.Errorf("Método não permitido: %s", r.Method)
		http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
		return
	}

	if r.Header.Get("apitoken") != API_TOKEN || r.Header.Get("SESSIONKEY") != SESSION_KEY {
		wc.logger.Errorf("Autenticação inválida para IP: %s", r.RemoteAddr)
		http.Error(w, "Autenticação inválida", http.StatusUnauthorized)
		return
	}

	var req MessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		wc.logger.Errorf("JSON inválido: %v", err)
		http.Error(w, "JSON inválido", http.StatusBadRequest)
		return
	}

	if req.Number == "" || req.Text == "" {
		wc.logger.Errorf("Campos 'number' ou 'text' ausentes")
		http.Error(w, "Campos 'number' e 'text' são obrigatórios", http.StatusBadRequest)
		return
	}

	err := wc.SendMessage(req.Number, req.Text)
	if err != nil {
		wc.logger.Errorf("Erro ao enviar mensagem para %s: %v", req.Number, err)
		http.Error(w, fmt.Sprintf("Erro ao enviar mensagem: %v", err), http.StatusInternalServerError)
		return
	}

	resp := APIResponse{Status: "success", Message: "Mensagem enviada com sucesso"}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		wc.logger.Errorf("Erro ao codificar resposta JSON: %v", err)
	}
	wc.logger.Infof("Mensagem enviada para %s com sucesso", req.Number)
}

func (wc *WhatsAppClient) handleSendMedia(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		wc.logger.Errorf("Método não permitido: %s", r.Method)
		http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
		return
	}

	if r.Header.Get("apitoken") != API_TOKEN || r.Header.Get("SESSIONKEY") != SESSION_KEY {
		wc.logger.Errorf("Autenticação inválida para IP: %s", r.RemoteAddr)
		http.Error(w, "Autenticação inválida", http.StatusUnauthorized)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, MAX_UPLOAD_SIZE)

	var req MediaRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		wc.logger.Errorf("JSON inválido: %v", err)
		http.Error(w, "JSON inválido", http.StatusBadRequest)
		return
	}

	if req.Number == "" {
		wc.logger.Errorf("Campo 'number' ausente")
		http.Error(w, "Campo 'number' é obrigatório", http.StatusBadRequest)
		return
	}

	if req.MediaURL == "" && req.MediaBase64 == "" {
		wc.logger.Errorf("É necessário fornecer 'media_url' ou 'media_base64'")
		http.Error(w, "É necessário fornecer 'media_url' ou 'media_base64'", http.StatusBadRequest)
		return
	}

	if req.MediaBase64 != "" && req.MimeType == "" {
		wc.logger.Errorf("Campo 'mime_type' é obrigatório quando usar 'media_base64'")
		http.Error(w, "Campo 'mime_type' é obrigatório quando usar 'media_base64'", http.StatusBadRequest)
		return
	}

	err := wc.SendMediaMessage(req.Number, req.Caption, req.MediaURL, req.MediaBase64, req.MimeType)
	if err != nil {
		wc.logger.Errorf("Erro ao enviar mídia para %s: %v", req.Number, err)
		http.Error(w, fmt.Sprintf("Erro ao enviar mídia: %v", err), http.StatusInternalServerError)
		return
	}

	resp := APIResponse{Status: "success", Message: "Mídia enviada com sucesso"}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		wc.logger.Errorf("Erro ao codificar resposta JSON: %v", err)
	}
	wc.logger.Infof("Mídia enviada para %s com sucesso", req.Number)
}

func main() {
	wc, err := NewWhatsAppClient()
	if err != nil {
		log.Fatalf("❌ Erro ao inicializar cliente WhatsApp: %v", err)
	}

	if err := wc.Connect(); err != nil {
		log.Fatalf("❌ Erro ao conectar ao WhatsApp: %v", err)
	}

	http.HandleFunc(API_ENDPOINT, wc.handleSendText)
	http.HandleFunc(MEDIA_ENDPOINT, wc.handleSendMedia)
	server := &http.Server{
		Addr:         ":" + HTTP_PORT,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	csig := make(chan os.Signal, 1)
	signal.Notify(csig, os.Interrupt)

	go func() {
		wc.logger.Infof("🤖 Servidor iniciado na porta %s", HTTP_PORT)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("❌ Erro ao iniciar servidor: %v", err)
		}
	}()

	<-csig
	wc.logger.Infof("Encerrando servidor...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		wc.logger.Errorf("Erro ao encerrar servidor: %v", err)
	}
	wc.client.Disconnect()
	wc.logger.Infof("Servidor encerrado com sucesso")
}
