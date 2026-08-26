DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM outbox_messages WHERE attempts < 0) THEN
    RAISE EXCEPTION 'cannot add outbox attempts constraint: conflicting rows exist';
  END IF;
  IF EXISTS (SELECT 1 FROM outbox_messages WHERE delivered_at IS NOT NULL AND failed_at IS NOT NULL) THEN
    RAISE EXCEPTION 'cannot add outbox terminal state constraint: conflicting rows exist';
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='outbox_attempts_check' AND conrelid='outbox_messages'::regclass) THEN
    ALTER TABLE outbox_messages ADD CONSTRAINT outbox_attempts_check CHECK (attempts >= 0);
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='outbox_terminal_state_check' AND conrelid='outbox_messages'::regclass) THEN
    ALTER TABLE outbox_messages ADD CONSTRAINT outbox_terminal_state_check CHECK (delivered_at IS NULL OR failed_at IS NULL);
  END IF;
END $$;
