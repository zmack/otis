package aggregator

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type Store struct {
	db *sql.DB
}

// NewStore creates a new Store instance and initializes the database
func NewStore(dbPath string) (*Store, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Enable WAL mode for better concurrent access
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		return nil, fmt.Errorf("failed to enable WAL mode: %w", err)
	}

	store := &Store{db: db}
	if err := store.InitSchema(); err != nil {
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	return store, nil
}

// InitSchema creates the database tables if they don't exist
func (s *Store) InitSchema() error {
	schema := `
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
		last_byte_offset INTEGER DEFAULT 0,
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
	`

	_, err := s.db.Exec(schema)
	return err
}

// Close closes the database connection
func (s *Store) Close() error {
	return s.db.Close()
}

// UpsertSessionStats inserts or updates session statistics
func (s *Store) UpsertSessionStats(stats *SessionStats) error {
	query := `
	INSERT INTO session_stats (
		session_id, user_id, organization_id, service_name,
		start_time, last_update_time,
		terminal_type, host_arch, os_type,
		total_cost_usd, total_input_tokens, total_output_tokens,
		total_cache_read_tokens, total_cache_creation_tokens, total_active_time_seconds,
		api_request_count, user_prompt_count, tool_execution_count,
		tool_success_count, tool_failure_count,
		avg_api_latency_ms, total_api_latency_ms,
		models_used, tools_used,
		created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(session_id) DO UPDATE SET
		last_update_time = excluded.last_update_time,
		total_cost_usd = excluded.total_cost_usd,
		total_input_tokens = excluded.total_input_tokens,
		total_output_tokens = excluded.total_output_tokens,
		total_cache_read_tokens = excluded.total_cache_read_tokens,
		total_cache_creation_tokens = excluded.total_cache_creation_tokens,
		total_active_time_seconds = excluded.total_active_time_seconds,
		api_request_count = excluded.api_request_count,
		user_prompt_count = excluded.user_prompt_count,
		tool_execution_count = excluded.tool_execution_count,
		tool_success_count = excluded.tool_success_count,
		tool_failure_count = excluded.tool_failure_count,
		avg_api_latency_ms = excluded.avg_api_latency_ms,
		total_api_latency_ms = excluded.total_api_latency_ms,
		models_used = excluded.models_used,
		tools_used = excluded.tools_used,
		updated_at = excluded.updated_at
	`

	_, err := s.db.Exec(query,
		stats.SessionID, stats.UserID, stats.OrganizationID, stats.ServiceName,
		stats.StartTime.Unix(), stats.LastUpdateTime.Unix(),
		stats.TerminalType, stats.HostArch, stats.OSType,
		stats.TotalCostUSD, stats.TotalInputTokens, stats.TotalOutputTokens,
		stats.TotalCacheReadTokens, stats.TotalCacheCreationTokens, stats.TotalActiveTimeSeconds,
		stats.APIRequestCount, stats.UserPromptCount, stats.ToolExecutionCount,
		stats.ToolSuccessCount, stats.ToolFailureCount,
		stats.AvgAPILatencyMS, stats.TotalAPILatencyMS,
		stats.ModelsUsed, stats.ToolsUsed,
		stats.CreatedAt.Unix(), stats.UpdatedAt.Unix(),
	)

	return err
}

// UpsertSessionModelStats upserts model statistics for a session
func (s *Store) UpsertSessionModelStats(modelStats *SessionModelStats) error {
	query := `
	INSERT INTO session_model_stats (
		session_id, model, cost_usd, input_tokens, output_tokens,
		cache_read_tokens, cache_creation_tokens, request_count,
		total_latency_ms, avg_latency_ms
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(session_id, model) DO UPDATE SET
		cost_usd = excluded.cost_usd,
		input_tokens = excluded.input_tokens,
		output_tokens = excluded.output_tokens,
		cache_read_tokens = excluded.cache_read_tokens,
		cache_creation_tokens = excluded.cache_creation_tokens,
		request_count = excluded.request_count,
		total_latency_ms = excluded.total_latency_ms,
		avg_latency_ms = excluded.avg_latency_ms
	`

	_, err := s.db.Exec(query,
		modelStats.SessionID, modelStats.Model, modelStats.CostUSD,
		modelStats.InputTokens, modelStats.OutputTokens,
		modelStats.CacheReadTokens, modelStats.CacheCreationTokens,
		modelStats.RequestCount, modelStats.TotalLatencyMS, modelStats.AvgLatencyMS,
	)

	return err
}

// UpsertSessionToolStats upserts tool statistics for a session
func (s *Store) UpsertSessionToolStats(toolStats *SessionToolStats) error {
	query := `
	INSERT INTO session_tool_stats (
		session_id, tool_name, execution_count, success_count, failure_count,
		total_duration_ms, avg_duration_ms, min_duration_ms, max_duration_ms
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(session_id, tool_name) DO UPDATE SET
		execution_count = excluded.execution_count,
		success_count = excluded.success_count,
		failure_count = excluded.failure_count,
		total_duration_ms = excluded.total_duration_ms,
		avg_duration_ms = excluded.avg_duration_ms,
		min_duration_ms = excluded.min_duration_ms,
		max_duration_ms = excluded.max_duration_ms
	`

	_, err := s.db.Exec(query,
		toolStats.SessionID, toolStats.ToolName,
		toolStats.ExecutionCount, toolStats.SuccessCount, toolStats.FailureCount,
		toolStats.TotalDurationMS, toolStats.AvgDurationMS,
		toolStats.MinDurationMS, toolStats.MaxDurationMS,
	)

	return err
}

// GetSessionStats retrieves statistics for a specific session
func (s *Store) GetSessionStats(sessionID string) (*SessionStats, error) {
	query := `
	SELECT session_id, user_id, organization_id, service_name,
		start_time, last_update_time,
		terminal_type, host_arch, os_type,
		total_cost_usd, total_input_tokens, total_output_tokens,
		total_cache_read_tokens, total_cache_creation_tokens, total_active_time_seconds,
		api_request_count, user_prompt_count, tool_execution_count,
		tool_success_count, tool_failure_count,
		avg_api_latency_ms, total_api_latency_ms,
		models_used, tools_used,
		created_at, updated_at
	FROM session_stats WHERE session_id = ?
	`

	var stats SessionStats
	var startTime, lastUpdateTime, createdAt, updatedAt int64
	var serviceName, terminalType, hostArch, osType sql.NullString
	var modelsUsed, toolsUsed sql.NullString

	err := s.db.QueryRow(query, sessionID).Scan(
		&stats.SessionID, &stats.UserID, &stats.OrganizationID, &serviceName,
		&startTime, &lastUpdateTime,
		&terminalType, &hostArch, &osType,
		&stats.TotalCostUSD, &stats.TotalInputTokens, &stats.TotalOutputTokens,
		&stats.TotalCacheReadTokens, &stats.TotalCacheCreationTokens, &stats.TotalActiveTimeSeconds,
		&stats.APIRequestCount, &stats.UserPromptCount, &stats.ToolExecutionCount,
		&stats.ToolSuccessCount, &stats.ToolFailureCount,
		&stats.AvgAPILatencyMS, &stats.TotalAPILatencyMS,
		&modelsUsed, &toolsUsed,
		&createdAt, &updatedAt,
	)

	if err != nil {
		return nil, err
	}

	stats.ServiceName = serviceName.String
	stats.TerminalType = terminalType.String
	stats.HostArch = hostArch.String
	stats.OSType = osType.String
	stats.ModelsUsed = modelsUsed.String
	stats.ToolsUsed = toolsUsed.String
	stats.StartTime = time.Unix(startTime, 0)
	stats.LastUpdateTime = time.Unix(lastUpdateTime, 0)
	stats.CreatedAt = time.Unix(createdAt, 0)
	stats.UpdatedAt = time.Unix(updatedAt, 0)

	return &stats, nil
}

// UpdateProcessingState updates the processing state for a file
func (s *Store) UpdateProcessingState(fileName string, byteOffset int64, fileSize int64) error {
	query := `
	INSERT INTO processing_state (file_name, last_byte_offset, last_processed_time, file_size_bytes, updated_at)
	VALUES (?, ?, ?, ?, ?)
	ON CONFLICT(file_name) DO UPDATE SET
		last_byte_offset = excluded.last_byte_offset,
		last_processed_time = excluded.last_processed_time,
		file_size_bytes = excluded.file_size_bytes,
		updated_at = excluded.updated_at
	`

	now := time.Now().Unix()
	_, err := s.db.Exec(query, fileName, byteOffset, now, fileSize, now)
	return err
}

// GetProcessingState retrieves the processing state for a file
func (s *Store) GetProcessingState(fileName string) (*ProcessingState, error) {
	query := `
	SELECT file_name, last_byte_offset, last_processed_time, file_size_bytes, updated_at
	FROM processing_state WHERE file_name = ?
	`

	var state ProcessingState
	var lastProcessedTime, updatedAt int64

	err := s.db.QueryRow(query, fileName).Scan(
		&state.FileName, &state.LastByteOffset, &lastProcessedTime,
		&state.FileSizeBytes, &updatedAt,
	)

	if err == sql.ErrNoRows {
		// Return empty state if not found
		return &ProcessingState{
			FileName:       fileName,
			LastByteOffset: 0,
		}, nil
	}

	if err != nil {
		return nil, err
	}

	state.LastProcessedTime = time.Unix(lastProcessedTime, 0)
	state.UpdatedAt = time.Unix(updatedAt, 0)

	return &state, nil
}

// GetUserSessionStats retrieves all sessions for a user
func (s *Store) GetUserSessionStats(userID string, limit int) ([]*SessionStats, error) {
	query := `
	SELECT session_id, user_id, organization_id, service_name,
		start_time, last_update_time,
		terminal_type, host_arch, os_type,
		total_cost_usd, total_input_tokens, total_output_tokens,
		total_cache_read_tokens, total_cache_creation_tokens, total_active_time_seconds,
		api_request_count, user_prompt_count, tool_execution_count,
		tool_success_count, tool_failure_count,
		avg_api_latency_ms, total_api_latency_ms,
		models_used, tools_used,
		created_at, updated_at
	FROM session_stats WHERE user_id = ?
	ORDER BY start_time DESC
	LIMIT ?
	`

	rows, err := s.db.Query(query, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []*SessionStats
	for rows.Next() {
		var stats SessionStats
		var startTime, lastUpdateTime, createdAt, updatedAt int64
		var serviceName, terminalType, hostArch, osType sql.NullString
		var modelsUsed, toolsUsed sql.NullString

		err := rows.Scan(
			&stats.SessionID, &stats.UserID, &stats.OrganizationID, &serviceName,
			&startTime, &lastUpdateTime,
			&terminalType, &hostArch, &osType,
			&stats.TotalCostUSD, &stats.TotalInputTokens, &stats.TotalOutputTokens,
			&stats.TotalCacheReadTokens, &stats.TotalCacheCreationTokens, &stats.TotalActiveTimeSeconds,
			&stats.APIRequestCount, &stats.UserPromptCount, &stats.ToolExecutionCount,
			&stats.ToolSuccessCount, &stats.ToolFailureCount,
			&stats.AvgAPILatencyMS, &stats.TotalAPILatencyMS,
			&modelsUsed, &toolsUsed,
			&createdAt, &updatedAt,
		)

		if err != nil {
			return nil, err
		}

		stats.ServiceName = serviceName.String
		stats.TerminalType = terminalType.String
		stats.HostArch = hostArch.String
		stats.OSType = osType.String
		stats.ModelsUsed = modelsUsed.String
		stats.ToolsUsed = toolsUsed.String
		stats.StartTime = time.Unix(startTime, 0)
		stats.LastUpdateTime = time.Unix(lastUpdateTime, 0)
		stats.CreatedAt = time.Unix(createdAt, 0)
		stats.UpdatedAt = time.Unix(updatedAt, 0)

		sessions = append(sessions, &stats)
	}

	return sessions, rows.Err()
}

// GetOrgSessionStats retrieves all sessions for an organization
func (s *Store) GetOrgSessionStats(orgID string, limit int) ([]*SessionStats, error) {
	query := `
	SELECT session_id, user_id, organization_id, service_name,
		start_time, last_update_time,
		terminal_type, host_arch, os_type,
		total_cost_usd, total_input_tokens, total_output_tokens,
		total_cache_read_tokens, total_cache_creation_tokens, total_active_time_seconds,
		api_request_count, user_prompt_count, tool_execution_count,
		tool_success_count, tool_failure_count,
		avg_api_latency_ms, total_api_latency_ms,
		models_used, tools_used,
		created_at, updated_at
	FROM session_stats WHERE organization_id = ?
	ORDER BY start_time DESC
	LIMIT ?
	`

	rows, err := s.db.Query(query, orgID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []*SessionStats
	for rows.Next() {
		var stats SessionStats
		var startTime, lastUpdateTime, createdAt, updatedAt int64
		var serviceName, terminalType, hostArch, osType sql.NullString
		var modelsUsed, toolsUsed sql.NullString

		err := rows.Scan(
			&stats.SessionID, &stats.UserID, &stats.OrganizationID, &serviceName,
			&startTime, &lastUpdateTime,
			&terminalType, &hostArch, &osType,
			&stats.TotalCostUSD, &stats.TotalInputTokens, &stats.TotalOutputTokens,
			&stats.TotalCacheReadTokens, &stats.TotalCacheCreationTokens, &stats.TotalActiveTimeSeconds,
			&stats.APIRequestCount, &stats.UserPromptCount, &stats.ToolExecutionCount,
			&stats.ToolSuccessCount, &stats.ToolFailureCount,
			&stats.AvgAPILatencyMS, &stats.TotalAPILatencyMS,
			&modelsUsed, &toolsUsed,
			&createdAt, &updatedAt,
		)

		if err != nil {
			return nil, err
		}

		stats.ServiceName = serviceName.String
		stats.TerminalType = terminalType.String
		stats.HostArch = hostArch.String
		stats.OSType = osType.String
		stats.ModelsUsed = modelsUsed.String
		stats.ToolsUsed = toolsUsed.String
		stats.StartTime = time.Unix(startTime, 0)
		stats.LastUpdateTime = time.Unix(lastUpdateTime, 0)
		stats.CreatedAt = time.Unix(createdAt, 0)
		stats.UpdatedAt = time.Unix(updatedAt, 0)

		sessions = append(sessions, &stats)
	}

	return sessions, rows.Err()
}

// GetSessionModelStats retrieves per-model statistics for a specific session
func (s *Store) GetSessionModelStats(sessionID string) ([]*SessionModelStats, error) {
	query := `
	SELECT session_id, model, cost_usd, input_tokens, output_tokens,
		cache_read_tokens, cache_creation_tokens, request_count,
		total_latency_ms, avg_latency_ms
	FROM session_model_stats
	WHERE session_id = ?
	ORDER BY cost_usd DESC
	`

	rows, err := s.db.Query(query, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var modelStats []*SessionModelStats
	for rows.Next() {
		var stats SessionModelStats
		err := rows.Scan(
			&stats.SessionID, &stats.Model, &stats.CostUSD,
			&stats.InputTokens, &stats.OutputTokens,
			&stats.CacheReadTokens, &stats.CacheCreationTokens,
			&stats.RequestCount, &stats.TotalLatencyMS, &stats.AvgLatencyMS,
		)
		if err != nil {
			return nil, err
		}
		modelStats = append(modelStats, &stats)
	}

	return modelStats, rows.Err()
}

// GetSessionToolStats retrieves per-tool statistics for a specific session
func (s *Store) GetSessionToolStats(sessionID string) ([]*SessionToolStats, error) {
	query := `
	SELECT session_id, tool_name, execution_count, success_count, failure_count,
		total_duration_ms, avg_duration_ms, min_duration_ms, max_duration_ms
	FROM session_tool_stats
	WHERE session_id = ?
	ORDER BY execution_count DESC
	`

	rows, err := s.db.Query(query, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var toolStats []*SessionToolStats
	for rows.Next() {
		var stats SessionToolStats
		err := rows.Scan(
			&stats.SessionID, &stats.ToolName,
			&stats.ExecutionCount, &stats.SuccessCount, &stats.FailureCount,
			&stats.TotalDurationMS, &stats.AvgDurationMS,
			&stats.MinDurationMS, &stats.MaxDurationMS,
		)
		if err != nil {
			return nil, err
		}
		toolStats = append(toolStats, &stats)
	}

	return toolStats, rows.Err()
}

// ModelAggregates represents aggregated statistics for a model across all sessions
type ModelAggregates struct {
	Model                    string
	TotalSessions            int
	TotalCostUSD             float64
	TotalRequests            int
	TotalInputTokens         int64
	TotalOutputTokens        int64
	TotalCacheReadTokens     int64
	TotalCacheCreationTokens int64
	AvgCostPerSession        float64
	AvgLatencyMS             float64
}

// GetAllModelStats retrieves aggregated statistics across all models
func (s *Store) GetAllModelStats(limit int) ([]*ModelAggregates, error) {
	query := `
	SELECT
		model,
		COUNT(DISTINCT session_id) as total_sessions,
		SUM(cost_usd) as total_cost,
		SUM(request_count) as total_requests,
		SUM(input_tokens) as total_input_tokens,
		SUM(output_tokens) as total_output_tokens,
		SUM(cache_read_tokens) as total_cache_read_tokens,
		SUM(cache_creation_tokens) as total_cache_creation_tokens,
		AVG(cost_usd) as avg_cost_per_session,
		AVG(avg_latency_ms) as avg_latency_ms
	FROM session_model_stats
	GROUP BY model
	ORDER BY total_cost DESC
	LIMIT ?
	`

	rows, err := s.db.Query(query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var aggregates []*ModelAggregates
	for rows.Next() {
		var agg ModelAggregates
		err := rows.Scan(
			&agg.Model, &agg.TotalSessions, &agg.TotalCostUSD,
			&agg.TotalRequests, &agg.TotalInputTokens, &agg.TotalOutputTokens,
			&agg.TotalCacheReadTokens, &agg.TotalCacheCreationTokens,
			&agg.AvgCostPerSession, &agg.AvgLatencyMS,
		)
		if err != nil {
			return nil, err
		}
		aggregates = append(aggregates, &agg)
	}

	return aggregates, rows.Err()
}

// ToolAggregates represents aggregated statistics for a tool across all sessions
type ToolAggregates struct {
	ToolName        string
	TotalExecutions int
	TotalSuccesses  int
	TotalFailures   int
	SuccessRate     float64
	AvgDurationMS   float64
	SessionsUsedIn  int
}

// GetAllToolStats retrieves aggregated statistics across all tools
func (s *Store) GetAllToolStats(limit int) ([]*ToolAggregates, error) {
	query := `
	SELECT
		tool_name,
		SUM(execution_count) as total_executions,
		SUM(success_count) as total_successes,
		SUM(failure_count) as total_failures,
		CAST(SUM(success_count) AS REAL) / CAST(SUM(execution_count) AS REAL) as success_rate,
		AVG(avg_duration_ms) as avg_duration_ms,
		COUNT(DISTINCT session_id) as sessions_used_in
	FROM session_tool_stats
	GROUP BY tool_name
	ORDER BY total_executions DESC
	LIMIT ?
	`

	rows, err := s.db.Query(query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var aggregates []*ToolAggregates
	for rows.Next() {
		var agg ToolAggregates
		err := rows.Scan(
			&agg.ToolName, &agg.TotalExecutions,
			&agg.TotalSuccesses, &agg.TotalFailures,
			&agg.SuccessRate, &agg.AvgDurationMS,
			&agg.SessionsUsedIn,
		)
		if err != nil {
			return nil, err
		}
		aggregates = append(aggregates, &agg)
	}

	return aggregates, rows.Err()
}
