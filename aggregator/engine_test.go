package aggregator

import (
	"os"
	"testing"
	"time"
)

func TestEngineProcessMetric(t *testing.T) {
	dbPath := "./test_engine.db"
	defer os.Remove(dbPath)

	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	engine := NewEngine(store)

	// Test processing cost metric
	costRecord := &MetricRecord{
		Timestamp:      time.Now(),
		SessionID:      "session-123",
		UserID:         "user-456",
		OrganizationID: "org-789",
		ServiceName:    "test-service",
		MetricName:     "claude_code.cost.usage",
		MetricValue:    1.25,
		Attributes: map[string]string{
			"model": "claude-3-5-sonnet",
		},
	}

	engine.ProcessMetric(costRecord)

	// Verify session was created in cache
	engine.cacheMutex.RLock()
	session, exists := engine.sessionCache["session-123"]
	engine.cacheMutex.RUnlock()

	if !exists {
		t.Fatal("Session not found in cache")
	}

	if session.TotalCostUSD != 1.25 {
		t.Errorf("Expected cost 1.25, got %f", session.TotalCostUSD)
	}

	// Test processing token metric
	tokenRecord := &MetricRecord{
		Timestamp:      time.Now(),
		SessionID:      "session-123",
		UserID:         "user-456",
		OrganizationID: "org-789",
		MetricName:     "claude_code.token.usage",
		MetricValue:    int64(1000),
		Attributes: map[string]string{
			"type": "input",
		},
	}

	engine.ProcessMetric(tokenRecord)

	engine.cacheMutex.RLock()
	session = engine.sessionCache["session-123"]
	engine.cacheMutex.RUnlock()

	if session.TotalInputTokens != 1000 {
		t.Errorf("Expected 1000 input tokens, got %d", session.TotalInputTokens)
	}

	// Test multiple token types
	outputTokenRecord := &MetricRecord{
		Timestamp:   time.Now(),
		SessionID:   "session-123",
		MetricName:  "claude_code.token.usage",
		MetricValue: int64(500),
		Attributes: map[string]string{
			"type": "output",
		},
	}

	engine.ProcessMetric(outputTokenRecord)

	engine.cacheMutex.RLock()
	session = engine.sessionCache["session-123"]
	engine.cacheMutex.RUnlock()

	if session.TotalOutputTokens != 500 {
		t.Errorf("Expected 500 output tokens, got %d", session.TotalOutputTokens)
	}
}

func TestEngineProcessLog(t *testing.T) {
	dbPath := "./test_engine_log.db"
	defer os.Remove(dbPath)

	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	engine := NewEngine(store)

	// Test API request log
	apiRecord := &LogRecord{
		Timestamp:      time.Now(),
		SessionID:      "session-456",
		UserID:         "user-789",
		OrganizationID: "org-123",
		Body:           "claude_code.api_request",
		Attributes: map[string]interface{}{
			"duration_ms": map[string]interface{}{
				"doubleValue": 123.45,
			},
		},
	}

	engine.ProcessLog(apiRecord)

	engine.cacheMutex.RLock()
	session := engine.sessionCache["session-456"]
	engine.cacheMutex.RUnlock()

	if session.APIRequestCount != 1 {
		t.Errorf("Expected 1 API request, got %d", session.APIRequestCount)
	}

	if session.TotalAPILatencyMS != 123.45 {
		t.Errorf("Expected latency 123.45ms, got %f", session.TotalAPILatencyMS)
	}

	// Test user prompt log
	promptRecord := &LogRecord{
		Timestamp: time.Now(),
		SessionID: "session-456",
		Body:      "claude_code.user_prompt",
	}

	engine.ProcessLog(promptRecord)

	engine.cacheMutex.RLock()
	session = engine.sessionCache["session-456"]
	engine.cacheMutex.RUnlock()

	if session.UserPromptCount != 1 {
		t.Errorf("Expected 1 user prompt, got %d", session.UserPromptCount)
	}

	// Test tool result log
	toolRecord := &LogRecord{
		Timestamp: time.Now(),
		SessionID: "session-456",
		Body:      "claude_code.tool_result",
		Attributes: map[string]interface{}{
			"success": map[string]interface{}{
				"boolValue": true,
			},
			"tool_name": map[string]interface{}{
				"stringValue": "Read",
			},
		},
	}

	engine.ProcessLog(toolRecord)

	engine.cacheMutex.RLock()
	session = engine.sessionCache["session-456"]
	engine.cacheMutex.RUnlock()

	if session.ToolExecutionCount != 1 {
		t.Errorf("Expected 1 tool execution, got %d", session.ToolExecutionCount)
	}

	if session.ToolSuccessCount != 1 {
		t.Errorf("Expected 1 successful tool, got %d", session.ToolSuccessCount)
	}
}

func TestEngineFlushCache(t *testing.T) {
	dbPath := "./test_engine_flush.db"
	defer os.Remove(dbPath)

	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	engine := NewEngine(store)

	// Add some data to cache
	record := &MetricRecord{
		Timestamp:      time.Now(),
		SessionID:      "flush-test",
		UserID:         "user-123",
		OrganizationID: "org-456",
		MetricName:     "claude_code.cost.usage",
		MetricValue:    5.0,
	}

	engine.ProcessMetric(record)

	// Flush cache
	engine.FlushCache()

	// Verify data was written to database
	stats, err := store.GetSessionStats("flush-test")
	if err != nil {
		t.Fatalf("Failed to retrieve flushed stats: %v", err)
	}

	if stats.TotalCostUSD != 5.0 {
		t.Errorf("Expected flushed cost 5.0, got %f", stats.TotalCostUSD)
	}
}
