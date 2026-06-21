BEGIN;

ALTER TABLE links ADD COLUMN created_at TIMESTAMPTZ NOT NULL DEFAULT NOW();
CREATE INDEX index_links_created_at ON links(created_at);

COMMIT;
