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
	err = store.UpdateProcessingState("test.jsonl", 42, 1024)
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
