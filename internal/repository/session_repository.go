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
	return &SessionRepository{db: db, logger: log}
}

const sessionSelectCols = `
	id, tenant_id, whatsapp_session_key, nome_pessoa, email_pessoa, phone_number, device_jid,
	status, qr_code, qr_code_expires_at, created_at, updated_at, last_connected_at
`

const sessionSelectBase = `
	SELECT ` + sessionSelectCols + `
	FROM whatsapp_sessions
`

func scanSession(scanner interface{ Scan(dest ...any) error }) (*models.WhatsAppSession, error) {
	s := &models.WhatsAppSession{}
	if err := scanner.Scan(
		&s.ID,
		&s.TenantID,
		&s.WhatsAppSessionKey,
		&s.NomePessoa,
		&s.EmailPessoa,
		&s.PhoneNumber,
		&s.DeviceJID,
		&s.Status,
		&s.QRCode,
		&s.QRCodeExpiresAt,
		&s.CreatedAt,
		&s.UpdatedAt,
		&s.LastConnectedAt,
	); err != nil {
		return nil, err
	}
	return s, nil
}

func closeRows(log *logger.Logger, rows *sql.Rows) {
	if err := rows.Close(); err != nil {
		log.Errorf("Erro ao fechar linhas de sessão: %v", err)
	}
}

func (r *SessionRepository) Create(session *models.WhatsAppSession) error {
	exists, err := r.ExistsBySessionKeyAndTenant(session.WhatsAppSessionKey, session.TenantID)
	if err != nil {
		return fmt.Errorf("falha ao verificar sessão existente: %w", err)
	}
	if exists {
		return fmt.Errorf("já existe uma sessão com a chave: %s", session.WhatsAppSessionKey)
	}

	exists, err = r.ExistsByEmailAndTenant(session.EmailPessoa, session.TenantID)
	if err != nil {
		return fmt.Errorf("falha ao verificar email existente: %w", err)
	}
	if exists {
		return fmt.Errorf("já existe uma sessão com o email: %s para este tenant", session.EmailPessoa)
	}

	query := `
		INSERT INTO whatsapp_sessions (
			id, tenant_id, whatsapp_session_key, nome_pessoa, email_pessoa,
			status, qr_code, qr_code_expires_at, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`

	_, err = r.db.Exec(query,
		session.ID,
		session.TenantID,
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

	r.logger.Infof("Sessão criada com sucesso: %s (%s) [Tenant: %s]", session.WhatsAppSessionKey, session.ID, session.TenantID)
	return nil
}

func (r *SessionRepository) GetByID(id uuid.UUID) (*models.WhatsAppSession, error) {
	query := sessionSelectBase + ` WHERE id = $1`
	row := r.db.QueryRow(query, id)

	session, err := scanSession(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("sessão não encontrada")
	}
	if err != nil {
		return nil, fmt.Errorf("falha ao buscar sessão: %w", err)
	}

	return session, nil
}

func (r *SessionRepository) GetBySessionKey(sessionKey string) (*models.WhatsAppSession, error) {
	query := sessionSelectBase + ` WHERE whatsapp_session_key = $1`
	row := r.db.QueryRow(query, sessionKey)

	session, err := scanSession(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("sessão não encontrada")
	}
	if err != nil {
		return nil, fmt.Errorf("falha ao buscar sessão: %w", err)
	}

	return session, nil
}

func (r *SessionRepository) GetBySessionKeyAndTenant(sessionKey string, tenantID string) (*models.WhatsAppSession, error) {
	query := sessionSelectBase + ` WHERE whatsapp_session_key = $1 AND tenant_id = $2`
	row := r.db.QueryRow(query, sessionKey, tenantID)

	session, err := scanSession(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("sessão não encontrada para este tenant")
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
	if _, err := r.db.Exec(query, qrCode, expiresAt, time.Now(), id); err != nil {
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

	query := `
		UPDATE whatsapp_sessions
		SET status = $1,
		    phone_number = COALESCE(NULLIF($2, ''), phone_number),
		    device_jid   = COALESCE(NULLIF($3, ''), device_jid),
		    updated_at = $4,
		    last_connected_at = $5
		WHERE id = $6
	`

	if _, err := r.db.Exec(query, status, phoneNumber, deviceJID, now, lastConnectedAt, id); err != nil {
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

	if _, err := r.db.Exec(query, deviceJID, time.Now(), id); err != nil {
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

	if _, err := r.db.Exec(query, models.SessionStatusPending, time.Now(), id); err != nil {
		return fmt.Errorf("falha ao marcar logout: %w", err)
	}

	r.logger.Infof("Sessão marcada como pending após logout: %s", id)
	return nil
}

func (r *SessionRepository) ResetSessionForReRegister(id uuid.UUID, nomePessoa string, emailPessoa string) error {
	query := `
		UPDATE whatsapp_sessions
		SET nome_pessoa = $1,
		    email_pessoa = $2,
		    status = $3,
		    phone_number = NULL,
		    device_jid = NULL,
		    qr_code = NULL,
		    qr_code_expires_at = NULL,
		    updated_at = $4,
		    last_connected_at = NULL
		WHERE id = $5
	`

	if _, err := r.db.Exec(query, nomePessoa, emailPessoa, models.SessionStatusPending, time.Now(), id); err != nil {
		return fmt.Errorf("falha ao resetar sessão: %w", err)
	}
	return nil
}

func (r *SessionRepository) List() ([]*models.WhatsAppSession, error) {
	query := sessionSelectBase + ` ORDER BY created_at DESC`

	rows, err := r.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("falha ao listar sessões: %w", err)
	}
	defer closeRows(r.logger, rows)

	sessions := make([]*models.WhatsAppSession, 0)
	for rows.Next() {
		s, err := scanSession(rows)
		if err != nil {
			return nil, fmt.Errorf("falha ao escanear sessão: %w", err)
		}
		sessions = append(sessions, s)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("falha ao iterar sessões: %w", err)
	}

	return sessions, nil
}

func (r *SessionRepository) ListByTenant(tenantID string) ([]*models.WhatsAppSession, error) {
	query := sessionSelectBase + ` WHERE tenant_id = $1 ORDER BY created_at DESC`

	rows, err := r.db.Query(query, tenantID)
	if err != nil {
		return nil, fmt.Errorf("falha ao listar sessões do tenant: %w", err)
	}
	defer closeRows(r.logger, rows)

	sessions := make([]*models.WhatsAppSession, 0)
	for rows.Next() {
		s, err := scanSession(rows)
		if err != nil {
			return nil, fmt.Errorf("falha ao escanear sessão: %w", err)
		}
		sessions = append(sessions, s)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("falha ao iterar sessões: %w", err)
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
	if err := r.db.QueryRow(query, sessionKey).Scan(&exists); err != nil {
		return false, fmt.Errorf("falha ao verificar sessão existente: %w", err)
	}
	return exists, nil
}

func (r *SessionRepository) ExistsBySessionKeyAndTenant(sessionKey string, tenantID string) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM whatsapp_sessions WHERE whatsapp_session_key = $1 AND tenant_id = $2)`
	var exists bool
	if err := r.db.QueryRow(query, sessionKey, tenantID).Scan(&exists); err != nil {
		return false, fmt.Errorf("falha ao verificar sessão existente: %w", err)
	}
	return exists, nil
}

func (r *SessionRepository) ExistsByEmailAndTenant(email string, tenantID string) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM whatsapp_sessions WHERE email_pessoa = $1 AND tenant_id = $2)`
	var exists bool
	if err := r.db.QueryRow(query, email, tenantID).Scan(&exists); err != nil {
		return false, fmt.Errorf("falha ao verificar email existente: %w", err)
	}
	return exists, nil
}
