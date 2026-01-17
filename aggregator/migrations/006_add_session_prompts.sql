-- +goose Up
CREATE TABLE session_prompts (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id TEXT NOT NULL,
    prompt_text TEXT NOT NULL,
    prompt_length INTEGER,
    timestamp INTEGER NOT NULL,

    FOREIGN KEY (session_id) REFERENCES sessions(session_id),
    UNIQUE(session_id, timestamp)
);

CREATE INDEX idx_session_prompts_session ON session_prompts(session_id);
CREATE INDEX idx_session_prompts_timestamp ON session_prompts(timestamp);

-- +goose Down
DROP INDEX IF EXISTS idx_session_prompts_timestamp;
DROP INDEX IF EXISTS idx_session_prompts_session;
DROP TABLE IF EXISTS session_prompts;
