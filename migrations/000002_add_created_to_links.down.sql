BEGIN;

DROP INDEX index_links_created_at;
ALTER TABLE links DROP COLUMN created_at;

COMMIT;
