-- +goose Up
-- +goose StatementBegin

-- Add environment/client columns to sessions table
ALTER TABLE sessions ADD COLUMN client_name TEXT;
ALTER TABLE sessions ADD COLUMN client_version TEXT;
ALTER TABLE sessions ADD COLUMN terminal_type TEXT;
ALTER TABLE sessions ADD COLUMN host_arch TEXT;
ALTER TABLE sessions ADD COLUMN os_type TEXT;
ALTER TABLE sessions ADD COLUMN os_version TEXT;

-- Add missing aggregate columns
ALTER TABLE sessions ADD COLUMN api_request_count INTEGER DEFAULT 0;
ALTER TABLE sessions ADD COLUMN api_error_count INTEGER DEFAULT 0;
ALTER TABLE sessions ADD COLUMN user_prompt_count INTEGER DEFAULT 0;
ALTER TABLE sessions ADD COLUMN total_api_latency_ms REAL DEFAULT 0;

-- Create session_models table for per-model breakdown
CREATE TABLE session_models (
    session_id TEXT NOT NULL,
    model TEXT NOT NULL,
    request_count INTEGER DEFAULT 0,
    cost_usd REAL DEFAULT 0,
    input_tokens INTEGER DEFAULT 0,
    output_tokens INTEGER DEFAULT 0,
    cache_read_tokens INTEGER DEFAULT 0,
    cache_creation_tokens INTEGER DEFAULT 0,
    total_latency_ms REAL DEFAULT 0,

    PRIMARY KEY (session_id, model),
    FOREIGN KEY (session_id) REFERENCES sessions(session_id)
);

CREATE INDEX idx_session_models_model ON session_models(model);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP INDEX IF EXISTS idx_session_models_model;
DROP TABLE IF EXISTS session_models;

-- SQLite doesn't support DROP COLUMN easily, so we recreate
CREATE TABLE sessions_backup (
    session_id TEXT PRIMARY KEY,
    organization_id TEXT NOT NULL,
    user_id TEXT NOT NULL,
    start_time INTEGER NOT NULL,
    end_time INTEGER,
    total_cost_usd REAL DEFAULT 0,
    total_input_tokens INTEGER DEFAULT 0,
    total_output_tokens INTEGER DEFAULT 0,
    total_cache_read_tokens INTEGER DEFAULT 0,
    total_cache_creation_tokens INTEGER DEFAULT 0,
    tool_call_count INTEGER DEFAULT 0,
    created_at INTEGER,
    updated_at INTEGER
);

INSERT INTO sessions_backup SELECT
    session_id, organization_id, user_id, start_time, end_time,
    total_cost_usd, total_input_tokens, total_output_tokens,
    total_cache_read_tokens, total_cache_creation_tokens, tool_call_count,
    created_at, updated_at
FROM sessions;

DROP TABLE sessions;
ALTER TABLE sessions_backup RENAME TO sessions;

CREATE INDEX idx_sessions_org ON sessions(organization_id);
CREATE INDEX idx_sessions_user ON sessions(user_id);
CREATE INDEX idx_sessions_start ON sessions(start_time);

-- +goose StatementEnd
