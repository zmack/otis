-- +goose Up
CREATE TABLE IF NOT EXISTS session_stats (
    session_id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    organization_id TEXT NOT NULL,
    service_name TEXT,
    start_time INTEGER,
    last_update_time INTEGER,

    terminal_type TEXT,
    host_arch TEXT,
    os_type TEXT,

    total_cost_usd REAL DEFAULT 0,
    total_input_tokens INTEGER DEFAULT 0,
    total_output_tokens INTEGER DEFAULT 0,
    total_cache_read_tokens INTEGER DEFAULT 0,
    total_cache_creation_tokens INTEGER DEFAULT 0,
    total_active_time_seconds REAL DEFAULT 0,

    api_request_count INTEGER DEFAULT 0,
    user_prompt_count INTEGER DEFAULT 0,
    tool_execution_count INTEGER DEFAULT 0,
    tool_success_count INTEGER DEFAULT 0,
    tool_failure_count INTEGER DEFAULT 0,

    avg_api_latency_ms REAL DEFAULT 0,
    total_api_latency_ms REAL DEFAULT 0,

    models_used TEXT,
    tools_used TEXT,

    created_at INTEGER,
    updated_at INTEGER
);

CREATE INDEX IF NOT EXISTS idx_session_user_id ON session_stats(user_id);
CREATE INDEX IF NOT EXISTS idx_session_org_id ON session_stats(organization_id);
CREATE INDEX IF NOT EXISTS idx_session_start_time ON session_stats(start_time);

CREATE TABLE IF NOT EXISTS user_stats (
    user_id TEXT NOT NULL,
    organization_id TEXT NOT NULL,
    window_start INTEGER,
    window_end INTEGER,
    window_type TEXT,

    total_sessions INTEGER DEFAULT 0,
    total_cost_usd REAL DEFAULT 0,
    total_input_tokens INTEGER DEFAULT 0,
    total_output_tokens INTEGER DEFAULT 0,
    total_cache_read_tokens INTEGER DEFAULT 0,
    total_cache_creation_tokens INTEGER DEFAULT 0,
    total_active_time_seconds REAL DEFAULT 0,

    avg_cost_per_session REAL DEFAULT 0,
    avg_tokens_per_session REAL DEFAULT 0,
    avg_session_duration_seconds REAL DEFAULT 0,

    preferred_models TEXT,
    favorite_tools TEXT,

    tool_success_rate REAL DEFAULT 0,

    last_session_time INTEGER,
    created_at INTEGER,
    updated_at INTEGER,

    PRIMARY KEY (user_id, window_type, window_start)
);

CREATE INDEX IF NOT EXISTS idx_user_org_id ON user_stats(organization_id);
CREATE INDEX IF NOT EXISTS idx_user_window ON user_stats(window_start, window_end);

CREATE TABLE IF NOT EXISTS org_stats (
    organization_id TEXT NOT NULL,
    window_start INTEGER,
    window_end INTEGER,
    window_type TEXT,

    total_users INTEGER DEFAULT 0,
    total_sessions INTEGER DEFAULT 0,
    total_cost_usd REAL DEFAULT 0,
    total_tokens INTEGER DEFAULT 0,
    total_active_time_seconds REAL DEFAULT 0,

    avg_cost_per_user REAL DEFAULT 0,
    avg_sessions_per_user REAL DEFAULT 0,

    top_users_by_cost TEXT,
    top_users_by_usage TEXT,

    created_at INTEGER,
    updated_at INTEGER,

    PRIMARY KEY (organization_id, window_type, window_start)
);

CREATE INDEX IF NOT EXISTS idx_org_window ON org_stats(window_start, window_end);

CREATE TABLE IF NOT EXISTS processing_state (
    file_name TEXT PRIMARY KEY,
    last_processed_time INTEGER,
    file_size_bytes INTEGER,
    updated_at INTEGER
);

CREATE TABLE IF NOT EXISTS session_model_stats (
    session_id TEXT NOT NULL,
    model TEXT NOT NULL,
    cost_usd REAL DEFAULT 0,
    input_tokens INTEGER DEFAULT 0,
    output_tokens INTEGER DEFAULT 0,
    cache_read_tokens INTEGER DEFAULT 0,
    cache_creation_tokens INTEGER DEFAULT 0,
    request_count INTEGER DEFAULT 0,
    total_latency_ms REAL DEFAULT 0,
    avg_latency_ms REAL DEFAULT 0,
    PRIMARY KEY (session_id, model),
    FOREIGN KEY (session_id) REFERENCES session_stats(session_id)
);

CREATE INDEX IF NOT EXISTS idx_model_name ON session_model_stats(model);
CREATE INDEX IF NOT EXISTS idx_model_session ON session_model_stats(session_id);

CREATE TABLE IF NOT EXISTS session_tool_stats (
    session_id TEXT NOT NULL,
    tool_name TEXT NOT NULL,
    execution_count INTEGER DEFAULT 0,
    success_count INTEGER DEFAULT 0,
    failure_count INTEGER DEFAULT 0,
    total_duration_ms REAL DEFAULT 0,
    avg_duration_ms REAL DEFAULT 0,
    min_duration_ms REAL DEFAULT 0,
    max_duration_ms REAL DEFAULT 0,
    PRIMARY KEY (session_id, tool_name),
    FOREIGN KEY (session_id) REFERENCES session_stats(session_id)
);

CREATE INDEX IF NOT EXISTS idx_tool_name ON session_tool_stats(tool_name);
CREATE INDEX IF NOT EXISTS idx_tool_session ON session_tool_stats(session_id);

-- +goose Down
DROP TABLE IF EXISTS session_tool_stats;
DROP TABLE IF EXISTS session_model_stats;
DROP TABLE IF EXISTS processing_state;
DROP TABLE IF EXISTS org_stats;
DROP TABLE IF EXISTS user_stats;
DROP TABLE IF EXISTS session_stats;
