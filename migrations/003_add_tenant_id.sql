ALTER TABLE whatsapp_sessions ADD COLUMN IF NOT EXISTS tenant_id VARCHAR(255);

CREATE INDEX IF NOT EXISTS idx_whatsapp_sessions_tenant_id ON whatsapp_sessions(tenant_id);

UPDATE whatsapp_sessions SET tenant_id = 'default' WHERE tenant_id IS NULL;
