CREATE TABLE IF NOT EXISTS whatsapp_sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    whatsapp_session_key VARCHAR(255) UNIQUE NOT NULL,
    nome_pessoa VARCHAR(255) NOT NULL,
    email_pessoa VARCHAR(255) UNIQUE NOT NULL,
    phone_number VARCHAR(50),
    status VARCHAR(50) NOT NULL DEFAULT 'pending',
    qr_code TEXT,
    qr_code_expires_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_connected_at TIMESTAMP,
    
    CONSTRAINT chk_status CHECK (status IN ('pending', 'connected', 'disconnected', 'error'))
);

CREATE INDEX idx_whatsapp_sessions_key ON whatsapp_sessions(whatsapp_session_key);
CREATE INDEX idx_whatsapp_sessions_email ON whatsapp_sessions(email_pessoa);
CREATE INDEX idx_whatsapp_sessions_status ON whatsapp_sessions(status);
CREATE INDEX idx_whatsapp_sessions_created_at ON whatsapp_sessions(created_at DESC);

CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER update_whatsapp_sessions_updated_at
    BEFORE UPDATE ON whatsapp_sessions
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

COMMENT ON TABLE whatsapp_sessions IS 'Armazena as sessões de WhatsApp de múltiplos usuários';
COMMENT ON COLUMN whatsapp_sessions.id IS 'UUID único da sessão';
COMMENT ON COLUMN whatsapp_sessions.whatsapp_session_key IS 'Chave única de identificação da sessão (ex: botwhat01)';
COMMENT ON COLUMN whatsapp_sessions.nome_pessoa IS 'Nome completo da pessoa responsável pela sessão';
COMMENT ON COLUMN whatsapp_sessions.email_pessoa IS 'Email da pessoa (único)';
COMMENT ON COLUMN whatsapp_sessions.phone_number IS 'Número do WhatsApp conectado';
COMMENT ON COLUMN whatsapp_sessions.status IS 'Status da conexão: pending, connected, disconnected, error';
COMMENT ON COLUMN whatsapp_sessions.qr_code IS 'QR Code em base64 para conexão';
COMMENT ON COLUMN whatsapp_sessions.qr_code_expires_at IS 'Data/hora de expiração do QR Code';
COMMENT ON COLUMN whatsapp_sessions.last_connected_at IS 'Última vez que conectou com sucesso';
