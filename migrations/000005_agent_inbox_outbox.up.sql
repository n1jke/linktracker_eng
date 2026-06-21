BEGIN;

CREATE TABLE agent_inbox (
  id SERIAL PRIMARY KEY,
  -- RawUpdate
  update_id BIGINT,
  url TEXT,
  event_type TEXT,
  description TEXT,
  author TEXT,
  chat_ids BIGINT[],
  updated_at TIMESTAMPTZ,
  --
  status TEXT NOT NULL DEFAULT 'pending',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE(update_id)
);

CREATE TABLE agent_outbox (
  id SERIAL PRIMARY KEY,
  -- PreparedUpdate
  update_id BIGINT,
  url TEXT,
  description TEXT,
  chat_ids BIGINT[],
  updated_at TIMESTAMPTZ,
  priority TEXT,
  -- retry data
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  processed_at TIMESTAMPTZ,
  error TEXT,
  retry_count INT NOT NULL DEFAULT 0
);

CREATE INDEX idx_agent_inbox_pending
  ON agent_inbox(created_at)
  WHERE status = 'pending';

CREATE INDEX idx_agent_outbox_pending
  ON agent_outbox(created_at)
  WHERE processed_at IS NULL;

COMMIT;