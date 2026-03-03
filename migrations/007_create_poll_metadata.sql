CREATE TABLE IF NOT EXISTS poll_metadata (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id UUID NOT NULL REFERENCES whatsapp_sessions(id) ON DELETE CASCADE,
    tenant_id VARCHAR(255) NOT NULL,
    chat_jid VARCHAR(255) NOT NULL,
    poll_message_id VARCHAR(255) NOT NULL,
    question TEXT NOT NULL DEFAULT '',
    options JSONB NOT NULL DEFAULT '[]'::jsonb,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT uq_poll_metadata_session_chat_poll UNIQUE (session_id, chat_jid, poll_message_id)
);

CREATE INDEX IF NOT EXISTS idx_poll_metadata_session_poll ON poll_metadata(session_id, poll_message_id);
CREATE INDEX IF NOT EXISTS idx_poll_metadata_tenant ON poll_metadata(tenant_id);
CREATE INDEX IF NOT EXISTS idx_poll_metadata_chat ON poll_metadata(chat_jid);
CREATE INDEX IF NOT EXISTS idx_poll_metadata_updated_at ON poll_metadata(updated_at DESC);
