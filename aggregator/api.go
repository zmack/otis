package aggregator

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
)

type APIServer struct {
	store      *Store
	engine     *Engine
	httpServer *http.Server
	port       int
}

// NewAPIServer creates a new API server
func NewAPIServer(port int, store *Store, engine *Engine) *APIServer {
	server := &APIServer{
		store:  store,
		engine: engine,
		port:   port,
	}

	mux := http.NewServeMux()

	// Register endpoints (legacy)
	mux.HandleFunc("/api/stats/session/", server.handleSessionStats)
	mux.HandleFunc("/api/stats/user/", server.handleUserStats)
	mux.HandleFunc("/api/stats/org/", server.handleOrgStats)
	mux.HandleFunc("/api/stats/models", server.handleModelsStats)
	mux.HandleFunc("/api/stats/tools", server.handleToolsStats)
	mux.HandleFunc("/api/health", server.handleHealth)

	// New schema endpoints
	mux.HandleFunc("/api/v2/sessions/", server.handleV2Session)
	mux.HandleFunc("/api/v2/sessions", server.handleV2SessionsList)
	mux.HandleFunc("/api/v2/tools", server.handleV2Tools)

	server.httpServer = &http.Server{
		Addr:         fmt.Sprintf(":%d", port),
		Handler:      server.loggingMiddleware(mux),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	return server
}

// Start starts the API server
func (s *APIServer) Start() error {
	log.Printf("Starting aggregation API server on port %d", s.port)
	log.Printf("Legacy endpoints:")
	log.Printf("  GET http://localhost:%d/api/stats/session/{session_id}", s.port)
	log.Printf("  GET http://localhost:%d/api/stats/session/{session_id}/models", s.port)
	log.Printf("  GET http://localhost:%d/api/stats/session/{session_id}/tools", s.port)
	log.Printf("  GET http://localhost:%d/api/stats/user/{user_id}?limit=10", s.port)
	log.Printf("  GET http://localhost:%d/api/stats/org/{org_id}?limit=10", s.port)
	log.Printf("  GET http://localhost:%d/api/stats/models?limit=50", s.port)
	log.Printf("  GET http://localhost:%d/api/stats/tools?limit=50", s.port)
	log.Printf("  GET http://localhost:%d/api/health", s.port)
	log.Printf("V2 endpoints (new schema):")
	log.Printf("  GET http://localhost:%d/api/v2/sessions?org_id=X&user_id=Y&limit=10", s.port)
	log.Printf("  GET http://localhost:%d/api/v2/sessions/{session_id}", s.port)
	log.Printf("  GET http://localhost:%d/api/v2/sessions/{session_id}/tools", s.port)
	log.Printf("  GET http://localhost:%d/api/v2/sessions/{session_id}/prompts", s.port)
	log.Printf("  GET http://localhost:%d/api/v2/tools?limit=50", s.port)

	if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("failed to start API server: %w", err)
	}
	return nil
}

// Shutdown gracefully shuts down the API server
func (s *APIServer) Shutdown(ctx context.Context) error {
	log.Println("Shutting down API server...")
	return s.httpServer.Shutdown(ctx)
}

// handleSessionStats handles GET /api/stats/session/{session_id}[/models|/tools]
func (s *APIServer) handleSessionStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract path after /api/stats/session/
	path := strings.TrimPrefix(r.URL.Path, "/api/stats/session/")
	parts := strings.Split(path, "/")

	if len(parts) == 0 || parts[0] == "" {
		http.Error(w, "Session ID required", http.StatusBadRequest)
		return
	}

	sessionID := parts[0]

	// Check for sub-routes
	if len(parts) > 1 {
		switch parts[1] {
		case "models":
			s.handleSessionModels(w, r, sessionID)
			return
		case "tools":
			s.handleSessionTools(w, r, sessionID)
			return
		default:
			http.Error(w, "Unknown sub-resource", http.StatusNotFound)
			return
		}
	}

	// Get session stats from database
	stats, err := s.store.GetSessionStats(sessionID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Session not found: %v", err), http.StatusNotFound)
		return
	}

	// Build response
	response := buildSessionStatsResponse(stats)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleUserStats handles GET /api/stats/user/{user_id}
func (s *APIServer) handleUserStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract user ID from path
	path := strings.TrimPrefix(r.URL.Path, "/api/stats/user/")
	userID := strings.TrimSpace(path)

	if userID == "" {
		http.Error(w, "User ID required", http.StatusBadRequest)
		return
	}

	// Get limit from query params
	limit := 10
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		fmt.Sscanf(limitStr, "%d", &limit)
	}
	if limit > 100 {
		limit = 100
	}

	// Get user sessions from database
	sessions, err := s.store.GetUserSessionStats(userID, limit)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error retrieving user stats: %v", err), http.StatusInternalServerError)
		return
	}

	// Build aggregated response
	response := buildUserStatsResponse(userID, sessions)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleOrgStats handles GET /api/stats/org/{org_id}
func (s *APIServer) handleOrgStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract org ID from path
	path := strings.TrimPrefix(r.URL.Path, "/api/stats/org/")
	orgID := strings.TrimSpace(path)

	if orgID == "" {
		http.Error(w, "Organization ID required", http.StatusBadRequest)
		return
	}

	// Get limit from query params
	limit := 10
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		fmt.Sscanf(limitStr, "%d", &limit)
	}
	if limit > 100 {
		limit = 100
	}

	// Get org sessions from database
	sessions, err := s.store.GetOrgSessionStats(orgID, limit)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error retrieving org stats: %v", err), http.StatusInternalServerError)
		return
	}

	// Build aggregated response
	response := buildOrgStatsResponse(orgID, sessions)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleHealth handles GET /api/health
func (s *APIServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Flush engine cache before reporting health
	s.engine.FlushCache()

	health := map[string]interface{}{
		"status":    "ok",
		"timestamp": time.Now().Format(time.RFC3339),
		"service":   "otis-aggregator",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(health)
}

// loggingMiddleware logs HTTP requests
func (s *APIServer) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		log.Printf("API: Started %s %s", r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
		log.Printf("API: Completed %s %s in %v", r.Method, r.URL.Path, time.Since(start))
	})
}

// buildSessionStatsResponse builds a JSON response for session stats
func buildSessionStatsResponse(stats *SessionStats) map[string]interface{} {
	// Parse models and tools from JSON
	var models []string
	json.Unmarshal([]byte(stats.ModelsUsed), &models)

	var tools map[string]int
	json.Unmarshal([]byte(stats.ToolsUsed), &tools)

	// Calculate cost by model (simplified - assuming single model for now)
	costByModel := make(map[string]float64)
	if len(models) > 0 {
		// Distribute cost evenly across models (simplified)
		costPerModel := stats.TotalCostUSD / float64(len(models))
		for _, model := range models {
			costByModel[model] = costPerModel
		}
	}

	return map[string]interface{}{
		"session_id":      stats.SessionID,
		"user_id":         stats.UserID,
		"organization_id": stats.OrganizationID,
		"service_name":    stats.ServiceName,
		"window": map[string]interface{}{
			"start":            stats.StartTime.Format(time.RFC3339),
			"end":              stats.LastUpdateTime.Format(time.RFC3339),
			"duration_seconds": stats.LastUpdateTime.Sub(stats.StartTime).Seconds(),
		},
		"environment": map[string]interface{}{
			"terminal_type": stats.TerminalType,
			"host_arch":     stats.HostArch,
			"os_type":       stats.OSType,
		},
		"costs": map[string]interface{}{
			"total_usd": stats.TotalCostUSD,
			"by_model":  costByModel,
		},
		"tokens": map[string]interface{}{
			"total":          stats.TotalInputTokens + stats.TotalOutputTokens + stats.TotalCacheReadTokens,
			"input":          stats.TotalInputTokens,
			"output":         stats.TotalOutputTokens,
			"cache_read":     stats.TotalCacheReadTokens,
			"cache_creation": stats.TotalCacheCreationTokens,
		},
		"activity": map[string]interface{}{
			"api_requests":     stats.APIRequestCount,
			"user_prompts":     stats.UserPromptCount,
			"tools_executed":   stats.ToolExecutionCount,
			"tools_succeeded":  stats.ToolSuccessCount,
			"tools_failed":     stats.ToolFailureCount,
			"active_time_seconds": stats.TotalActiveTimeSeconds,
		},
		"performance": map[string]interface{}{
			"avg_api_latency_ms": stats.AvgAPILatencyMS,
		},
		"tools":  tools,
		"models": models,
		"metadata": map[string]interface{}{
			"created_at": stats.CreatedAt.Format(time.RFC3339),
			"updated_at": stats.UpdatedAt.Format(time.RFC3339),
		},
	}
}

// buildUserStatsResponse builds aggregated stats for a user across sessions
func buildUserStatsResponse(userID string, sessions []*SessionStats) map[string]interface{} {
	if len(sessions) == 0 {
		return map[string]interface{}{
			"user_id":        userID,
			"total_sessions": 0,
			"message":        "No sessions found for this user",
		}
	}

	// Aggregate across all sessions
	var totalCost float64
	var totalInputTokens, totalOutputTokens, totalCacheRead, totalCacheCreation int64
	var totalActiveTime float64
	var totalAPIRequests, totalPrompts, totalToolExecs int
	modelCounts := make(map[string]int)
	toolCounts := make(map[string]int)
	var firstSession, lastSession time.Time

	for i, session := range sessions {
		totalCost += session.TotalCostUSD
		totalInputTokens += session.TotalInputTokens
		totalOutputTokens += session.TotalOutputTokens
		totalCacheRead += session.TotalCacheReadTokens
		totalCacheCreation += session.TotalCacheCreationTokens
		totalActiveTime += session.TotalActiveTimeSeconds
		totalAPIRequests += session.APIRequestCount
		totalPrompts += session.UserPromptCount
		totalToolExecs += session.ToolExecutionCount

		// Track first and last session times
		if i == 0 || session.StartTime.After(lastSession) {
			lastSession = session.StartTime
		}
		if i == 0 || session.StartTime.Before(firstSession) {
			firstSession = session.StartTime
		}

		// Count models
		var models []string
		json.Unmarshal([]byte(session.ModelsUsed), &models)
		for _, model := range models {
			modelCounts[model]++
		}

		// Count tools
		var tools map[string]int
		json.Unmarshal([]byte(session.ToolsUsed), &tools)
		for tool, count := range tools {
			toolCounts[tool] += count
		}
	}

	numSessions := len(sessions)

	return map[string]interface{}{
		"user_id":         userID,
		"organization_id": sessions[0].OrganizationID,
		"summary": map[string]interface{}{
			"total_sessions":       numSessions,
			"first_session":        firstSession.Format(time.RFC3339),
			"last_session":         lastSession.Format(time.RFC3339),
			"total_active_time_seconds": totalActiveTime,
		},
		"costs": map[string]interface{}{
			"total_usd":          totalCost,
			"avg_per_session":    totalCost / float64(numSessions),
		},
		"tokens": map[string]interface{}{
			"total":              totalInputTokens + totalOutputTokens + totalCacheRead,
			"input":              totalInputTokens,
			"output":             totalOutputTokens,
			"cache_read":         totalCacheRead,
			"cache_creation":     totalCacheCreation,
			"avg_per_session":    float64(totalInputTokens+totalOutputTokens+totalCacheRead) / float64(numSessions),
		},
		"activity": map[string]interface{}{
			"total_api_requests": totalAPIRequests,
			"total_prompts":      totalPrompts,
			"total_tool_execs":   totalToolExecs,
			"avg_api_per_session": float64(totalAPIRequests) / float64(numSessions),
		},
		"models":   modelCounts,
		"tools":    toolCounts,
		"sessions": buildSessionList(sessions),
	}
}

// buildOrgStatsResponse builds aggregated stats for an organization across sessions
func buildOrgStatsResponse(orgID string, sessions []*SessionStats) map[string]interface{} {
	if len(sessions) == 0 {
		return map[string]interface{}{
			"organization_id": orgID,
			"total_sessions":  0,
			"message":         "No sessions found for this organization",
		}
	}

	// Aggregate across all sessions
	var totalCost float64
	var totalTokens int64
	var totalActiveTime float64
	userSet := make(map[string]bool)
	var firstSession, lastSession time.Time

	for i, session := range sessions {
		totalCost += session.TotalCostUSD
		totalTokens += session.TotalInputTokens + session.TotalOutputTokens + session.TotalCacheReadTokens
		totalActiveTime += session.TotalActiveTimeSeconds
		userSet[session.UserID] = true

		if i == 0 || session.StartTime.After(lastSession) {
			lastSession = session.StartTime
		}
		if i == 0 || session.StartTime.Before(firstSession) {
			firstSession = session.StartTime
		}
	}

	numSessions := len(sessions)
	numUsers := len(userSet)

	return map[string]interface{}{
		"organization_id": orgID,
		"summary": map[string]interface{}{
			"total_users":          numUsers,
			"total_sessions":       numSessions,
			"first_session":        firstSession.Format(time.RFC3339),
			"last_session":         lastSession.Format(time.RFC3339),
			"total_active_time_seconds": totalActiveTime,
		},
		"costs": map[string]interface{}{
			"total_usd":          totalCost,
			"avg_per_session":    totalCost / float64(numSessions),
			"avg_per_user":       totalCost / float64(numUsers),
		},
		"tokens": map[string]interface{}{
			"total":           totalTokens,
			"avg_per_session": float64(totalTokens) / float64(numSessions),
			"avg_per_user":    float64(totalTokens) / float64(numUsers),
		},
		"sessions": buildSessionList(sessions),
	}
}

// buildSessionList builds a simplified list of sessions
func buildSessionList(sessions []*SessionStats) []map[string]interface{} {
	result := make([]map[string]interface{}, len(sessions))
	for i, session := range sessions {
		result[i] = map[string]interface{}{
			"session_id":    session.SessionID,
			"user_id":       session.UserID,
			"start_time":    session.StartTime.Format(time.RFC3339),
			"cost_usd":      session.TotalCostUSD,
			"total_tokens":  session.TotalInputTokens + session.TotalOutputTokens,
			"api_requests":  session.APIRequestCount,
		}
	}
	return result
}

// handleSessionModels handles GET /api/stats/session/{session_id}/models
func (s *APIServer) handleSessionModels(w http.ResponseWriter, r *http.Request, sessionID string) {
	modelStats, err := s.store.GetSessionModelStats(sessionID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error retrieving model stats: %v", err), http.StatusInternalServerError)
		return
	}

	// Build response
	models := make([]map[string]interface{}, len(modelStats))
	for i, ms := range modelStats {
		models[i] = map[string]interface{}{
			"model": ms.Model,
			"cost_usd": ms.CostUSD,
			"tokens": map[string]interface{}{
				"input":          ms.InputTokens,
				"output":         ms.OutputTokens,
				"cache_read":     ms.CacheReadTokens,
				"cache_creation": ms.CacheCreationTokens,
				"total":          ms.InputTokens + ms.OutputTokens + ms.CacheReadTokens,
			},
			"request_count":  ms.RequestCount,
			"avg_latency_ms": ms.AvgLatencyMS,
		}
	}

	response := map[string]interface{}{
		"session_id": sessionID,
		"models":     models,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleSessionTools handles GET /api/stats/session/{session_id}/tools
func (s *APIServer) handleSessionTools(w http.ResponseWriter, r *http.Request, sessionID string) {
	toolStats, err := s.store.GetSessionToolStats(sessionID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error retrieving tool stats: %v", err), http.StatusInternalServerError)
		return
	}

	// Build response
	tools := make([]map[string]interface{}, len(toolStats))
	for i, ts := range toolStats {
		successRate := 0.0
		if ts.ExecutionCount > 0 {
			successRate = float64(ts.SuccessCount) / float64(ts.ExecutionCount)
		}

		tools[i] = map[string]interface{}{
			"tool_name":       ts.ToolName,
			"execution_count": ts.ExecutionCount,
			"success_count":   ts.SuccessCount,
			"failure_count":   ts.FailureCount,
			"duration": map[string]interface{}{
				"avg_ms":   ts.AvgDurationMS,
				"min_ms":   ts.MinDurationMS,
				"max_ms":   ts.MaxDurationMS,
				"total_ms": ts.TotalDurationMS,
			},
			"success_rate": successRate,
		}
	}

	response := map[string]interface{}{
		"session_id": sessionID,
		"tools":      tools,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleModelsStats handles GET /api/stats/models
func (s *APIServer) handleModelsStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get limit from query params
	limit := 50
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		fmt.Sscanf(limitStr, "%d", &limit)
	}
	if limit > 100 {
		limit = 100
	}

	modelAggs, err := s.store.GetAllModelStats(limit)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error retrieving model stats: %v", err), http.StatusInternalServerError)
		return
	}

	// Build response
	models := make([]map[string]interface{}, len(modelAggs))
	for i, ma := range modelAggs {
		models[i] = map[string]interface{}{
			"model":             ma.Model,
			"total_sessions":    ma.TotalSessions,
			"total_cost_usd":    ma.TotalCostUSD,
			"total_requests":    ma.TotalRequests,
			"total_tokens": map[string]interface{}{
				"input":          ma.TotalInputTokens,
				"output":         ma.TotalOutputTokens,
				"cache_read":     ma.TotalCacheReadTokens,
				"cache_creation": ma.TotalCacheCreationTokens,
				"total":          ma.TotalInputTokens + ma.TotalOutputTokens + ma.TotalCacheReadTokens,
			},
			"avg_cost_per_session": ma.AvgCostPerSession,
			"avg_latency_ms":       ma.AvgLatencyMS,
		}
	}

	response := map[string]interface{}{
		"models": models,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleToolsStats handles GET /api/stats/tools
func (s *APIServer) handleToolsStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get limit from query params
	limit := 50
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		fmt.Sscanf(limitStr, "%d", &limit)
	}
	if limit > 100 {
		limit = 100
	}

	toolAggs, err := s.store.GetAllToolStats(limit)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error retrieving tool stats: %v", err), http.StatusInternalServerError)
		return
	}

	// Build response
	tools := make([]map[string]interface{}, len(toolAggs))
	for i, ta := range toolAggs {
		tools[i] = map[string]interface{}{
			"tool_name":        ta.ToolName,
			"total_executions": ta.TotalExecutions,
			"total_successes":  ta.TotalSuccesses,
			"total_failures":   ta.TotalFailures,
			"success_rate":     ta.SuccessRate,
			"avg_duration_ms":  ta.AvgDurationMS,
			"used_in_sessions": ta.SessionsUsedIn,
		}
	}

	response := map[string]interface{}{
		"tools": tools,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// V2 API handlers for new schema

// handleV2SessionsList handles GET /api/v2/sessions?org_id=X&user_id=Y&limit=N
func (s *APIServer) handleV2SessionsList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get query params
	orgID := r.URL.Query().Get("org_id")
	userID := r.URL.Query().Get("user_id")
	limit := 10
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		fmt.Sscanf(limitStr, "%d", &limit)
	}
	if limit > 100 {
		limit = 100
	}

	var sessions []*Session
	var err error

	if userID != "" {
		sessions, err = s.store.GetSessionsByUser(userID, limit)
	} else if orgID != "" {
		sessions, err = s.store.GetSessionsByOrg(orgID, limit)
	} else {
		sessions, err = s.store.GetAllSessions(limit)
	}

	if err != nil {
		http.Error(w, fmt.Sprintf("Error retrieving sessions: %v", err), http.StatusInternalServerError)
		return
	}

	// Build response
	sessionList := make([]map[string]interface{}, len(sessions))
	for i, session := range sessions {
		sessionList[i] = buildV2SessionResponse(session)
	}

	response := map[string]interface{}{
		"sessions": sessionList,
		"count":    len(sessions),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleV2Session handles GET /api/v2/sessions/{session_id}[/tools]
func (s *APIServer) handleV2Session(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract path after /api/v2/sessions/
	path := strings.TrimPrefix(r.URL.Path, "/api/v2/sessions/")
	parts := strings.Split(path, "/")

	if len(parts) == 0 || parts[0] == "" {
		http.Error(w, "Session ID required", http.StatusBadRequest)
		return
	}

	sessionID := parts[0]

	// Check for sub-routes
	if len(parts) > 1 {
		switch parts[1] {
		case "tools":
			s.handleV2SessionTools(w, r, sessionID)
			return
		case "prompts":
			s.handleV2SessionPrompts(w, r, sessionID)
			return
		default:
			http.Error(w, "Unknown sub-resource", http.StatusNotFound)
			return
		}
	}

	// Get session from database
	session, err := s.store.GetSession(sessionID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Session not found: %v", err), http.StatusNotFound)
		return
	}

	response := buildV2SessionResponse(session)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleV2SessionPrompts handles GET /api/v2/sessions/{session_id}/prompts
func (s *APIServer) handleV2SessionPrompts(w http.ResponseWriter, r *http.Request, sessionID string) {
	prompts, err := s.store.GetSessionPrompts(sessionID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error retrieving session prompts: %v", err), http.StatusInternalServerError)
		return
	}

	promptList := make([]map[string]interface{}, len(prompts))
	for i, prompt := range prompts {
		promptList[i] = map[string]interface{}{
			"id":            prompt.ID,
			"prompt_text":   prompt.PromptText,
			"prompt_length": prompt.PromptLength,
			"timestamp":     prompt.Timestamp.Format(time.RFC3339Nano),
		}
	}

	response := map[string]interface{}{
		"session_id": sessionID,
		"count":      len(prompts),
		"prompts":    promptList,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleV2SessionTools handles GET /api/v2/sessions/{session_id}/tools
func (s *APIServer) handleV2SessionTools(w http.ResponseWriter, r *http.Request, sessionID string) {
	tools, err := s.store.GetSessionTools(sessionID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error retrieving session tools: %v", err), http.StatusInternalServerError)
		return
	}

	// Build response
	toolList := make([]map[string]interface{}, len(tools))
	for i, tool := range tools {
		successRate := 0.0
		if tool.CallCount > 0 {
			successRate = float64(tool.SuccessCount) / float64(tool.CallCount)
		}
		avgDurationMS := 0.0
		if tool.CallCount > 0 {
			avgDurationMS = tool.TotalExecutionTimeMS / float64(tool.CallCount)
		}
		avgResultSizeBytes := int64(0)
		if tool.CallCount > 0 {
			avgResultSizeBytes = tool.TotalResultSizeBytes / int64(tool.CallCount)
		}

		toolList[i] = map[string]interface{}{
			"tool_name":               tool.ToolName,
			"call_count":              tool.CallCount,
			"success_count":           tool.SuccessCount,
			"failure_count":           tool.FailureCount,
			"success_rate":            successRate,
			"total_execution_time_ms": tool.TotalExecutionTimeMS,
			"avg_execution_time_ms":   avgDurationMS,
			"decisions": map[string]interface{}{
				"auto_approved":  tool.AutoApprovedCount,
				"user_approved":  tool.UserApprovedCount,
				"rejected":       tool.RejectedCount,
			},
			"result_size": map[string]interface{}{
				"total_bytes": tool.TotalResultSizeBytes,
				"avg_bytes":   avgResultSizeBytes,
			},
		}
	}

	response := map[string]interface{}{
		"session_id": sessionID,
		"tools":      toolList,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleV2Tools handles GET /api/v2/tools
func (s *APIServer) handleV2Tools(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get limit from query params
	limit := 50
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		fmt.Sscanf(limitStr, "%d", &limit)
	}
	if limit > 100 {
		limit = 100
	}

	toolAggs, err := s.store.GetToolAggregates(limit)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error retrieving tool stats: %v", err), http.StatusInternalServerError)
		return
	}

	// Build response
	tools := make([]map[string]interface{}, len(toolAggs))
	for i, ta := range toolAggs {
		tools[i] = map[string]interface{}{
			"tool_name":        ta.ToolName,
			"total_executions": ta.TotalExecutions,
			"total_successes":  ta.TotalSuccesses,
			"total_failures":   ta.TotalFailures,
			"success_rate":     ta.SuccessRate,
			"avg_duration_ms":  ta.AvgDurationMS,
			"used_in_sessions": ta.SessionsUsedIn,
		}
	}

	response := map[string]interface{}{
		"tools": tools,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// buildV2SessionResponse builds the JSON response for a session
func buildV2SessionResponse(session *Session) map[string]interface{} {
	response := map[string]interface{}{
		"session_id":      session.SessionID,
		"organization_id": session.OrganizationID,
		"user_id":         session.UserID,
		"start_time":      session.StartTime.Format(time.RFC3339),
		"costs": map[string]interface{}{
			"total_usd": session.TotalCostUSD,
		},
		"tokens": map[string]interface{}{
			"input":          session.TotalInputTokens,
			"output":         session.TotalOutputTokens,
			"cache_read":     session.TotalCacheReadTokens,
			"cache_creation": session.TotalCacheCreationTokens,
			"total":          session.TotalInputTokens + session.TotalOutputTokens + session.TotalCacheReadTokens,
		},
		"tool_call_count": session.ToolCallCount,
		"metadata": map[string]interface{}{
			"created_at": session.CreatedAt.Format(time.RFC3339),
			"updated_at": session.UpdatedAt.Format(time.RFC3339),
		},
	}

	if !session.EndTime.IsZero() {
		response["end_time"] = session.EndTime.Format(time.RFC3339)
		response["duration_seconds"] = session.EndTime.Sub(session.StartTime).Seconds()
	}

	return response
}
