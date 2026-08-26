ALTER TABLE outbox_messages ADD COLUMN IF NOT EXISTS failed_at TIMESTAMPTZ;
ALTER TABLE outbox_messages ADD COLUMN IF NOT EXISTS failure_reason TEXT;

DROP INDEX IF EXISTS idx_outbox_ready;
CREATE INDEX idx_outbox_ready ON outbox_messages(available_at)
WHERE delivered_at IS NULL AND failed_at IS NULL;
