-- Tabela para armazenar histórico de mensagens
CREATE TABLE IF NOT EXISTS messages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id UUID NOT NULL REFERENCES whatsapp_sessions(id) ON DELETE CASCADE,
    tenant_id VARCHAR(255) NOT NULL,
    whatsapp_message_id VARCHAR(255),
    direction VARCHAR(20) NOT NULL, -- 'inbound' ou 'outbound'
    sender VARCHAR(255),
    recipient VARCHAR(255),
    message_type VARCHAR(50) NOT NULL DEFAULT 'text', -- 'text', 'image', 'document', 'video', etc
    content TEXT,
    media_url VARCHAR(500),
    mime_type VARCHAR(100),
    status VARCHAR(50) NOT NULL DEFAULT 'pending', -- 'pending', 'sent', 'delivered', 'read', 'failed'
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    
    CONSTRAINT chk_direction CHECK (direction IN ('inbound', 'outbound')),
    CONSTRAINT chk_message_type CHECK (message_type IN ('text', 'image', 'document', 'video', 'audio', 'sticker', 'location', 'contact', 'reaction', 'unknown'))
);

-- Índices para otimizar buscas
CREATE INDEX idx_messages_session_id ON messages(session_id);
CREATE INDEX idx_messages_tenant_id ON messages(tenant_id);
CREATE INDEX idx_messages_whatsapp_message_id ON messages(whatsapp_message_id);
CREATE INDEX idx_messages_direction ON messages(direction);
CREATE INDEX idx_messages_status ON messages(status);
CREATE INDEX idx_messages_created_at ON messages(created_at DESC);
CREATE INDEX idx_messages_sender ON messages(sender);
CREATE INDEX idx_messages_recipient ON messages(recipient);
CREATE INDEX idx_messages_session_created ON messages(session_id, created_at DESC);

-- Trigger para atualizar updated_at
CREATE TRIGGER update_messages_updated_at
BEFORE UPDATE ON messages
FOR EACH ROW
EXECUTE FUNCTION update_updated_at_column();
