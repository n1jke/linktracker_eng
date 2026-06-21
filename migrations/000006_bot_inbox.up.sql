BEGIN;

CREATE TABLE bot_inbox (
  id SERIAL PRIMARY KEY,
  -- Update
  idempotency_key TEXT NOT NULL,
  shot_id BIGINT NOT NULL,
  url TEXT NOT NULL,
  description TEXT NOT NULL,
  chat_ids BIGINT[] NOT NULL,
  --
  status TEXT NOT NULL DEFAULT 'pending',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE(idempotency_key)
);

CREATE INDEX idx_bot_inbox_pending
  ON bot_inbox(created_at)
  WHERE status = 'pending';

COMMIT;