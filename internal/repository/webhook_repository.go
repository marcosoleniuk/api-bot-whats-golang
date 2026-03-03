package repository

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"boot-whatsapp-golang/internal/models"
	"boot-whatsapp-golang/pkg/logger"
	"github.com/google/uuid"
	"github.com/lib/pq"
)

type WebhookRepository struct {
	db     *sql.DB
	logger *logger.Logger
}

func NewWebhookRepository(db *sql.DB, log *logger.Logger) *WebhookRepository {
	return &WebhookRepository{db: db, logger: log}
}

const webhookSelectCols = `
	id, session_id, tenant_id, url, events, headers, active, created_at, updated_at, last_triggered_at
`

const webhookSelectBase = `
	SELECT ` + webhookSelectCols + `
	FROM webhooks
`

func scanWebhook(scanner interface{ Scan(dest ...any) error }) (*models.Webhook, error) {
	var (
		id              uuid.UUID
		sessionID       uuid.UUID
		tenantID        string
		url             string
		events          pq.StringArray
		headersJSON     sql.NullString
		active          bool
		createdAt       time.Time
		updatedAt       time.Time
		lastTriggeredAt sql.NullTime
	)

	if err := scanner.Scan(
		&id,
		&sessionID,
		&tenantID,
		&url,
		&events,
		&headersJSON,
		&active,
		&createdAt,
		&updatedAt,
		&lastTriggeredAt,
	); err != nil {
		return nil, err
	}

	headers := make(map[string]string)
	if headersJSON.Valid {
		if err := json.Unmarshal([]byte(headersJSON.String), &headers); err != nil {
			return nil, fmt.Errorf("erro ao desserializar headers: %w", err)
		}
	}

	webhook := &models.Webhook{
		ID:        id,
		SessionID: sessionID,
		TenantID:  tenantID,
		URL:       url,
		Events:    []string(events),
		Headers:   headers,
		Active:    active,
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
	}

	if lastTriggeredAt.Valid {
		webhook.LastTriggeredAt = &lastTriggeredAt.Time
	}

	return webhook, nil
}

func (r *WebhookRepository) Create(webhook *models.Webhook) error {
	headersJSON, err := json.Marshal(webhook.Headers)
	if err != nil {
		return fmt.Errorf("erro ao serializar headers: %w", err)
	}

	query := `
		INSERT INTO webhooks (
			id, session_id, tenant_id, url, events, headers, active, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id
	`

	_, err = r.db.Exec(query,
		webhook.ID,
		webhook.SessionID,
		webhook.TenantID,
		webhook.URL,
		pq.Array(webhook.Events),
		string(headersJSON),
		webhook.Active,
		webhook.CreatedAt,
		webhook.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("falha ao criar webhook: %w", err)
	}

	return nil
}

func (r *WebhookRepository) GetByID(id uuid.UUID) (*models.Webhook, error) {
	query := webhookSelectBase + " WHERE id = $1"
	row := r.db.QueryRow(query, id)
	webhook, err := scanWebhook(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("webhook não encontrado")
		}
		return nil, fmt.Errorf("falha ao buscar webhook: %w", err)
	}
	return webhook, nil
}

func (r *WebhookRepository) GetBySessionID(sessionID uuid.UUID) ([]*models.Webhook, error) {
	query := webhookSelectBase + " WHERE session_id = $1 AND active = true ORDER BY created_at DESC"
	rows, err := r.db.Query(query, sessionID)
	if err != nil {
		return nil, fmt.Errorf("falha ao buscar webhooks da sessão: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			r.logger.Errorf("Erro ao fechar linhas de webhook: %v", err)
		}
	}()

	var webhooks []*models.Webhook
	for rows.Next() {
		webhook, err := scanWebhook(rows)
		if err != nil {
			return nil, fmt.Errorf("falha ao processar webhook: %w", err)
		}
		webhooks = append(webhooks, webhook)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("erro ao iterar webhooks: %w", err)
	}

	return webhooks, nil
}

func (r *WebhookRepository) GetBySessionIDAndTenant(sessionID uuid.UUID, tenantID string) ([]*models.Webhook, error) {
	query := webhookSelectBase + " WHERE session_id = $1 AND tenant_id = $2 ORDER BY created_at DESC"
	rows, err := r.db.Query(query, sessionID, tenantID)
	if err != nil {
		return nil, fmt.Errorf("falha ao buscar webhooks: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			r.logger.Errorf("Erro ao fechar linhas de webhook: %v", err)
		}
	}()

	var webhooks []*models.Webhook
	for rows.Next() {
		webhook, err := scanWebhook(rows)
		if err != nil {
			return nil, fmt.Errorf("falha ao processar webhook: %w", err)
		}
		webhooks = append(webhooks, webhook)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("erro ao iterar webhooks: %w", err)
	}

	return webhooks, nil
}

func (r *WebhookRepository) Update(webhook *models.Webhook) error {
	headersJSON, err := json.Marshal(webhook.Headers)
	if err != nil {
		return fmt.Errorf("erro ao serializar headers: %w", err)
	}

	query := `
		UPDATE webhooks
		SET events = $2, headers = $3, active = $4, updated_at = $5
		WHERE id = $1
	`

	result, err := r.db.Exec(query,
		webhook.ID,
		pq.Array(webhook.Events),
		string(headersJSON),
		webhook.Active,
		time.Now(),
	)

	if err != nil {
		return fmt.Errorf("falha ao atualizar webhook: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("erro ao obter linhas afetadas: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("webhook não encontrado")
	}

	return nil
}

func (r *WebhookRepository) Delete(id uuid.UUID) error {
	query := "DELETE FROM webhooks WHERE id = $1"
	result, err := r.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("falha ao deletar webhook: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("erro ao obter linhas afetadas: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("webhook não encontrado")
	}

	return nil
}

func (r *WebhookRepository) UpdateLastTriggered(id uuid.UUID) error {
	query := "UPDATE webhooks SET last_triggered_at = $1 WHERE id = $2"
	_, err := r.db.Exec(query, time.Now(), id)
	if err != nil {
		return fmt.Errorf("falha ao atualizar last_triggered_at: %w", err)
	}
	return nil
}

func (r *WebhookRepository) LogExecution(log *models.WebhookLog) error {
	query := `
		INSERT INTO webhook_logs (
			id, webhook_id, event_type, payload, http_status, response_body, error_message, execution_time_ms, triggered_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`

	_, err := r.db.Exec(query,
		log.ID,
		log.WebhookID,
		log.EventType,
		log.Payload,
		log.HTTPStatus,
		log.ResponseBody,
		log.ErrorMessage,
		log.ExecutionTimeMS,
		log.TriggeredAt,
	)

	if err != nil {
		return fmt.Errorf("falha ao registrar log de webhook: %w", err)
	}

	return nil
}

func (r *WebhookRepository) GetLogs(webhookID uuid.UUID, limit int) ([]*models.WebhookLog, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 500 {
		limit = 500
	}

	query := `
		SELECT id, webhook_id, event_type, payload, http_status, response_body, error_message, execution_time_ms, triggered_at
		FROM webhook_logs
		WHERE webhook_id = $1
		ORDER BY triggered_at DESC
		LIMIT $2
	`

	rows, err := r.db.Query(query, webhookID, limit)
	if err != nil {
		return nil, fmt.Errorf("falha ao buscar logs: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			r.logger.Errorf("Erro ao fechar linhas de log: %v", err)
		}
	}()

	var logs []*models.WebhookLog
	for rows.Next() {
		log := &models.WebhookLog{}
		if err := rows.Scan(
			&log.ID,
			&log.WebhookID,
			&log.EventType,
			&log.Payload,
			&log.HTTPStatus,
			&log.ResponseBody,
			&log.ErrorMessage,
			&log.ExecutionTimeMS,
			&log.TriggeredAt,
		); err != nil {
			return nil, fmt.Errorf("falha ao processar log: %w", err)
		}
		logs = append(logs, log)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("erro ao iterar logs: %w", err)
	}

	return logs, nil
}
