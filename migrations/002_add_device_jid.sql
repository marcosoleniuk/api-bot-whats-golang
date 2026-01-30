ALTER TABLE whatsapp_sessions
ADD COLUMN IF NOT EXISTS device_jid VARCHAR(255);

CREATE INDEX IF NOT EXISTS idx_whatsapp_sessions_device_jid
ON whatsapp_sessions (device_jid);
