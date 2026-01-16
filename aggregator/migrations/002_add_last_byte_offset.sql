-- +goose Up
-- Add last_byte_offset column to processing_state table
-- Note: This will fail if column already exists, which is handled by the pre-migration fix
ALTER TABLE processing_state ADD COLUMN last_byte_offset INTEGER DEFAULT 0;

-- +goose Down
-- SQLite doesn't support DROP COLUMN in versions before 3.35.0
-- For older versions, you'd need to recreate the table
-- This is intentionally left as a no-op for safety
SELECT 1;
