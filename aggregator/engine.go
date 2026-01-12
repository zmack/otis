package aggregator

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"
)

type Engine struct {
	store            *Store
	sessionCache     map[string]*SessionStats
	modelStatsCache  map[string]map[string]*SessionModelStats  // sessionID -> model -> stats
	toolStatsCache   map[string]map[string]*SessionToolStats   // sessionID -> toolName -> stats
	cacheMutex       sync.RWMutex
	flushInterval    time.Duration
}

// NewEngine creates a new aggregation engine
func NewEngine(store *Store) *Engine {
	engine := &Engine{
		store:           store,
		sessionCache:    make(map[string]*SessionStats),
		modelStatsCache: make(map[string]map[string]*SessionModelStats),
		toolStatsCache:  make(map[string]map[string]*SessionToolStats),
		flushInterval:   10 * time.Second,
	}

	// Start periodic flush
	go engine.periodicFlush()

	return engine
}

// periodicFlush periodically writes cached data to database
func (e *Engine) periodicFlush() {
	ticker := time.NewTicker(e.flushInterval)
	for range ticker.C {
		e.FlushCache()
	}
}

// FlushCache writes all cached session stats to the database
func (e *Engine) FlushCache() {
	e.cacheMutex.Lock()
	defer e.cacheMutex.Unlock()

	// Flush session stats
	for sessionID, stats := range e.sessionCache {
		stats.UpdatedAt = time.Now()
		if err := e.store.UpsertSessionStats(stats); err != nil {
			log.Printf("Error upserting session stats for %s: %v", sessionID, err)
		}
	}

	// Flush model stats
	modelStatsCount := 0
	for sessionID, modelMap := range e.modelStatsCache {
		for _, modelStats := range modelMap {
			if err := e.store.UpsertSessionModelStats(modelStats); err != nil {
				log.Printf("Error upserting model stats for session %s, model %s: %v", sessionID, modelStats.Model, err)
			} else {
				modelStatsCount++
			}
		}
	}

	// Flush tool stats
	toolStatsCount := 0
	for sessionID, toolMap := range e.toolStatsCache {
		for _, toolStats := range toolMap {
			if err := e.store.UpsertSessionToolStats(toolStats); err != nil {
				log.Printf("Error upserting tool stats for session %s, tool %s: %v", sessionID, toolStats.ToolName, err)
			} else {
				toolStatsCount++
			}
		}
	}

	log.Printf("Flushed %d session stats, %d model stats, %d tool stats to database", len(e.sessionCache), modelStatsCount, toolStatsCount)
}

// ProcessMetric processes a metric record and updates aggregations
func (e *Engine) ProcessMetric(record *MetricRecord) {
	if record.SessionID == "" {
		return // Skip if no session ID
	}

	e.cacheMutex.Lock()
	defer e.cacheMutex.Unlock()

	// Get or create session stats
	stats, exists := e.sessionCache[record.SessionID]
	if !exists {
		stats = &SessionStats{
			SessionID:      record.SessionID,
			UserID:         record.UserID,
			OrganizationID: record.OrganizationID,
			ServiceName:    record.ServiceName,
			StartTime:      record.Timestamp,
			CreatedAt:      time.Now(),
			TerminalType:   record.Attributes["terminal.type"],
			HostArch:       record.Attributes["host.arch"],
			OSType:         record.Attributes["os.type"],
			ModelsUsed:     "[]",
			ToolsUsed:      "{}",
		}
		e.sessionCache[record.SessionID] = stats
	}

	stats.LastUpdateTime = record.Timestamp

	// Process specific metric types
	switch record.MetricName {
	case "claude_code.session.count":
		// Session start marker
		if stats.StartTime.IsZero() || record.Timestamp.Before(stats.StartTime) {
			stats.StartTime = record.Timestamp
		}

	case "claude_code.cost.usage":
		// Add to total cost
		var cost float64
		if c, ok := record.MetricValue.(float64); ok {
			cost = c
			stats.TotalCostUSD += cost
		} else if costInt, ok := record.MetricValue.(int64); ok {
			cost = float64(costInt)
			stats.TotalCostUSD += cost
		}

		// Track per-model cost
		if model := record.Attributes["model"]; model != "" && cost > 0 {
			e.updateModelStats(record.SessionID, model, func(ms *SessionModelStats) {
				ms.CostUSD += cost
				ms.RequestCount++
			})
		}

	case "claude_code.token.usage":
		// Add to token counters based on type
		tokenType := record.Attributes["type"]
		var tokenValue int64

		if val, ok := record.MetricValue.(int64); ok {
			tokenValue = val
		} else if val, ok := record.MetricValue.(float64); ok {
			tokenValue = int64(val)
		}

		switch tokenType {
		case "input":
			stats.TotalInputTokens += tokenValue
		case "output":
			stats.TotalOutputTokens += tokenValue
		case "cacheRead":
			stats.TotalCacheReadTokens += tokenValue
		case "cacheCreation":
			stats.TotalCacheCreationTokens += tokenValue
		}

		// Track per-model tokens
		if model := record.Attributes["model"]; model != "" && tokenValue > 0 {
			e.updateModelStats(record.SessionID, model, func(ms *SessionModelStats) {
				switch tokenType {
				case "input":
					ms.InputTokens += tokenValue
				case "output":
					ms.OutputTokens += tokenValue
				case "cacheRead":
					ms.CacheReadTokens += tokenValue
				case "cacheCreation":
					ms.CacheCreationTokens += tokenValue
				}
			})
		}

	case "claude_code.active_time.total":
		// Add to active time
		if activeTime, ok := record.MetricValue.(float64); ok {
			stats.TotalActiveTimeSeconds += activeTime
		} else if activeTimeInt, ok := record.MetricValue.(int64); ok {
			stats.TotalActiveTimeSeconds += float64(activeTimeInt)
		}
	}

	// Track models used
	if model := record.Attributes["model"]; model != "" {
		e.addToModelsUsed(stats, model)
	}
}

// ProcessLog processes a log record and updates aggregations
func (e *Engine) ProcessLog(record *LogRecord) {
	if record.SessionID == "" {
		return
	}

	e.cacheMutex.Lock()
	defer e.cacheMutex.Unlock()

	// Get or create session stats
	stats, exists := e.sessionCache[record.SessionID]
	if !exists {
		stats = &SessionStats{
			SessionID:      record.SessionID,
			UserID:         record.UserID,
			OrganizationID: record.OrganizationID,
			ServiceName:    record.ServiceName,
			StartTime:      record.Timestamp,
			CreatedAt:      time.Now(),
			ModelsUsed:     "[]",
			ToolsUsed:      "{}",
		}
		e.sessionCache[record.SessionID] = stats
	}

	stats.LastUpdateTime = record.Timestamp

	// Determine log type from body
	if containsString(record.Body, "claude_code.api_request") {
		stats.APIRequestCount++

		// Extract latency if available
		durationMS := extractFloat(record.Attributes, "duration_ms")
		if durationMS > 0 {
			stats.TotalAPILatencyMS += durationMS
			stats.AvgAPILatencyMS = stats.TotalAPILatencyMS / float64(stats.APIRequestCount)
		}

		// Track per-model latency
		if model := extractString(record.Attributes, "model"); model != "" && durationMS > 0 {
			e.updateModelStats(record.SessionID, model, func(ms *SessionModelStats) {
				ms.TotalLatencyMS += durationMS
				// Request count is tracked in cost.usage, so we calculate avg based on that
				if ms.RequestCount > 0 {
					ms.AvgLatencyMS = ms.TotalLatencyMS / float64(ms.RequestCount)
				}
			})
		}

	} else if containsString(record.Body, "claude_code.user_prompt") {
		stats.UserPromptCount++

	} else if containsString(record.Body, "claude_code.tool_decision") {
		// Track tool usage from decisions
		if toolName := extractString(record.Attributes, "tool_name"); toolName != "" {
			e.addToToolsUsed(stats, toolName)
		}

	} else if containsString(record.Body, "claude_code.tool_result") {
		stats.ToolExecutionCount++

		// Track success/failure
		success := extractBool(record.Attributes, "success")
		if success {
			stats.ToolSuccessCount++
		} else {
			stats.ToolFailureCount++
		}

		// Track tool name
		toolName := extractString(record.Attributes, "tool_name")
		if toolName != "" {
			e.addToToolsUsed(stats, toolName)

			// Track per-tool stats
			durationMS := extractFloat(record.Attributes, "duration_ms")
			e.updateToolStats(record.SessionID, toolName, func(ts *SessionToolStats) {
				ts.ExecutionCount++
				if success {
					ts.SuccessCount++
				} else {
					ts.FailureCount++
				}
				if durationMS > 0 {
					ts.TotalDurationMS += durationMS
					ts.AvgDurationMS = ts.TotalDurationMS / float64(ts.ExecutionCount)
					if ts.MinDurationMS == 0 || durationMS < ts.MinDurationMS {
						ts.MinDurationMS = durationMS
					}
					if durationMS > ts.MaxDurationMS {
						ts.MaxDurationMS = durationMS
					}
				}
			})
		}
	}
}

// ProcessTrace processes a trace record and updates aggregations
func (e *Engine) ProcessTrace(record *TraceRecord) {
	if record.SessionID == "" {
		return
	}

	e.cacheMutex.Lock()
	defer e.cacheMutex.Unlock()

	// Get or create session stats
	stats, exists := e.sessionCache[record.SessionID]
	if !exists {
		stats = &SessionStats{
			SessionID:      record.SessionID,
			UserID:         record.UserID,
			OrganizationID: record.OrganizationID,
			ServiceName:    record.ServiceName,
			StartTime:      record.Timestamp,
			CreatedAt:      time.Now(),
			ModelsUsed:     "[]",
			ToolsUsed:      "{}",
		}
		e.sessionCache[record.SessionID] = stats
	}

	stats.LastUpdateTime = record.Timestamp

	// Could track span performance metrics here
	// For now, we're mainly using logs for detailed tracking
}

// Helper functions

func (e *Engine) addToModelsUsed(stats *SessionStats, model string) {
	var models []string
	if err := json.Unmarshal([]byte(stats.ModelsUsed), &models); err != nil {
		models = []string{}
	}

	// Add if not already present
	found := false
	for _, m := range models {
		if m == model {
			found = true
			break
		}
	}

	if !found {
		models = append(models, model)
		if data, err := json.Marshal(models); err == nil {
			stats.ModelsUsed = string(data)
		}
	}
}

func (e *Engine) addToToolsUsed(stats *SessionStats, toolName string) {
	var tools map[string]int
	if err := json.Unmarshal([]byte(stats.ToolsUsed), &tools); err != nil {
		tools = make(map[string]int)
	}

	tools[toolName]++

	if data, err := json.Marshal(tools); err == nil {
		stats.ToolsUsed = string(data)
	}
}

func containsString(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 &&
		(s == substr || len(s) > len(substr) && stringContains(s, substr))
}

func stringContains(s, substr string) bool {
	return len(s) >= len(substr) &&
		(s == substr ||
		 (len(s) > len(substr) &&
		  (hasSubstring(s, substr))))
}

func hasSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func extractFloat(attrs map[string]interface{}, key string) float64 {
	if val, ok := attrs[key]; ok {
		// Try different numeric types
		switch v := val.(type) {
		case float64:
			return v
		case int:
			return float64(v)
		case int64:
			return float64(v)
		case string:
			var f float64
			fmt.Sscanf(v, "%f", &f)
			return f
		case map[string]interface{}:
			// Could be wrapped in a value object
			if doubleVal, ok := v["doubleValue"].(float64); ok {
				return doubleVal
			}
			if intVal, ok := v["intValue"].(float64); ok {
				return intVal
			}
		}
	}
	return 0
}

func extractString(attrs map[string]interface{}, key string) string {
	if val, ok := attrs[key]; ok {
		switch v := val.(type) {
		case string:
			return v
		case map[string]interface{}:
			if strVal, ok := v["stringValue"].(string); ok {
				return strVal
			}
		}
	}
	return ""
}

func extractBool(attrs map[string]interface{}, key string) bool {
	if val, ok := attrs[key]; ok {
		switch v := val.(type) {
		case bool:
			return v
		case map[string]interface{}:
			if boolVal, ok := v["boolValue"].(bool); ok {
				return boolVal
			}
		}
	}
	return false
}

// updateModelStats gets or creates model stats for a session and applies the update function
func (e *Engine) updateModelStats(sessionID, model string, updateFn func(*SessionModelStats)) {
	// Get or create session-level map
	if e.modelStatsCache[sessionID] == nil {
		e.modelStatsCache[sessionID] = make(map[string]*SessionModelStats)
	}

	// Get or create model stats
	modelStats, exists := e.modelStatsCache[sessionID][model]
	if !exists {
		modelStats = &SessionModelStats{
			SessionID: sessionID,
			Model:     model,
		}
		e.modelStatsCache[sessionID][model] = modelStats
	}

	// Apply update
	updateFn(modelStats)
}

// updateToolStats gets or creates tool stats for a session and applies the update function
func (e *Engine) updateToolStats(sessionID, toolName string, updateFn func(*SessionToolStats)) {
	// Get or create session-level map
	if e.toolStatsCache[sessionID] == nil {
		e.toolStatsCache[sessionID] = make(map[string]*SessionToolStats)
	}

	// Get or create tool stats
	toolStats, exists := e.toolStatsCache[sessionID][toolName]
	if !exists {
		toolStats = &SessionToolStats{
			SessionID: sessionID,
			ToolName:  toolName,
		}
		e.toolStatsCache[sessionID][toolName] = toolStats
	}

	// Apply update
	updateFn(toolStats)
}
