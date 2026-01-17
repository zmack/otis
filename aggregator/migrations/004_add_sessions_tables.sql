-- +goose Up
-- +goose StatementBegin

-- New sessions table with cleaner schema
CREATE TABLE sessions (
    session_id TEXT PRIMARY KEY,
    organization_id TEXT NOT NULL,
    user_id TEXT NOT NULL,
    start_time INTEGER NOT NULL,
    end_time INTEGER,

    -- Summary stats
    total_cost_usd REAL DEFAULT 0,
    total_input_tokens INTEGER DEFAULT 0,
    total_output_tokens INTEGER DEFAULT 0,
    total_cache_read_tokens INTEGER DEFAULT 0,
    total_cache_creation_tokens INTEGER DEFAULT 0,
    tool_call_count INTEGER DEFAULT 0,

    created_at INTEGER,
    updated_at INTEGER
);

CREATE INDEX idx_sessions_org ON sessions(organization_id);
CREATE INDEX idx_sessions_user ON sessions(user_id);
CREATE INDEX idx_sessions_start ON sessions(start_time);

-- Session tools breakdown table
CREATE TABLE session_tools (
    session_id TEXT NOT NULL,
    tool_name TEXT NOT NULL,
    call_count INTEGER DEFAULT 0,
    success_count INTEGER DEFAULT 0,
    failure_count INTEGER DEFAULT 0,
    total_execution_time_ms REAL DEFAULT 0,

    PRIMARY KEY (session_id, tool_name),
    FOREIGN KEY (session_id) REFERENCES sessions(session_id)
);

CREATE INDEX idx_session_tools_tool ON session_tools(tool_name);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_session_tools_tool;
DROP TABLE IF EXISTS session_tools;
DROP INDEX IF EXISTS idx_sessions_start;
DROP INDEX IF EXISTS idx_sessions_user;
DROP INDEX IF EXISTS idx_sessions_org;
DROP TABLE IF EXISTS sessions;
-- +goose StatementEnd
