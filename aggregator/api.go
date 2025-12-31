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

	// Register endpoints
	mux.HandleFunc("/api/stats/session/", server.handleSessionStats)
	mux.HandleFunc("/api/stats/user/", server.handleUserStats)
	mux.HandleFunc("/api/stats/org/", server.handleOrgStats)
	mux.HandleFunc("/api/health", server.handleHealth)

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
	log.Printf("Endpoints:")
	log.Printf("  GET http://localhost:%d/api/stats/session/{session_id}", s.port)
	log.Printf("  GET http://localhost:%d/api/stats/user/{user_id}?limit=10", s.port)
	log.Printf("  GET http://localhost:%d/api/stats/org/{org_id}?limit=10", s.port)
	log.Printf("  GET http://localhost:%d/api/health", s.port)

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

// handleSessionStats handles GET /api/stats/session/{session_id}
func (s *APIServer) handleSessionStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract session ID from path
	path := strings.TrimPrefix(r.URL.Path, "/api/stats/session/")
	sessionID := strings.TrimSpace(path)

	if sessionID == "" {
		http.Error(w, "Session ID required", http.StatusBadRequest)
		return
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
