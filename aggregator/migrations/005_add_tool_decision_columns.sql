-- +goose Up
-- +goose StatementBegin

-- Add decision tracking columns to session_tools
ALTER TABLE session_tools ADD COLUMN auto_approved_count INTEGER DEFAULT 0;
ALTER TABLE session_tools ADD COLUMN user_approved_count INTEGER DEFAULT 0;
ALTER TABLE session_tools ADD COLUMN rejected_count INTEGER DEFAULT 0;
ALTER TABLE session_tools ADD COLUMN total_result_size_bytes INTEGER DEFAULT 0;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- SQLite doesn't support DROP COLUMN, so we need to recreate the table
CREATE TABLE session_tools_backup (
    session_id TEXT NOT NULL,
    tool_name TEXT NOT NULL,
    call_count INTEGER DEFAULT 0,
    success_count INTEGER DEFAULT 0,
    failure_count INTEGER DEFAULT 0,
    total_execution_time_ms REAL DEFAULT 0,
    PRIMARY KEY (session_id, tool_name),
    FOREIGN KEY (session_id) REFERENCES sessions(session_id)
);

INSERT INTO session_tools_backup SELECT session_id, tool_name, call_count, success_count, failure_count, total_execution_time_ms FROM session_tools;
DROP TABLE session_tools;
ALTER TABLE session_tools_backup RENAME TO session_tools;

CREATE INDEX idx_session_tools_tool ON session_tools(tool_name);

-- +goose StatementEnd
