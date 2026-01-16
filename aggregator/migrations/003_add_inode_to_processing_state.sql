-- +goose Up
-- Add inode column for file rotation detection
-- When inode changes, we know the file was rotated (renamed + new file created)
ALTER TABLE processing_state ADD COLUMN inode INTEGER DEFAULT 0;

-- +goose Down
-- SQLite 3.35.0+ supports DROP COLUMN, but for compatibility we leave as no-op
SELECT 1;
