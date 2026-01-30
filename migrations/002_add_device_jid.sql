-- Adiciona coluna para armazenar o device JID do WhatsApp
ALTER TABLE whatsapp_sessions
ADD COLUMN IF NOT EXISTS device_jid VARCHAR(255);

-- Índice para acelerar buscas por device_jid
CREATE INDEX IF NOT EXISTS idx_whatsapp_sessions_device_jid
ON whatsapp_sessions (device_jid);
