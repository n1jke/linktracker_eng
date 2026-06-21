BEGIN;

CREATE TABLE outbox (
  id SERIAL PRIMARY KEY,
  -- ResourceShot
  shot_id BIGINT NOT NULL,
  url TEXT NOT NULL,
  description TEXT NOT NULL,
  chat_ids BIGINT[] NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL,
  -- retry data
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  processed_at TIMESTAMPTZ,
  error TEXT,
  retry_count INT NOT NULL DEFAULT 0
);

CREATE INDEX idx_outbox_not_processed
  ON outbox(created_at)
  WHERE processed_at IS NULL;

COMMIT;
