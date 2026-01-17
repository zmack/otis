package aggregator

import (
	"fmt"
	"os"
	"testing"
	"time"
)

func TestStoreInitialization(t *testing.T) {
	// Create temporary database
	dbPath := "./test_otis.db"
	defer os.Remove(dbPath)

	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Verify tables were created
	var count int
	err = store.db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='session_stats'").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query session_stats table: %v", err)
	}
	if count != 1 {
		t.Errorf("Expected session_stats table to exist, got count: %d", count)
	}
}

func TestSessionStatsUpsert(t *testing.T) {
	dbPath := "./test_otis.db"
	defer os.Remove(dbPath)

	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Create test session stats
	stats := &SessionStats{
		SessionID:        "test-session-123",
		UserID:           "user-456",
		OrganizationID:   "org-789",
		ServiceName:      "test-service",
		StartTime:        time.Now(),
		LastUpdateTime:   time.Now(),
		TotalCostUSD:     1.23,
		TotalInputTokens: 1000,
		TotalOutputTokens: 500,
		APIRequestCount:  5,
		ModelsUsed:       `["claude-3-5-sonnet"]`,
		ToolsUsed:        `{"Read": 3, "Write": 2}`,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	// Insert
	err = store.UpsertSessionStats(stats)
	if err != nil {
		t.Fatalf("Failed to insert session stats: %v", err)
	}

	// Retrieve
	retrieved, err := store.GetSessionStats("test-session-123")
	if err != nil {
		t.Fatalf("Failed to retrieve session stats: %v", err)
	}

	// Verify
	if retrieved.SessionID != stats.SessionID {
		t.Errorf("Expected session_id %s, got %s", stats.SessionID, retrieved.SessionID)
	}
	if retrieved.UserID != stats.UserID {
		t.Errorf("Expected user_id %s, got %s", stats.UserID, retrieved.UserID)
	}
	if retrieved.TotalCostUSD != stats.TotalCostUSD {
		t.Errorf("Expected cost %f, got %f", stats.TotalCostUSD, retrieved.TotalCostUSD)
	}
	if retrieved.TotalInputTokens != stats.TotalInputTokens {
		t.Errorf("Expected input tokens %d, got %d", stats.TotalInputTokens, retrieved.TotalInputTokens)
	}

	// Update
	stats.TotalCostUSD = 2.46
	stats.APIRequestCount = 10
	stats.UpdatedAt = time.Now()

	err = store.UpsertSessionStats(stats)
	if err != nil {
		t.Fatalf("Failed to update session stats: %v", err)
	}

	// Retrieve updated
	updated, err := store.GetSessionStats("test-session-123")
	if err != nil {
		t.Fatalf("Failed to retrieve updated session stats: %v", err)
	}

	if updated.TotalCostUSD != 2.46 {
		t.Errorf("Expected updated cost 2.46, got %f", updated.TotalCostUSD)
	}
	if updated.APIRequestCount != 10 {
		t.Errorf("Expected updated API count 10, got %d", updated.APIRequestCount)
	}
}

func TestGetUserSessionStats(t *testing.T) {
	dbPath := "./test_otis.db"
	defer os.Remove(dbPath)

	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Insert multiple sessions for same user
	userID := "test-user-123"
	for i := 0; i < 5; i++ {
		stats := &SessionStats{
			SessionID:        fmt.Sprintf("session-%d", i),
			UserID:           userID,
			OrganizationID:   "org-789",
			StartTime:        time.Now().Add(time.Duration(i) * time.Hour),
			LastUpdateTime:   time.Now(),
			TotalCostUSD:     float64(i) * 0.5,
			TotalInputTokens: int64(i * 100),
			CreatedAt:        time.Now(),
			UpdatedAt:        time.Now(),
			ModelsUsed:       "[]",
			ToolsUsed:        "{}",
		}
		if err := store.UpsertSessionStats(stats); err != nil {
			t.Fatalf("Failed to insert session %d: %v", i, err)
		}
	}

	// Retrieve user sessions
	sessions, err := store.GetUserSessionStats(userID, 10)
	if err != nil {
		t.Fatalf("Failed to get user sessions: %v", err)
	}

	if len(sessions) != 5 {
		t.Errorf("Expected 5 sessions, got %d", len(sessions))
	}

	// Verify they're all for the correct user
	for _, session := range sessions {
		if session.UserID != userID {
			t.Errorf("Expected user_id %s, got %s", userID, session.UserID)
		}
	}

	// Test limit
	limited, err := store.GetUserSessionStats(userID, 3)
	if err != nil {
		t.Fatalf("Failed to get limited user sessions: %v", err)
	}

	if len(limited) != 3 {
		t.Errorf("Expected 3 sessions with limit, got %d", len(limited))
	}
}

func TestProcessingState(t *testing.T) {
	dbPath := "./test_otis.db"
	defer os.Remove(dbPath)

	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Get initial state (should be empty)
	state, err := store.GetProcessingState("test.jsonl")
	if err != nil {
		t.Fatalf("Failed to get processing state: %v", err)
	}

	if state.LastByteOffset != 0 {
		t.Errorf("Expected initial byte offset 0, got %d", state.LastByteOffset)
	}

	// Update state
	err = store.UpdateProcessingState("test.jsonl", 42, 1024, 12345)
	if err != nil {
		t.Fatalf("Failed to update processing state: %v", err)
	}

	// Retrieve updated state
	updated, err := store.GetProcessingState("test.jsonl")
	if err != nil {
		t.Fatalf("Failed to get updated processing state: %v", err)
	}

	if updated.LastByteOffset != 42 {
		t.Errorf("Expected byte offset 42, got %d", updated.LastByteOffset)
	}
	if updated.FileSizeBytes != 1024 {
		t.Errorf("Expected size 1024, got %d", updated.FileSizeBytes)
	}
	if updated.Inode != 12345 {
		t.Errorf("Expected inode 12345, got %d", updated.Inode)
	}
}

func TestSessionModelStatsUpsert(t *testing.T) {
	dbPath := "./test_model_stats.db"
	defer os.Remove(dbPath)

	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Create test model stats
	modelStats := &SessionModelStats{
		SessionID:           "session-123",
		Model:               "claude-sonnet-4-5",
		CostUSD:             0.0042,
		InputTokens:         1500,
		OutputTokens:        800,
		CacheReadTokens:     500,
		CacheCreationTokens: 200,
		RequestCount:        5,
		TotalLatencyMS:      6252.5,
		AvgLatencyMS:        1250.5,
	}

	// Insert
	err = store.UpsertSessionModelStats(modelStats)
	if err != nil {
		t.Fatalf("Failed to insert model stats: %v", err)
	}

	// Verify by querying database directly
	var retrieved SessionModelStats
	err = store.db.QueryRow(`
		SELECT session_id, model, cost_usd, input_tokens, output_tokens,
			cache_read_tokens, cache_creation_tokens, request_count,
			total_latency_ms, avg_latency_ms
		FROM session_model_stats
		WHERE session_id = ? AND model = ?
	`, "session-123", "claude-sonnet-4-5").Scan(
		&retrieved.SessionID, &retrieved.Model, &retrieved.CostUSD,
		&retrieved.InputTokens, &retrieved.OutputTokens,
		&retrieved.CacheReadTokens, &retrieved.CacheCreationTokens,
		&retrieved.RequestCount, &retrieved.TotalLatencyMS, &retrieved.AvgLatencyMS,
	)
	if err != nil {
		t.Fatalf("Failed to retrieve model stats: %v", err)
	}

	// Verify values
	if retrieved.CostUSD != modelStats.CostUSD {
		t.Errorf("Expected cost %f, got %f", modelStats.CostUSD, retrieved.CostUSD)
	}
	if retrieved.InputTokens != modelStats.InputTokens {
		t.Errorf("Expected input tokens %d, got %d", modelStats.InputTokens, retrieved.InputTokens)
	}
	if retrieved.RequestCount != modelStats.RequestCount {
		t.Errorf("Expected request count %d, got %d", modelStats.RequestCount, retrieved.RequestCount)
	}

	// Update
	modelStats.CostUSD = 0.0084
	modelStats.RequestCount = 10
	modelStats.AvgLatencyMS = 1100.0

	err = store.UpsertSessionModelStats(modelStats)
	if err != nil {
		t.Fatalf("Failed to update model stats: %v", err)
	}

	// Retrieve updated
	err = store.db.QueryRow(`
		SELECT cost_usd, request_count, avg_latency_ms
		FROM session_model_stats
		WHERE session_id = ? AND model = ?
	`, "session-123", "claude-sonnet-4-5").Scan(
		&retrieved.CostUSD, &retrieved.RequestCount, &retrieved.AvgLatencyMS,
	)
	if err != nil {
		t.Fatalf("Failed to retrieve updated model stats: %v", err)
	}

	if retrieved.CostUSD != 0.0084 {
		t.Errorf("Expected updated cost 0.0084, got %f", retrieved.CostUSD)
	}
	if retrieved.RequestCount != 10 {
		t.Errorf("Expected updated request count 10, got %d", retrieved.RequestCount)
	}
}

func TestSessionToolStatsUpsert(t *testing.T) {
	dbPath := "./test_tool_stats.db"
	defer os.Remove(dbPath)

	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Create test tool stats
	toolStats := &SessionToolStats{
		SessionID:       "session-456",
		ToolName:        "Read",
		ExecutionCount:  12,
		SuccessCount:    12,
		FailureCount:    0,
		TotalDurationMS: 542.4,
		AvgDurationMS:   45.2,
		MinDurationMS:   12.3,
		MaxDurationMS:   120.8,
	}

	// Insert
	err = store.UpsertSessionToolStats(toolStats)
	if err != nil {
		t.Fatalf("Failed to insert tool stats: %v", err)
	}

	// Verify by querying database directly
	var retrieved SessionToolStats
	err = store.db.QueryRow(`
		SELECT session_id, tool_name, execution_count, success_count, failure_count,
			total_duration_ms, avg_duration_ms, min_duration_ms, max_duration_ms
		FROM session_tool_stats
		WHERE session_id = ? AND tool_name = ?
	`, "session-456", "Read").Scan(
		&retrieved.SessionID, &retrieved.ToolName,
		&retrieved.ExecutionCount, &retrieved.SuccessCount, &retrieved.FailureCount,
		&retrieved.TotalDurationMS, &retrieved.AvgDurationMS,
		&retrieved.MinDurationMS, &retrieved.MaxDurationMS,
	)
	if err != nil {
		t.Fatalf("Failed to retrieve tool stats: %v", err)
	}

	// Verify values
	if retrieved.ExecutionCount != toolStats.ExecutionCount {
		t.Errorf("Expected execution count %d, got %d", toolStats.ExecutionCount, retrieved.ExecutionCount)
	}
	if retrieved.SuccessCount != toolStats.SuccessCount {
		t.Errorf("Expected success count %d, got %d", toolStats.SuccessCount, retrieved.SuccessCount)
	}
	if retrieved.AvgDurationMS != toolStats.AvgDurationMS {
		t.Errorf("Expected avg duration %f, got %f", toolStats.AvgDurationMS, retrieved.AvgDurationMS)
	}

	// Update
	toolStats.ExecutionCount = 15
	toolStats.SuccessCount = 14
	toolStats.FailureCount = 1
	toolStats.AvgDurationMS = 50.0

	err = store.UpsertSessionToolStats(toolStats)
	if err != nil {
		t.Fatalf("Failed to update tool stats: %v", err)
	}

	// Retrieve updated
	err = store.db.QueryRow(`
		SELECT execution_count, success_count, failure_count, avg_duration_ms
		FROM session_tool_stats
		WHERE session_id = ? AND tool_name = ?
	`, "session-456", "Read").Scan(
		&retrieved.ExecutionCount, &retrieved.SuccessCount,
		&retrieved.FailureCount, &retrieved.AvgDurationMS,
	)
	if err != nil {
		t.Fatalf("Failed to retrieve updated tool stats: %v", err)
	}

	if retrieved.ExecutionCount != 15 {
		t.Errorf("Expected updated execution count 15, got %d", retrieved.ExecutionCount)
	}
	if retrieved.FailureCount != 1 {
		t.Errorf("Expected updated failure count 1, got %d", retrieved.FailureCount)
	}
}

// Tests for new schema (sessions and session_tools tables)

func TestSessionUpsert(t *testing.T) {
	dbPath := "./test_sessions.db"
	defer os.Remove(dbPath)

	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Create test session
	session := &Session{
		SessionID:                "test-session-v2-123",
		OrganizationID:           "org-789",
		UserID:                   "user-456",
		StartTime:                time.Now().Add(-time.Hour),
		EndTime:                  time.Now(),
		TotalCostUSD:             1.23,
		TotalInputTokens:         1000,
		TotalOutputTokens:        500,
		TotalCacheReadTokens:     200,
		TotalCacheCreationTokens: 100,
		ToolCallCount:            15,
		CreatedAt:                time.Now(),
		UpdatedAt:                time.Now(),
	}

	// Insert
	err = store.UpsertSession(session)
	if err != nil {
		t.Fatalf("Failed to insert session: %v", err)
	}

	// Retrieve
	retrieved, err := store.GetSession("test-session-v2-123")
	if err != nil {
		t.Fatalf("Failed to retrieve session: %v", err)
	}

	// Verify
	if retrieved.SessionID != session.SessionID {
		t.Errorf("Expected session_id %s, got %s", session.SessionID, retrieved.SessionID)
	}
	if retrieved.OrganizationID != session.OrganizationID {
		t.Errorf("Expected organization_id %s, got %s", session.OrganizationID, retrieved.OrganizationID)
	}
	if retrieved.UserID != session.UserID {
		t.Errorf("Expected user_id %s, got %s", session.UserID, retrieved.UserID)
	}
	if retrieved.TotalCostUSD != session.TotalCostUSD {
		t.Errorf("Expected cost %f, got %f", session.TotalCostUSD, retrieved.TotalCostUSD)
	}
	if retrieved.TotalInputTokens != session.TotalInputTokens {
		t.Errorf("Expected input tokens %d, got %d", session.TotalInputTokens, retrieved.TotalInputTokens)
	}
	if retrieved.ToolCallCount != session.ToolCallCount {
		t.Errorf("Expected tool call count %d, got %d", session.ToolCallCount, retrieved.ToolCallCount)
	}

	// Update
	session.TotalCostUSD = 2.46
	session.ToolCallCount = 25
	session.UpdatedAt = time.Now()

	err = store.UpsertSession(session)
	if err != nil {
		t.Fatalf("Failed to update session: %v", err)
	}

	// Retrieve updated
	updated, err := store.GetSession("test-session-v2-123")
	if err != nil {
		t.Fatalf("Failed to retrieve updated session: %v", err)
	}

	if updated.TotalCostUSD != 2.46 {
		t.Errorf("Expected updated cost 2.46, got %f", updated.TotalCostUSD)
	}
	if updated.ToolCallCount != 25 {
		t.Errorf("Expected updated tool call count 25, got %d", updated.ToolCallCount)
	}
}

func TestSessionToolUpsert(t *testing.T) {
	dbPath := "./test_session_tools.db"
	defer os.Remove(dbPath)

	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// First create a session (foreign key constraint)
	session := &Session{
		SessionID:      "session-for-tools",
		OrganizationID: "org-123",
		UserID:         "user-456",
		StartTime:      time.Now(),
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
	err = store.UpsertSession(session)
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	// Create test session tool
	tool := &SessionTool{
		SessionID:            "session-for-tools",
		ToolName:             "Read",
		CallCount:            12,
		SuccessCount:         10,
		FailureCount:         2,
		TotalExecutionTimeMS: 542.4,
	}

	// Insert
	err = store.UpsertSessionTool(tool)
	if err != nil {
		t.Fatalf("Failed to insert session tool: %v", err)
	}

	// Retrieve
	tools, err := store.GetSessionTools("session-for-tools")
	if err != nil {
		t.Fatalf("Failed to retrieve session tools: %v", err)
	}

	if len(tools) != 1 {
		t.Fatalf("Expected 1 tool, got %d", len(tools))
	}

	retrieved := tools[0]
	if retrieved.ToolName != tool.ToolName {
		t.Errorf("Expected tool_name %s, got %s", tool.ToolName, retrieved.ToolName)
	}
	if retrieved.CallCount != tool.CallCount {
		t.Errorf("Expected call_count %d, got %d", tool.CallCount, retrieved.CallCount)
	}
	if retrieved.SuccessCount != tool.SuccessCount {
		t.Errorf("Expected success_count %d, got %d", tool.SuccessCount, retrieved.SuccessCount)
	}
	if retrieved.FailureCount != tool.FailureCount {
		t.Errorf("Expected failure_count %d, got %d", tool.FailureCount, retrieved.FailureCount)
	}

	// Update
	tool.CallCount = 20
	tool.SuccessCount = 18
	tool.FailureCount = 2

	err = store.UpsertSessionTool(tool)
	if err != nil {
		t.Fatalf("Failed to update session tool: %v", err)
	}

	// Retrieve updated
	tools, err = store.GetSessionTools("session-for-tools")
	if err != nil {
		t.Fatalf("Failed to retrieve updated session tools: %v", err)
	}

	if tools[0].CallCount != 20 {
		t.Errorf("Expected updated call_count 20, got %d", tools[0].CallCount)
	}
	if tools[0].SuccessCount != 18 {
		t.Errorf("Expected updated success_count 18, got %d", tools[0].SuccessCount)
	}
}

func TestSessionToolDecisionTracking(t *testing.T) {
	dbPath := "./test_session_tools_decisions.db"
	defer os.Remove(dbPath)

	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Create a session first
	session := &Session{
		SessionID:      "session-decisions",
		OrganizationID: "org-123",
		UserID:         "user-456",
		StartTime:      time.Now(),
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
	err = store.UpsertSession(session)
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	// Create tool with decision tracking
	tool := &SessionTool{
		SessionID:            "session-decisions",
		ToolName:             "Bash",
		CallCount:            15,
		SuccessCount:         14,
		FailureCount:         1,
		TotalExecutionTimeMS: 1500.0,
		AutoApprovedCount:    8,
		UserApprovedCount:    6,
		RejectedCount:        1,
		TotalResultSizeBytes: 25000,
	}

	// Insert
	err = store.UpsertSessionTool(tool)
	if err != nil {
		t.Fatalf("Failed to insert session tool: %v", err)
	}

	// Retrieve and verify
	tools, err := store.GetSessionTools("session-decisions")
	if err != nil {
		t.Fatalf("Failed to retrieve session tools: %v", err)
	}

	if len(tools) != 1 {
		t.Fatalf("Expected 1 tool, got %d", len(tools))
	}

	retrieved := tools[0]

	// Verify decision counts
	if retrieved.AutoApprovedCount != 8 {
		t.Errorf("Expected auto_approved_count 8, got %d", retrieved.AutoApprovedCount)
	}
	if retrieved.UserApprovedCount != 6 {
		t.Errorf("Expected user_approved_count 6, got %d", retrieved.UserApprovedCount)
	}
	if retrieved.RejectedCount != 1 {
		t.Errorf("Expected rejected_count 1, got %d", retrieved.RejectedCount)
	}
	if retrieved.TotalResultSizeBytes != 25000 {
		t.Errorf("Expected total_result_size_bytes 25000, got %d", retrieved.TotalResultSizeBytes)
	}

	// Verify counts add up
	totalDecisions := retrieved.AutoApprovedCount + retrieved.UserApprovedCount + retrieved.RejectedCount
	if totalDecisions != retrieved.CallCount {
		t.Errorf("Decision counts (%d) should equal call_count (%d)", totalDecisions, retrieved.CallCount)
	}

	// Update with more calls
	tool.CallCount = 20
	tool.AutoApprovedCount = 12
	tool.UserApprovedCount = 7
	tool.RejectedCount = 1
	tool.TotalResultSizeBytes = 35000

	err = store.UpsertSessionTool(tool)
	if err != nil {
		t.Fatalf("Failed to update session tool: %v", err)
	}

	// Retrieve updated
	tools, err = store.GetSessionTools("session-decisions")
	if err != nil {
		t.Fatalf("Failed to retrieve updated session tools: %v", err)
	}

	if tools[0].AutoApprovedCount != 12 {
		t.Errorf("Expected updated auto_approved_count 12, got %d", tools[0].AutoApprovedCount)
	}
	if tools[0].TotalResultSizeBytes != 35000 {
		t.Errorf("Expected updated total_result_size_bytes 35000, got %d", tools[0].TotalResultSizeBytes)
	}
}

func TestGetSessionsByUser(t *testing.T) {
	dbPath := "./test_sessions_by_user.db"
	defer os.Remove(dbPath)

	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	userID := "test-user-v2"

	// Insert multiple sessions for same user
	for i := 0; i < 5; i++ {
		session := &Session{
			SessionID:        fmt.Sprintf("session-v2-%d", i),
			OrganizationID:   "org-789",
			UserID:           userID,
			StartTime:        time.Now().Add(time.Duration(i) * time.Hour),
			TotalCostUSD:     float64(i) * 0.5,
			TotalInputTokens: int64(i * 100),
			CreatedAt:        time.Now(),
			UpdatedAt:        time.Now(),
		}
		if err := store.UpsertSession(session); err != nil {
			t.Fatalf("Failed to insert session %d: %v", i, err)
		}
	}

	// Retrieve user sessions
	sessions, err := store.GetSessionsByUser(userID, 10)
	if err != nil {
		t.Fatalf("Failed to get user sessions: %v", err)
	}

	if len(sessions) != 5 {
		t.Errorf("Expected 5 sessions, got %d", len(sessions))
	}

	// Verify they're all for the correct user
	for _, session := range sessions {
		if session.UserID != userID {
			t.Errorf("Expected user_id %s, got %s", userID, session.UserID)
		}
	}

	// Test limit
	limited, err := store.GetSessionsByUser(userID, 3)
	if err != nil {
		t.Fatalf("Failed to get limited user sessions: %v", err)
	}

	if len(limited) != 3 {
		t.Errorf("Expected 3 sessions with limit, got %d", len(limited))
	}
}

func TestGetSessionsByOrg(t *testing.T) {
	dbPath := "./test_sessions_by_org.db"
	defer os.Remove(dbPath)

	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	orgID := "test-org-v2"

	// Insert multiple sessions for same org with different users
	for i := 0; i < 5; i++ {
		session := &Session{
			SessionID:        fmt.Sprintf("session-org-%d", i),
			OrganizationID:   orgID,
			UserID:           fmt.Sprintf("user-%d", i),
			StartTime:        time.Now().Add(time.Duration(i) * time.Hour),
			TotalCostUSD:     float64(i) * 0.5,
			TotalInputTokens: int64(i * 100),
			CreatedAt:        time.Now(),
			UpdatedAt:        time.Now(),
		}
		if err := store.UpsertSession(session); err != nil {
			t.Fatalf("Failed to insert session %d: %v", i, err)
		}
	}

	// Retrieve org sessions
	sessions, err := store.GetSessionsByOrg(orgID, 10)
	if err != nil {
		t.Fatalf("Failed to get org sessions: %v", err)
	}

	if len(sessions) != 5 {
		t.Errorf("Expected 5 sessions, got %d", len(sessions))
	}

	// Verify they're all for the correct org
	for _, session := range sessions {
		if session.OrganizationID != orgID {
			t.Errorf("Expected organization_id %s, got %s", orgID, session.OrganizationID)
		}
	}
}

func TestGetToolAggregates(t *testing.T) {
	dbPath := "./test_tool_aggregates.db"
	defer os.Remove(dbPath)

	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Create sessions first
	for i := 0; i < 3; i++ {
		session := &Session{
			SessionID:      fmt.Sprintf("session-agg-%d", i),
			OrganizationID: "org-123",
			UserID:         "user-456",
			StartTime:      time.Now(),
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		}
		if err := store.UpsertSession(session); err != nil {
			t.Fatalf("Failed to create session: %v", err)
		}
	}

	// Insert tools across multiple sessions
	tools := []SessionTool{
		{SessionID: "session-agg-0", ToolName: "Read", CallCount: 10, SuccessCount: 9, FailureCount: 1, TotalExecutionTimeMS: 100},
		{SessionID: "session-agg-0", ToolName: "Write", CallCount: 5, SuccessCount: 5, FailureCount: 0, TotalExecutionTimeMS: 50},
		{SessionID: "session-agg-1", ToolName: "Read", CallCount: 8, SuccessCount: 8, FailureCount: 0, TotalExecutionTimeMS: 80},
		{SessionID: "session-agg-1", ToolName: "Edit", CallCount: 3, SuccessCount: 2, FailureCount: 1, TotalExecutionTimeMS: 30},
		{SessionID: "session-agg-2", ToolName: "Read", CallCount: 12, SuccessCount: 11, FailureCount: 1, TotalExecutionTimeMS: 120},
	}

	for _, tool := range tools {
		if err := store.UpsertSessionTool(&tool); err != nil {
			t.Fatalf("Failed to insert tool: %v", err)
		}
	}

	// Get aggregates
	aggregates, err := store.GetToolAggregates(10)
	if err != nil {
		t.Fatalf("Failed to get tool aggregates: %v", err)
	}

	if len(aggregates) != 3 {
		t.Errorf("Expected 3 tools, got %d", len(aggregates))
	}

	// Find Read tool aggregate (should be first due to ORDER BY total_executions DESC)
	var readAgg *ToolAggregates
	for _, agg := range aggregates {
		if agg.ToolName == "Read" {
			readAgg = agg
			break
		}
	}

	if readAgg == nil {
		t.Fatal("Read tool aggregate not found")
	}

	// Read: 10 + 8 + 12 = 30 executions
	if readAgg.TotalExecutions != 30 {
		t.Errorf("Expected 30 total executions for Read, got %d", readAgg.TotalExecutions)
	}

	// Read: 9 + 8 + 11 = 28 successes
	if readAgg.TotalSuccesses != 28 {
		t.Errorf("Expected 28 total successes for Read, got %d", readAgg.TotalSuccesses)
	}

	// Read used in 3 sessions
	if readAgg.SessionsUsedIn != 3 {
		t.Errorf("Expected Read to be used in 3 sessions, got %d", readAgg.SessionsUsedIn)
	}
}
