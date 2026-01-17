package aggregator

import (
	"database/sql"
	"embed"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/pressly/goose/v3"
)

//go:embed migrations/*.sql
var embedMigrations embed.FS

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
	if err := store.RunMigrations(); err != nil {
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	return store, nil
}

// RunMigrations runs all pending database migrations using goose
func (s *Store) RunMigrations() error {
	// Handle legacy databases that exist but weren't created with goose
	// by applying necessary schema fixes before running migrations
	if err := s.applyLegacyFixes(); err != nil {
		return fmt.Errorf("failed to apply legacy fixes: %w", err)
	}

	goose.SetBaseFS(embedMigrations)

	if err := goose.SetDialect("sqlite3"); err != nil {
		return fmt.Errorf("failed to set dialect: %w", err)
	}

	if err := goose.Up(s.db, "migrations"); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	return nil
}

// applyLegacyFixes handles databases that were created before goose migrations
// were introduced. It marks migration 001 as applied for existing databases
// so that goose can correctly apply only new migrations.
func (s *Store) applyLegacyFixes() error {
	// Check if this is a legacy database (has tables but no goose version table)
	var hasLegacyTables int
	err := s.db.QueryRow(`
		SELECT COUNT(*) FROM sqlite_master
		WHERE type='table' AND name='session_stats'
	`).Scan(&hasLegacyTables)
	if err != nil {
		return err
	}

	var hasGooseTable int
	err = s.db.QueryRow(`
		SELECT COUNT(*) FROM sqlite_master
		WHERE type='table' AND name='goose_db_version'
	`).Scan(&hasGooseTable)
	if err != nil {
		return err
	}

	if hasLegacyTables > 0 && hasGooseTable == 0 {
		// This is a legacy database - create goose table and mark migration 001 as applied
		_, err = s.db.Exec(`
			CREATE TABLE IF NOT EXISTS goose_db_version (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				version_id INTEGER NOT NULL,
				is_applied INTEGER NOT NULL,
				tstamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP
			)
		`)
		if err != nil {
			return fmt.Errorf("failed to create goose version table: %w", err)
		}

		// Mark migration 001 (initial schema) as already applied
		_, err = s.db.Exec(`
			INSERT INTO goose_db_version (version_id, is_applied) VALUES (1, 1)
		`)
		if err != nil {
			return fmt.Errorf("failed to mark migration 001 as applied: %w", err)
		}
	}

	return nil
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
func (s *Store) UpdateProcessingState(fileName string, byteOffset int64, fileSize int64, inode uint64) error {
	query := `
	INSERT INTO processing_state (file_name, last_byte_offset, last_processed_time, file_size_bytes, inode, updated_at)
	VALUES (?, ?, ?, ?, ?, ?)
	ON CONFLICT(file_name) DO UPDATE SET
		last_byte_offset = excluded.last_byte_offset,
		last_processed_time = excluded.last_processed_time,
		file_size_bytes = excluded.file_size_bytes,
		inode = excluded.inode,
		updated_at = excluded.updated_at
	`

	now := time.Now().Unix()
	_, err := s.db.Exec(query, fileName, byteOffset, now, fileSize, inode, now)
	return err
}

// GetProcessingState retrieves the processing state for a file
func (s *Store) GetProcessingState(fileName string) (*ProcessingState, error) {
	query := `
	SELECT file_name, last_byte_offset, last_processed_time, file_size_bytes, COALESCE(inode, 0), updated_at
	FROM processing_state WHERE file_name = ?
	`

	var state ProcessingState
	var lastProcessedTime, updatedAt int64

	err := s.db.QueryRow(query, fileName).Scan(
		&state.FileName, &state.LastByteOffset, &lastProcessedTime,
		&state.FileSizeBytes, &state.Inode, &updatedAt,
	)

	if err == sql.ErrNoRows {
		// Return empty state if not found
		return &ProcessingState{
			FileName:       fileName,
			LastByteOffset: 0,
			Inode:          0,
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

// UpsertSession inserts or updates a session in the new sessions table
func (s *Store) UpsertSession(session *Session) error {
	query := `
	INSERT INTO sessions (
		session_id, organization_id, user_id, start_time, end_time,
		total_cost_usd, total_input_tokens, total_output_tokens,
		total_cache_read_tokens, total_cache_creation_tokens, tool_call_count,
		created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(session_id) DO UPDATE SET
		end_time = excluded.end_time,
		total_cost_usd = excluded.total_cost_usd,
		total_input_tokens = excluded.total_input_tokens,
		total_output_tokens = excluded.total_output_tokens,
		total_cache_read_tokens = excluded.total_cache_read_tokens,
		total_cache_creation_tokens = excluded.total_cache_creation_tokens,
		tool_call_count = excluded.tool_call_count,
		updated_at = excluded.updated_at
	`

	var endTime *int64
	if !session.EndTime.IsZero() {
		t := session.EndTime.Unix()
		endTime = &t
	}

	_, err := s.db.Exec(query,
		session.SessionID, session.OrganizationID, session.UserID,
		session.StartTime.Unix(), endTime,
		session.TotalCostUSD, session.TotalInputTokens, session.TotalOutputTokens,
		session.TotalCacheReadTokens, session.TotalCacheCreationTokens, session.ToolCallCount,
		session.CreatedAt.Unix(), session.UpdatedAt.Unix(),
	)

	return err
}

// UpsertSessionTool inserts or updates tool statistics for a session
func (s *Store) UpsertSessionTool(tool *SessionTool) error {
	query := `
	INSERT INTO session_tools (
		session_id, tool_name, call_count, success_count, failure_count,
		total_execution_time_ms, auto_approved_count, user_approved_count,
		rejected_count, total_result_size_bytes
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(session_id, tool_name) DO UPDATE SET
		call_count = excluded.call_count,
		success_count = excluded.success_count,
		failure_count = excluded.failure_count,
		total_execution_time_ms = excluded.total_execution_time_ms,
		auto_approved_count = excluded.auto_approved_count,
		user_approved_count = excluded.user_approved_count,
		rejected_count = excluded.rejected_count,
		total_result_size_bytes = excluded.total_result_size_bytes
	`

	_, err := s.db.Exec(query,
		tool.SessionID, tool.ToolName, tool.CallCount,
		tool.SuccessCount, tool.FailureCount, tool.TotalExecutionTimeMS,
		tool.AutoApprovedCount, tool.UserApprovedCount,
		tool.RejectedCount, tool.TotalResultSizeBytes,
	)

	return err
}

// GetSession retrieves a session by ID from the new sessions table
func (s *Store) GetSession(sessionID string) (*Session, error) {
	query := `
	SELECT session_id, organization_id, user_id, start_time, end_time,
		total_cost_usd, total_input_tokens, total_output_tokens,
		total_cache_read_tokens, total_cache_creation_tokens, tool_call_count,
		created_at, updated_at
	FROM sessions WHERE session_id = ?
	`

	var session Session
	var startTime, createdAt, updatedAt int64
	var endTime sql.NullInt64

	err := s.db.QueryRow(query, sessionID).Scan(
		&session.SessionID, &session.OrganizationID, &session.UserID,
		&startTime, &endTime,
		&session.TotalCostUSD, &session.TotalInputTokens, &session.TotalOutputTokens,
		&session.TotalCacheReadTokens, &session.TotalCacheCreationTokens, &session.ToolCallCount,
		&createdAt, &updatedAt,
	)

	if err != nil {
		return nil, err
	}

	session.StartTime = time.Unix(startTime, 0)
	if endTime.Valid {
		session.EndTime = time.Unix(endTime.Int64, 0)
	}
	session.CreatedAt = time.Unix(createdAt, 0)
	session.UpdatedAt = time.Unix(updatedAt, 0)

	return &session, nil
}

// GetSessionTools retrieves tool statistics for a session from the new table
func (s *Store) GetSessionTools(sessionID string) ([]*SessionTool, error) {
	query := `
	SELECT session_id, tool_name, call_count, success_count, failure_count,
		total_execution_time_ms, auto_approved_count, user_approved_count,
		rejected_count, total_result_size_bytes
	FROM session_tools
	WHERE session_id = ?
	ORDER BY call_count DESC
	`

	rows, err := s.db.Query(query, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tools []*SessionTool
	for rows.Next() {
		var tool SessionTool
		err := rows.Scan(
			&tool.SessionID, &tool.ToolName, &tool.CallCount,
			&tool.SuccessCount, &tool.FailureCount, &tool.TotalExecutionTimeMS,
			&tool.AutoApprovedCount, &tool.UserApprovedCount,
			&tool.RejectedCount, &tool.TotalResultSizeBytes,
		)
		if err != nil {
			return nil, err
		}
		tools = append(tools, &tool)
	}

	return tools, rows.Err()
}

// GetSessionsByOrg retrieves sessions for an organization
func (s *Store) GetSessionsByOrg(orgID string, limit int) ([]*Session, error) {
	query := `
	SELECT session_id, organization_id, user_id, start_time, end_time,
		total_cost_usd, total_input_tokens, total_output_tokens,
		total_cache_read_tokens, total_cache_creation_tokens, tool_call_count,
		created_at, updated_at
	FROM sessions WHERE organization_id = ?
	ORDER BY start_time DESC
	LIMIT ?
	`

	rows, err := s.db.Query(query, orgID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []*Session
	for rows.Next() {
		var session Session
		var startTime, createdAt, updatedAt int64
		var endTime sql.NullInt64

		err := rows.Scan(
			&session.SessionID, &session.OrganizationID, &session.UserID,
			&startTime, &endTime,
			&session.TotalCostUSD, &session.TotalInputTokens, &session.TotalOutputTokens,
			&session.TotalCacheReadTokens, &session.TotalCacheCreationTokens, &session.ToolCallCount,
			&createdAt, &updatedAt,
		)
		if err != nil {
			return nil, err
		}

		session.StartTime = time.Unix(startTime, 0)
		if endTime.Valid {
			session.EndTime = time.Unix(endTime.Int64, 0)
		}
		session.CreatedAt = time.Unix(createdAt, 0)
		session.UpdatedAt = time.Unix(updatedAt, 0)

		sessions = append(sessions, &session)
	}

	return sessions, rows.Err()
}

// GetSessionsByUser retrieves sessions for a user
func (s *Store) GetSessionsByUser(userID string, limit int) ([]*Session, error) {
	query := `
	SELECT session_id, organization_id, user_id, start_time, end_time,
		total_cost_usd, total_input_tokens, total_output_tokens,
		total_cache_read_tokens, total_cache_creation_tokens, tool_call_count,
		created_at, updated_at
	FROM sessions WHERE user_id = ?
	ORDER BY start_time DESC
	LIMIT ?
	`

	rows, err := s.db.Query(query, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []*Session
	for rows.Next() {
		var session Session
		var startTime, createdAt, updatedAt int64
		var endTime sql.NullInt64

		err := rows.Scan(
			&session.SessionID, &session.OrganizationID, &session.UserID,
			&startTime, &endTime,
			&session.TotalCostUSD, &session.TotalInputTokens, &session.TotalOutputTokens,
			&session.TotalCacheReadTokens, &session.TotalCacheCreationTokens, &session.ToolCallCount,
			&createdAt, &updatedAt,
		)
		if err != nil {
			return nil, err
		}

		session.StartTime = time.Unix(startTime, 0)
		if endTime.Valid {
			session.EndTime = time.Unix(endTime.Int64, 0)
		}
		session.CreatedAt = time.Unix(createdAt, 0)
		session.UpdatedAt = time.Unix(updatedAt, 0)

		sessions = append(sessions, &session)
	}

	return sessions, rows.Err()
}

// GetToolAggregates retrieves aggregated statistics across all tools from the new table
func (s *Store) GetToolAggregates(limit int) ([]*ToolAggregates, error) {
	query := `
	SELECT
		tool_name,
		SUM(call_count) as total_executions,
		SUM(success_count) as total_successes,
		SUM(failure_count) as total_failures,
		CASE WHEN SUM(call_count) > 0
			THEN CAST(SUM(success_count) AS REAL) / CAST(SUM(call_count) AS REAL)
			ELSE 0 END as success_rate,
		CASE WHEN SUM(call_count) > 0
			THEN SUM(total_execution_time_ms) / SUM(call_count)
			ELSE 0 END as avg_duration_ms,
		COUNT(DISTINCT session_id) as sessions_used_in
	FROM session_tools
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
