-- Adicionar coluna para armazenar media em base64
-- Isso permite ler a mídia mesmo que a URL do WhatsApp tenha expirado

ALTER TABLE messages ADD COLUMN IF NOT EXISTS media_base64 BYTEA;

-- Além disso, aumentar o tamanho de mime_type se necessário
ALTER TABLE messages ALTER COLUMN mime_type TYPE VARCHAR(200);

-- Criar índice para buscar mensagens com mídia stored
CREATE INDEX IF NOT EXISTS idx_messages_has_media_base64 ON messages(id) WHERE media_base64 IS NOT NULL;
