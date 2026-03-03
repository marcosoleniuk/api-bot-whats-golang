package repository

import (
	"boot-whatsapp-golang/internal/models"
	"boot-whatsapp-golang/pkg/logger"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

type PollMetadataRepository struct {
	db  *sql.DB
	log *logger.Logger
}

func NewPollMetadataRepository(db *sql.DB, log *logger.Logger) *PollMetadataRepository {
	return &PollMetadataRepository{
		db:  db,
		log: log,
	}
}

func (r *PollMetadataRepository) Upsert(meta *models.PollMetadata) error {
	if meta == nil {
		return fmt.Errorf("metadados de enquete inválidos")
	}

	meta.ChatJID = strings.TrimSpace(meta.ChatJID)
	meta.PollMessageID = strings.TrimSpace(meta.PollMessageID)
	meta.Question = strings.TrimSpace(meta.Question)
	if meta.ChatJID == "" || meta.PollMessageID == "" || meta.SessionID == uuid.Nil {
		return fmt.Errorf("chat_jid, poll_message_id e session_id são obrigatórios")
	}

	if meta.ID == uuid.Nil {
		meta.ID = uuid.New()
	}
	now := time.Now()
	if meta.CreatedAt.IsZero() {
		meta.CreatedAt = now
	}
	meta.UpdatedAt = now

	optionsJSON, err := json.Marshal(meta.Options)
	if err != nil {
		return fmt.Errorf("falha ao serializar opções da enquete: %w", err)
	}

	query := `
		INSERT INTO poll_metadata (
			id, session_id, tenant_id, chat_jid, poll_message_id, question, options, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7::jsonb, $8, $9
		)
		ON CONFLICT (session_id, chat_jid, poll_message_id)
		DO UPDATE SET
			question = EXCLUDED.question,
			options = EXCLUDED.options,
			updated_at = EXCLUDED.updated_at
	`

	if _, err := r.db.Exec(
		query,
		meta.ID,
		meta.SessionID,
		meta.TenantID,
		meta.ChatJID,
		meta.PollMessageID,
		meta.Question,
		string(optionsJSON),
		meta.CreatedAt,
		meta.UpdatedAt,
	); err != nil {
		return fmt.Errorf("falha ao salvar metadados de enquete: %w", err)
	}

	return nil
}

func (r *PollMetadataRepository) GetBySessionChatAndPoll(sessionID uuid.UUID, chatJID, pollMessageID string) (*models.PollMetadata, error) {
	query := `
		SELECT id, session_id, tenant_id, chat_jid, poll_message_id, question, options, created_at, updated_at
		FROM poll_metadata
		WHERE session_id = $1 AND chat_jid = $2 AND poll_message_id = $3
		ORDER BY updated_at DESC
		LIMIT 1
	`
	return r.getOne(query, sessionID, strings.TrimSpace(chatJID), strings.TrimSpace(pollMessageID))
}

func (r *PollMetadataRepository) GetBySessionAndPoll(sessionID uuid.UUID, pollMessageID string) (*models.PollMetadata, error) {
	query := `
		SELECT id, session_id, tenant_id, chat_jid, poll_message_id, question, options, created_at, updated_at
		FROM poll_metadata
		WHERE session_id = $1 AND poll_message_id = $2
		ORDER BY updated_at DESC
		LIMIT 1
	`
	return r.getOne(query, sessionID, strings.TrimSpace(pollMessageID))
}

func (r *PollMetadataRepository) getOne(query string, args ...interface{}) (*models.PollMetadata, error) {
	meta := &models.PollMetadata{}
	var optionsRaw []byte
	err := r.db.QueryRow(query, args...).Scan(
		&meta.ID,
		&meta.SessionID,
		&meta.TenantID,
		&meta.ChatJID,
		&meta.PollMessageID,
		&meta.Question,
		&optionsRaw,
		&meta.CreatedAt,
		&meta.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("falha ao buscar metadados de enquete: %w", err)
	}

	if len(optionsRaw) > 0 {
		if err := json.Unmarshal(optionsRaw, &meta.Options); err != nil {
			return nil, fmt.Errorf("falha ao desserializar opções da enquete: %w", err)
		}
	} else {
		meta.Options = []string{}
	}

	return meta, nil
}
