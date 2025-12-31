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

	if state.LastProcessedLine != 0 {
		t.Errorf("Expected initial line 0, got %d", state.LastProcessedLine)
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

	if updated.LastProcessedLine != 42 {
		t.Errorf("Expected line 42, got %d", updated.LastProcessedLine)
	}
	if updated.FileSizeBytes != 1024 {
		t.Errorf("Expected size 1024, got %d", updated.FileSizeBytes)
	}
}
