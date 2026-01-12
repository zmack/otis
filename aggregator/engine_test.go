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

func TestEnginePerModelTracking(t *testing.T) {
	dbPath := "./test_engine_model.db"
	defer os.Remove(dbPath)

	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	engine := NewEngine(store)
	sessionID := "model-test-session"

	// Process cost metric with model attribute
	costRecord := &MetricRecord{
		Timestamp:      time.Now(),
		SessionID:      sessionID,
		UserID:         "user-123",
		OrganizationID: "org-456",
		MetricName:     "claude_code.cost.usage",
		MetricValue:    0.0042,
		Attributes: map[string]string{
			"model": "claude-sonnet-4-5",
		},
	}
	engine.ProcessMetric(costRecord)

	// Process token metrics with model attribute
	tokenRecords := []*MetricRecord{
		{
			Timestamp:   time.Now(),
			SessionID:   sessionID,
			MetricName:  "claude_code.token.usage",
			MetricValue: int64(1500),
			Attributes: map[string]string{
				"type":  "input",
				"model": "claude-sonnet-4-5",
			},
		},
		{
			Timestamp:   time.Now(),
			SessionID:   sessionID,
			MetricName:  "claude_code.token.usage",
			MetricValue: int64(800),
			Attributes: map[string]string{
				"type":  "output",
				"model": "claude-sonnet-4-5",
			},
		},
		{
			Timestamp:   time.Now(),
			SessionID:   sessionID,
			MetricName:  "claude_code.token.usage",
			MetricValue: int64(500),
			Attributes: map[string]string{
				"type":  "cacheRead",
				"model": "claude-sonnet-4-5",
			},
		},
	}

	for _, record := range tokenRecords {
		engine.ProcessMetric(record)
	}

	// Verify model stats in cache
	engine.cacheMutex.RLock()
	modelStats, exists := engine.modelStatsCache[sessionID]["claude-sonnet-4-5"]
	engine.cacheMutex.RUnlock()

	if !exists {
		t.Fatal("Model stats not found in cache")
	}

	if modelStats.CostUSD != 0.0042 {
		t.Errorf("Expected model cost 0.0042, got %f", modelStats.CostUSD)
	}
	if modelStats.InputTokens != 1500 {
		t.Errorf("Expected input tokens 1500, got %d", modelStats.InputTokens)
	}
	if modelStats.OutputTokens != 800 {
		t.Errorf("Expected output tokens 800, got %d", modelStats.OutputTokens)
	}
	if modelStats.CacheReadTokens != 500 {
		t.Errorf("Expected cache read tokens 500, got %d", modelStats.CacheReadTokens)
	}
	if modelStats.RequestCount != 1 {
		t.Errorf("Expected request count 1, got %d", modelStats.RequestCount)
	}

	// Flush and verify it was written to database
	engine.FlushCache()

	var retrievedModelStats SessionModelStats
	err = store.db.QueryRow(`
		SELECT session_id, model, cost_usd, input_tokens, output_tokens, cache_read_tokens, request_count
		FROM session_model_stats
		WHERE session_id = ? AND model = ?
	`, sessionID, "claude-sonnet-4-5").Scan(
		&retrievedModelStats.SessionID, &retrievedModelStats.Model,
		&retrievedModelStats.CostUSD, &retrievedModelStats.InputTokens,
		&retrievedModelStats.OutputTokens, &retrievedModelStats.CacheReadTokens,
		&retrievedModelStats.RequestCount,
	)
	if err != nil {
		t.Fatalf("Failed to retrieve flushed model stats: %v", err)
	}

	if retrievedModelStats.CostUSD != 0.0042 {
		t.Errorf("Expected flushed model cost 0.0042, got %f", retrievedModelStats.CostUSD)
	}
	if retrievedModelStats.InputTokens != 1500 {
		t.Errorf("Expected flushed input tokens 1500, got %d", retrievedModelStats.InputTokens)
	}
}

func TestEnginePerToolTracking(t *testing.T) {
	dbPath := "./test_engine_tool.db"
	defer os.Remove(dbPath)

	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	engine := NewEngine(store)
	sessionID := "tool-test-session"

	// Process multiple tool result logs
	toolRecords := []*LogRecord{
		{
			Timestamp: time.Now(),
			SessionID: sessionID,
			UserID:    "user-123",
			Body:      "claude_code.tool_result",
			Attributes: map[string]interface{}{
				"success": map[string]interface{}{
					"boolValue": true,
				},
				"tool_name": map[string]interface{}{
					"stringValue": "Read",
				},
				"duration_ms": map[string]interface{}{
					"doubleValue": 45.2,
				},
			},
		},
		{
			Timestamp: time.Now(),
			SessionID: sessionID,
			Body:      "claude_code.tool_result",
			Attributes: map[string]interface{}{
				"success": map[string]interface{}{
					"boolValue": true,
				},
				"tool_name": map[string]interface{}{
					"stringValue": "Read",
				},
				"duration_ms": map[string]interface{}{
					"doubleValue": 120.8,
				},
			},
		},
		{
			Timestamp: time.Now(),
			SessionID: sessionID,
			Body:      "claude_code.tool_result",
			Attributes: map[string]interface{}{
				"success": map[string]interface{}{
					"boolValue": false,
				},
				"tool_name": map[string]interface{}{
					"stringValue": "Read",
				},
				"duration_ms": map[string]interface{}{
					"doubleValue": 12.3,
				},
			},
		},
	}

	for _, record := range toolRecords {
		engine.ProcessLog(record)
	}

	// Verify tool stats in cache
	engine.cacheMutex.RLock()
	toolStats, exists := engine.toolStatsCache[sessionID]["Read"]
	engine.cacheMutex.RUnlock()

	if !exists {
		t.Fatal("Tool stats not found in cache")
	}

	if toolStats.ExecutionCount != 3 {
		t.Errorf("Expected execution count 3, got %d", toolStats.ExecutionCount)
	}
	if toolStats.SuccessCount != 2 {
		t.Errorf("Expected success count 2, got %d", toolStats.SuccessCount)
	}
	if toolStats.FailureCount != 1 {
		t.Errorf("Expected failure count 1, got %d", toolStats.FailureCount)
	}
	if toolStats.MinDurationMS != 12.3 {
		t.Errorf("Expected min duration 12.3, got %f", toolStats.MinDurationMS)
	}
	if toolStats.MaxDurationMS != 120.8 {
		t.Errorf("Expected max duration 120.8, got %f", toolStats.MaxDurationMS)
	}

	expectedAvg := (45.2 + 120.8 + 12.3) / 3.0
	// Use approximate comparison for floating point
	if diff := toolStats.AvgDurationMS - expectedAvg; diff < -0.001 || diff > 0.001 {
		t.Errorf("Expected avg duration %f, got %f", expectedAvg, toolStats.AvgDurationMS)
	}

	// Flush and verify it was written to database
	engine.FlushCache()

	var retrievedToolStats SessionToolStats
	err = store.db.QueryRow(`
		SELECT session_id, tool_name, execution_count, success_count, failure_count,
			avg_duration_ms, min_duration_ms, max_duration_ms
		FROM session_tool_stats
		WHERE session_id = ? AND tool_name = ?
	`, sessionID, "Read").Scan(
		&retrievedToolStats.SessionID, &retrievedToolStats.ToolName,
		&retrievedToolStats.ExecutionCount, &retrievedToolStats.SuccessCount,
		&retrievedToolStats.FailureCount, &retrievedToolStats.AvgDurationMS,
		&retrievedToolStats.MinDurationMS, &retrievedToolStats.MaxDurationMS,
	)
	if err != nil {
		t.Fatalf("Failed to retrieve flushed tool stats: %v", err)
	}

	if retrievedToolStats.ExecutionCount != 3 {
		t.Errorf("Expected flushed execution count 3, got %d", retrievedToolStats.ExecutionCount)
	}
	if retrievedToolStats.SuccessCount != 2 {
		t.Errorf("Expected flushed success count 2, got %d", retrievedToolStats.SuccessCount)
	}
}

func TestEngineMultipleModels(t *testing.T) {
	dbPath := "./test_engine_multi_model.db"
	defer os.Remove(dbPath)

	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	engine := NewEngine(store)
	sessionID := "multi-model-session"

	// Process costs for multiple models in same session
	models := []string{"claude-sonnet-4-5", "claude-opus-4-5", "claude-haiku-4"}
	for i, model := range models {
		costRecord := &MetricRecord{
			Timestamp:   time.Now(),
			SessionID:   sessionID,
			MetricName:  "claude_code.cost.usage",
			MetricValue: float64(i+1) * 0.001,
			Attributes: map[string]string{
				"model": model,
			},
		}
		engine.ProcessMetric(costRecord)
	}

	// Verify all models are tracked
	engine.cacheMutex.RLock()
	modelMap := engine.modelStatsCache[sessionID]
	engine.cacheMutex.RUnlock()

	if len(modelMap) != 3 {
		t.Errorf("Expected 3 models tracked, got %d", len(modelMap))
	}

	for i, model := range models {
		stats, exists := modelMap[model]
		if !exists {
			t.Errorf("Model %s not found in cache", model)
			continue
		}
		expectedCost := float64(i+1) * 0.001
		if stats.CostUSD != expectedCost {
			t.Errorf("Model %s: expected cost %f, got %f", model, expectedCost, stats.CostUSD)
		}
	}

	// Flush and verify all models were written
	engine.FlushCache()

	var count int
	err = store.db.QueryRow(`
		SELECT COUNT(*) FROM session_model_stats WHERE session_id = ?
	`, sessionID).Scan(&count)
	if err != nil {
		t.Fatalf("Failed to count flushed model stats: %v", err)
	}

	if count != 3 {
		t.Errorf("Expected 3 flushed model stats, got %d", count)
	}
}
