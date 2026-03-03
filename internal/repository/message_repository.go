package repository

import (
	"boot-whatsapp-golang/internal/models"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type MessageRepository struct {
	db *sql.DB
}

func NewMessageRepository(db *sql.DB) *MessageRepository {
	return &MessageRepository{db: db}
}

func (r *MessageRepository) SaveMessage(msg *models.Message) error {
	query := `
		INSERT INTO messages (
			id, session_id, tenant_id, whatsapp_message_id, direction,
			sender, recipient, message_type, content, media_url,
			mime_type, status, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14
		)
		RETURNING id
	`

	err := r.db.QueryRow(
		query,
		msg.ID,
		msg.SessionID,
		msg.TenantID,
		msg.WhatsAppMessageID,
		msg.Direction,
		msg.Sender,
		msg.Recipient,
		msg.MessageType,
		msg.Content,
		msg.MediaURL,
		msg.MimeType,
		msg.Status,
		msg.CreatedAt,
		msg.UpdatedAt,
	).Scan(&msg.ID)

	if err != nil {
		return fmt.Errorf("erro ao salvar mensagem: %w", err)
	}

	return nil
}

func (r *MessageRepository) UpdateMessageStatus(whatsappMessageID string, status string) error {
	query := `
		UPDATE messages
		SET status = $1, updated_at = $2
		WHERE whatsapp_message_id = $3
	`

	result, err := r.db.Exec(query, status, time.Now(), whatsappMessageID)
	if err != nil {
		return fmt.Errorf("erro ao atualizar status da mensagem: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("erro ao verificar linhas afetadas: %w", err)
	}

	if rowsAffected == 0 {
		return errors.New("mensagem não encontrada")
	}

	return nil
}

func (r *MessageRepository) GetMessageByID(id uuid.UUID) (*models.Message, error) {
	query := `
		SELECT id, session_id, tenant_id, whatsapp_message_id, direction,
		       sender, recipient, message_type, content, media_url, mime_type, status, created_at, updated_at
		FROM messages
		WHERE id = $1
	`

	msg := &models.Message{}
	err := r.db.QueryRow(query, id).Scan(
		&msg.ID, &msg.SessionID, &msg.TenantID, &msg.WhatsAppMessageID,
		&msg.Direction, &msg.Sender, &msg.Recipient, &msg.MessageType,
		&msg.Content, &msg.MediaURL, &msg.MimeType, &msg.Status,
		&msg.CreatedAt, &msg.UpdatedAt,
	)

	if err != nil {
		if errors.Is(sql.ErrNoRows, err) {
			return nil, errors.New("mensagem não encontrada")
		}
		return nil, fmt.Errorf("erro ao buscar mensagem: %w", err)
	}

	return msg, nil
}

func (r *MessageRepository) GetConversation(sessionID uuid.UUID, contactNumber string, limit int, offset int) ([]*models.Message, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 500 {
		limit = 500
	}

	query := `
		SELECT id, session_id, tenant_id, whatsapp_message_id, direction,
		       sender, recipient, message_type, content, media_url, mime_type, status, created_at, updated_at
		FROM messages
		WHERE session_id = $1
	`

	args := []interface{}{sessionID}
	nextArg := 2

	if contactNumber != "" {
		query += fmt.Sprintf(" AND (sender = $%d OR recipient = $%d)", nextArg, nextArg+1)
		args = append(args, contactNumber, contactNumber)
		nextArg += 2
	}

	query += fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d OFFSET $%d", nextArg, nextArg+1)
	args = append(args, limit, offset)

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("erro ao buscar conversa: %w", err)
	}
	defer rows.Close()

	var messages []*models.Message
	for rows.Next() {
		msg := &models.Message{}
		err := rows.Scan(
			&msg.ID, &msg.SessionID, &msg.TenantID, &msg.WhatsAppMessageID,
			&msg.Direction, &msg.Sender, &msg.Recipient, &msg.MessageType,
			&msg.Content, &msg.MediaURL, &msg.MimeType, &msg.Status,
			&msg.CreatedAt, &msg.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("erro ao ler mensagem: %w", err)
		}
		messages = append(messages, msg)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("erro ao percorrer mensagens: %w", err)
	}

	return messages, nil
}

func (r *MessageRepository) GetConversationCount(sessionID uuid.UUID, contactNumber string) (int, error) {
	query := "SELECT COUNT(*) FROM messages WHERE session_id = $1"
	args := []interface{}{sessionID}

	if contactNumber != "" {
		query += " AND (sender = $2 OR recipient = $2)"
		args = append(args, contactNumber)
	}

	var count int
	err := r.db.QueryRow(query, args...).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("erro ao contar mensagens: %w", err)
	}

	return count, nil
}

func (r *MessageRepository) GetMessagesByStatus(sessionID uuid.UUID, status string) ([]*models.Message, error) {
	query := `
		SELECT id, session_id, tenant_id, whatsapp_message_id, direction,
		       sender, recipient, message_type, content, media_url, mime_type, status, created_at, updated_at
		FROM messages
		WHERE session_id = $1 AND status = $2
		ORDER BY created_at DESC
	`

	rows, err := r.db.Query(query, sessionID, status)
	if err != nil {
		return nil, fmt.Errorf("erro ao buscar mensagens por status: %w", err)
	}
	defer rows.Close()

	var messages []*models.Message
	for rows.Next() {
		msg := &models.Message{}
		err := rows.Scan(
			&msg.ID, &msg.SessionID, &msg.TenantID, &msg.WhatsAppMessageID,
			&msg.Direction, &msg.Sender, &msg.Recipient, &msg.MessageType,
			&msg.Content, &msg.MediaURL, &msg.MimeType, &msg.Status,
			&msg.CreatedAt, &msg.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("erro ao ler mensagem: %w", err)
		}
		messages = append(messages, msg)
	}

	return messages, rows.Err()
}

func (r *MessageRepository) DeleteOldMessages(sessionID uuid.UUID, beforeDate time.Time) (int64, error) {
	result, err := r.db.Exec(
		"DELETE FROM messages WHERE session_id = $1 AND created_at < $2",
		sessionID, beforeDate,
	)
	if err != nil {
		return 0, fmt.Errorf("erro ao deletar mensagens antigas: %w", err)
	}

	return result.RowsAffected()
}

func (r *MessageRepository) GetContactNumbers(sessionID uuid.UUID) ([]string, error) {
	query := `
		SELECT DISTINCT COALESCE(sender, recipient) as contact
		FROM messages
		WHERE session_id = $1 AND (sender IS NOT NULL OR recipient IS NOT NULL)
		ORDER BY contact
	`

	rows, err := r.db.Query(query, sessionID)
	if err != nil {
		return nil, fmt.Errorf("erro ao buscar contatos: %w", err)
	}
	defer rows.Close()

	var contacts []string
	for rows.Next() {
		var contact string
		if err := rows.Scan(&contact); err != nil {
			return nil, fmt.Errorf("erro ao ler contato: %w", err)
		}
		contacts = append(contacts, contact)
	}

	return contacts, rows.Err()
}
