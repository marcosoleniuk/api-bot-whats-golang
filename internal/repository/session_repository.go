package repository

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"boot-whatsapp-golang/internal/models"
	"boot-whatsapp-golang/pkg/logger"

	"github.com/google/uuid"
)

type SessionRepository struct {
	db     *sql.DB
	logger *logger.Logger
}

func NewSessionRepository(db *sql.DB, log *logger.Logger) *SessionRepository {
	return &SessionRepository{
		db:     db,
		logger: log,
	}
}

func (r *SessionRepository) Create(session *models.WhatsAppSession) error {
	exists, err := r.ExistsBySessionKey(session.WhatsAppSessionKey)
	if err != nil {
		return fmt.Errorf("falha ao verificar sessão existente: %w", err)
	}
	if exists {
		return fmt.Errorf("já existe uma sessão com a chave: %s", session.WhatsAppSessionKey)
	}

	exists, err = r.ExistsByEmail(session.EmailPessoa)
	if err != nil {
		return fmt.Errorf("falha ao verificar email existente: %w", err)
	}
	if exists {
		return fmt.Errorf("já existe uma sessão com o email: %s", session.EmailPessoa)
	}

	query := `
		INSERT INTO whatsapp_sessions (
			id, whatsapp_session_key, nome_pessoa, email_pessoa, 
			status, qr_code, qr_code_expires_at, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`

	_, err = r.db.Exec(query,
		session.ID,
		session.WhatsAppSessionKey,
		session.NomePessoa,
		session.EmailPessoa,
		session.Status,
		session.QRCode,
		session.QRCodeExpiresAt,
		session.CreatedAt,
		session.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("falha ao criar sessão: %w", err)
	}

	r.logger.Infof("Sessão criada com sucesso: %s (%s)", session.WhatsAppSessionKey, session.ID)
	return nil
}

func (r *SessionRepository) GetByID(id uuid.UUID) (*models.WhatsAppSession, error) {
	query := `
		SELECT id, whatsapp_session_key, nome_pessoa, email_pessoa, phone_number, device_jid,
		       status, qr_code, qr_code_expires_at, created_at, updated_at, last_connected_at
		FROM whatsapp_sessions
		WHERE id = $1
	`

	session := &models.WhatsAppSession{}
	err := r.db.QueryRow(query, id).Scan(
		&session.ID,
		&session.WhatsAppSessionKey,
		&session.NomePessoa,
		&session.EmailPessoa,
		&session.PhoneNumber,
		&session.DeviceJID,
		&session.Status,
		&session.QRCode,
		&session.QRCodeExpiresAt,
		&session.CreatedAt,
		&session.UpdatedAt,
		&session.LastConnectedAt,
	)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("sessão não encontrada")
	}
	if err != nil {
		return nil, fmt.Errorf("falha ao buscar sessão: %w", err)
	}

	return session, nil
}

func (r *SessionRepository) GetBySessionKey(sessionKey string) (*models.WhatsAppSession, error) {
	query := `
		SELECT id, whatsapp_session_key, nome_pessoa, email_pessoa, phone_number, device_jid,
		       status, qr_code, qr_code_expires_at, created_at, updated_at, last_connected_at
		FROM whatsapp_sessions
		WHERE whatsapp_session_key = $1
	`

	session := &models.WhatsAppSession{}
	err := r.db.QueryRow(query, sessionKey).Scan(
		&session.ID,
		&session.WhatsAppSessionKey,
		&session.NomePessoa,
		&session.EmailPessoa,
		&session.PhoneNumber,
		&session.DeviceJID,
		&session.Status,
		&session.QRCode,
		&session.QRCodeExpiresAt,
		&session.CreatedAt,
		&session.UpdatedAt,
		&session.LastConnectedAt,
	)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("sessão não encontrada")
	}
	if err != nil {
		return nil, fmt.Errorf("falha ao buscar sessão: %w", err)
	}

	return session, nil
}

func (r *SessionRepository) UpdateQRCode(id uuid.UUID, qrCode string, expiresAt time.Time) error {
	query := `
		UPDATE whatsapp_sessions
		SET qr_code = $1, qr_code_expires_at = $2, updated_at = $3
		WHERE id = $4
	`

	_, err := r.db.Exec(query, qrCode, expiresAt, time.Now(), id)
	if err != nil {
		return fmt.Errorf("falha ao atualizar QR code: %w", err)
	}

	return nil
}

func (r *SessionRepository) UpdateStatus(id uuid.UUID, status string, phoneNumber string, deviceJID string) error {
	now := time.Now()
	var lastConnectedAt *time.Time

	if status == models.SessionStatusConnected {
		lastConnectedAt = &now
	}

	// Se phoneNumber/deviceJID forem vazios, não atualizar os campos (manter valores existentes)
	var query string
	var args []interface{}

	if phoneNumber != "" && deviceJID != "" {
		// Atualiza status, phone_number e device_jid
		query = `
			UPDATE whatsapp_sessions
			SET status = $1, phone_number = $2, device_jid = $3, updated_at = $4, last_connected_at = $5
			WHERE id = $6
		`
		args = []interface{}{status, phoneNumber, deviceJID, now, lastConnectedAt, id}
	} else if phoneNumber != "" {
		// Atualiza status e phone_number
		query = `
			UPDATE whatsapp_sessions
			SET status = $1, phone_number = $2, updated_at = $3, last_connected_at = $4
			WHERE id = $5
		`
		args = []interface{}{status, phoneNumber, now, lastConnectedAt, id}
	} else if deviceJID != "" {
		// Atualiza status e device_jid
		query = `
			UPDATE whatsapp_sessions
			SET status = $1, device_jid = $2, updated_at = $3, last_connected_at = $4
			WHERE id = $5
		`
		args = []interface{}{status, deviceJID, now, lastConnectedAt, id}
	} else {
		// Atualiza apenas status (mantém phone_number existente)
		query = `
			UPDATE whatsapp_sessions
			SET status = $1, updated_at = $2, last_connected_at = $3
			WHERE id = $4
		`
		args = []interface{}{status, now, lastConnectedAt, id}
	}

	_, err := r.db.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("falha ao atualizar status: %w", err)
	}

	r.logger.Infof("Status da sessão atualizado: %s -> %s", id, status)
	return nil
}

func (r *SessionRepository) UpdateDeviceJID(id uuid.UUID, deviceJID string) error {
	if deviceJID == "" {
		return nil
	}

	query := `
		UPDATE whatsapp_sessions
		SET device_jid = $1, updated_at = $2
		WHERE id = $3
	`

	_, err := r.db.Exec(query, deviceJID, time.Now(), id)
	if err != nil {
		return fmt.Errorf("falha ao atualizar device_jid: %w", err)
	}

	r.logger.Infof("Device JID atualizado: %s", id)
	return nil
}

func (r *SessionRepository) MarkLoggedOut(id uuid.UUID) error {
	query := `
		UPDATE whatsapp_sessions
		SET status = $1, phone_number = NULL, device_jid = NULL, updated_at = $2
		WHERE id = $3
	`

	_, err := r.db.Exec(query, models.SessionStatusPending, time.Now(), id)
	if err != nil {
		return fmt.Errorf("falha ao marcar logout: %w", err)
	}

	r.logger.Infof("Sessão marcada como pending após logout: %s", id)
	return nil
}

func (r *SessionRepository) List() ([]*models.WhatsAppSession, error) {
	query := `
		SELECT id, whatsapp_session_key, nome_pessoa, email_pessoa, phone_number, device_jid,
		       status, qr_code, qr_code_expires_at, created_at, updated_at, last_connected_at
		FROM whatsapp_sessions
		ORDER BY created_at DESC
	`

	rows, err := r.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("falha ao listar sessões: %w", err)
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			r.logger.Errorf("Erro ao fechar linhas de sessão: %v", err)
		}
	}(rows)

	var sessions []*models.WhatsAppSession
	for rows.Next() {
		session := &models.WhatsAppSession{}
		err := rows.Scan(
			&session.ID,
			&session.WhatsAppSessionKey,
			&session.NomePessoa,
			&session.EmailPessoa,
			&session.PhoneNumber,
			&session.DeviceJID,
			&session.Status,
			&session.QRCode,
			&session.QRCodeExpiresAt,
			&session.CreatedAt,
			&session.UpdatedAt,
			&session.LastConnectedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("falha ao escanear sessão: %w", err)
		}
		sessions = append(sessions, session)
	}

	return sessions, nil
}

func (r *SessionRepository) Delete(id uuid.UUID) error {
	query := `DELETE FROM whatsapp_sessions WHERE id = $1`

	result, err := r.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("falha ao deletar sessão: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("falha ao verificar linhas afetadas: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("sessão não encontrada")
	}

	r.logger.Infof("Sessão deletada: %s", id)
	return nil
}

func (r *SessionRepository) ExistsBySessionKey(sessionKey string) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM whatsapp_sessions WHERE whatsapp_session_key = $1)`

	var exists bool
	err := r.db.QueryRow(query, sessionKey).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("falha ao verificar sessão existente: %w", err)
	}

	return exists, nil
}

func (r *SessionRepository) ExistsByEmail(email string) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM whatsapp_sessions WHERE email_pessoa = $1)`

	var exists bool
	err := r.db.QueryRow(query, email).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("falha ao verificar email existente: %w", err)
	}

	return exists, nil
}
