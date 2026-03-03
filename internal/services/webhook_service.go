package services

import (
	"boot-whatsapp-golang/internal/models"
	"boot-whatsapp-golang/internal/repository"
	"boot-whatsapp-golang/pkg/logger"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
)

type WebhookService struct {
	repository *repository.WebhookRepository
	logger     *logger.Logger
	httpClient *http.Client
}

func NewWebhookService(repo *repository.WebhookRepository, log *logger.Logger) *WebhookService {
	return &WebhookService{
		repository: repo,
		logger:     log,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (s *WebhookService) RegisterWebhook(sessionID uuid.UUID, tenantID string, req *models.RegisterWebhookRequest) (*models.Webhook, error) {
	if len(req.Events) == 0 {
		return nil, fmt.Errorf("eventos não podem estar vazios")
	}

	webhook := &models.Webhook{
		ID:        uuid.New(),
		SessionID: sessionID,
		TenantID:  tenantID,
		URL:       req.URL,
		Events:    req.Events,
		Headers:   req.Headers,
		Active:    true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := s.repository.Create(webhook); err != nil {
		s.logger.Errorf("Falha ao registrar webhook: %v", err)
		return nil, err
	}

	s.logger.Infof("Webhook registrado com sucesso: %s para sessão %s", webhook.URL, sessionID)
	return webhook, nil
}

func (s *WebhookService) GetWebhooks(sessionID uuid.UUID, tenantID string) ([]*models.Webhook, error) {
	webhooks, err := s.repository.GetBySessionIDAndTenant(sessionID, tenantID)
	if err != nil {
		s.logger.Errorf("Falha ao buscar webhooks: %v", err)
		return nil, err
	}
	return webhooks, nil
}

func (s *WebhookService) GetWebhookByID(id uuid.UUID) (*models.Webhook, error) {
	webhook, err := s.repository.GetByID(id)
	if err != nil {
		s.logger.Errorf("Falha ao buscar webhook %s: %v", id, err)
		return nil, err
	}
	return webhook, nil
}

func (s *WebhookService) UpdateWebhook(id uuid.UUID, req *models.RegisterWebhookRequest) (*models.Webhook, error) {
	webhook, err := s.repository.GetByID(id)
	if err != nil {
		return nil, err
	}

	webhook.URL = req.URL
	webhook.Events = req.Events
	webhook.Headers = req.Headers
	webhook.UpdatedAt = time.Now()

	if err := s.repository.Update(webhook); err != nil {
		s.logger.Errorf("Falha ao atualizar webhook %s: %v", id, err)
		return nil, err
	}

	s.logger.Infof("Webhook atualizado: %s", id)
	return webhook, nil
}

func (s *WebhookService) DeleteWebhook(id uuid.UUID) error {
	if err := s.repository.Delete(id); err != nil {
		s.logger.Errorf("Falha ao deletar webhook %s: %v", id, err)
		return err
	}
	s.logger.Infof("Webhook deletado: %s", id)
	return nil
}

func (s *WebhookService) TriggerWebhooks(sessionID uuid.UUID, tenantID string, eventType string, data *models.MessageWebhookData, sessionKey string) {
	webhooks, err := s.repository.GetBySessionIDAndTenant(sessionID, tenantID)
	if err != nil {
		s.logger.Errorf("Falha ao buscar webhooks para trigger: %v", err)
		return
	}

	for _, webhook := range webhooks {
		if !webhook.Active {
			continue
		}

		if !s.eventAppliesTo(eventType, webhook.Events) {
			continue
		}

		go s.execWebhook(webhook, eventType, data, sessionKey)
	}
}

func (s *WebhookService) eventAppliesTo(eventType string, webhookEvents []string) bool {
	for _, we := range webhookEvents {
		if we == eventType || we == "*" {
			return true
		}
	}
	return false
}

func (s *WebhookService) execWebhook(webhook *models.Webhook, eventType string, data *models.MessageWebhookData, sessionKey string) {
	startTime := time.Now()

	payload := &models.WebhookPayload{
		Event:      eventType,
		SessionKey: sessionKey,
		TenantID:   webhook.TenantID,
		Timestamp:  time.Now(),
		Data:       data,
	}

	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		s.logWebhookExecution(webhook, eventType, payloadJSON, nil, fmt.Sprintf("Erro ao serializar payload: %v", err), startTime)
		return
	}

	req, err := http.NewRequest("POST", webhook.URL, bytes.NewBuffer(payloadJSON))
	if err != nil {
		s.logWebhookExecution(webhook, eventType, payloadJSON, nil, fmt.Sprintf("Erro ao criar request: %v", err), startTime)
		return
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "WhatsApp-Bot-API/2.0")
	req.Header.Set("X-Webhook-Event", eventType)
	req.Header.Set("X-Webhook-ID", webhook.ID.String())

	for key, value := range webhook.Headers {
		req.Header.Set(key, value)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		s.logWebhookExecution(webhook, eventType, payloadJSON, nil, fmt.Sprintf("Erro ao executar webhook: %v", err), startTime)
		return
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			s.logger.Errorf("Erro ao fechar response body do webhook %s: %v", webhook.ID, err)
		}
	}(resp.Body)

	var respBody []byte
	if resp.Body != nil {
		respBody, _ = io.ReadAll(resp.Body)
	}

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		s.logger.Infof("Webhook disparado com sucesso [%s]: %s (Status: %d)", eventType, webhook.URL, resp.StatusCode)
		s.logWebhookExecution(webhook, eventType, payloadJSON, new(resp.StatusCode), string(respBody), startTime)
		err := s.repository.UpdateLastTriggered(webhook.ID)
		if err != nil {
			s.logger.Errorf("Falha ao atualizar last_triggered do webhook %s: %v", webhook.ID, err)
		}
	} else {
		s.logger.Warnf("Webhook retornou status %d [%s]: %s", resp.StatusCode, eventType, webhook.URL)
		s.logWebhookExecution(webhook, eventType, payloadJSON, new(resp.StatusCode), string(respBody), startTime)
	}
}

func (s *WebhookService) logWebhookExecution(webhook *models.Webhook, eventType string, payloadJSON []byte, httpStatus *int, message string, startTime time.Time) {
	var errorMsg *string
	if message != "" {
		errorMsg = &message
	}

	log := &models.WebhookLog{
		ID:              uuid.New(),
		WebhookID:       webhook.ID,
		EventType:       eventType,
		Payload:         string(payloadJSON),
		HTTPStatus:      httpStatus,
		ResponseBody:    nil,
		ErrorMessage:    errorMsg,
		ExecutionTimeMS: new(int(time.Since(startTime).Milliseconds())),
		TriggeredAt:     time.Now(),
	}

	if message != "" && httpStatus == nil {
		log.ResponseBody = &message
	} else if message != "" {
		log.ResponseBody = &message
	}

	if err := s.repository.LogExecution(log); err != nil {
		s.logger.Errorf("Falha ao registrar log do webhook: %v", err)
	}
}

func (s *WebhookService) GetWebhookLogs(webhookID uuid.UUID, limit int) ([]*models.WebhookLog, error) {
	logs, err := s.repository.GetLogs(webhookID, limit)
	if err != nil {
		s.logger.Errorf("Falha ao buscar logs do webhook: %v", err)
		return nil, err
	}
	return logs, nil
}
