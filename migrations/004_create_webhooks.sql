CREATE TABLE IF NOT EXISTS webhooks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id UUID NOT NULL,
    tenant_id VARCHAR(255) NOT NULL,
    url VARCHAR(2000) NOT NULL,
    events VARCHAR(255)[] NOT NULL DEFAULT ARRAY['message.sent', 'message.received'],
    headers JSONB,
    active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_triggered_at TIMESTAMP,
    
    CONSTRAINT fk_webhooks_session FOREIGN KEY (session_id) REFERENCES whatsapp_sessions(id) ON DELETE CASCADE,
    CONSTRAINT chk_webhook_url CHECK (url ~ '^https?://')
);

CREATE INDEX idx_webhooks_session_id ON webhooks(session_id);
CREATE INDEX idx_webhooks_tenant_id ON webhooks(tenant_id);
CREATE INDEX idx_webhooks_active ON webhooks(active);
CREATE INDEX idx_webhooks_created_at ON webhooks(created_at DESC);

CREATE TABLE IF NOT EXISTS webhook_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    webhook_id UUID NOT NULL,
    event_type VARCHAR(100) NOT NULL,
    payload JSONB NOT NULL,
    http_status INT,
    response_body TEXT,
    error_message TEXT,
    execution_time_ms INT,
    triggered_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    
    CONSTRAINT fk_webhook_logs_webhook FOREIGN KEY (webhook_id) REFERENCES webhooks(id) ON DELETE CASCADE
);

CREATE INDEX idx_webhook_logs_webhook_id ON webhook_logs(webhook_id);
CREATE INDEX idx_webhook_logs_event_type ON webhook_logs(event_type);
CREATE INDEX idx_webhook_logs_triggered_at ON webhook_logs(triggered_at DESC);

CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER update_webhooks_updated_at
    BEFORE UPDATE ON webhooks
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

COMMENT ON TABLE webhooks IS 'Armazena URLs de webhooks para notificações de mensagens';
COMMENT ON COLUMN webhooks.id IS 'UUID único do webhook';
COMMENT ON COLUMN webhooks.session_id IS 'Referência à sessão WhatsApp';
COMMENT ON COLUMN webhooks.tenant_id IS 'ID do tenant';
COMMENT ON COLUMN webhooks.url IS 'URL do webhook';
COMMENT ON COLUMN webhooks.events IS 'Array de eventos que disparam este webhook';
COMMENT ON COLUMN webhooks.headers IS 'Headers customizados em formato JSON';
COMMENT ON COLUMN webhooks.active IS 'Se o webhook está ativo';
COMMENT ON COLUMN webhooks.last_triggered_at IS 'Última vez que o webhook foi disparado';

COMMENT ON TABLE webhook_logs IS 'Log de execuções de webhooks';
COMMENT ON COLUMN webhook_logs.webhook_id IS 'Referência ao webhook';
COMMENT ON COLUMN webhook_logs.event_type IS 'Tipo de evento disparado';
COMMENT ON COLUMN webhook_logs.payload IS 'Payload enviado ao webhook';
COMMENT ON COLUMN webhook_logs.http_status IS 'Status HTTP da resposta';
COMMENT ON COLUMN webhook_logs.response_body IS 'Corpo da resposta';
COMMENT ON COLUMN webhook_logs.error_message IS 'Mensagem de erro (se houver)';
COMMENT ON COLUMN webhook_logs.execution_time_ms IS 'Tempo de execução em milissegundos';
