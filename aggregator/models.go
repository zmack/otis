package aggregator

import "time"

// SessionStats represents aggregated statistics for a single session
type SessionStats struct {
	SessionID        string
	UserID           string
	OrganizationID   string
	ServiceName      string
	StartTime        time.Time
	LastUpdateTime   time.Time
	TerminalType     string
	HostArch         string
	OSType           string

	// Aggregated metrics
	TotalCostUSD             float64
	TotalInputTokens         int64
	TotalOutputTokens        int64
	TotalCacheReadTokens     int64
	TotalCacheCreationTokens int64
	TotalActiveTimeSeconds   float64

	// Event counts
	APIRequestCount     int
	UserPromptCount     int
	ToolExecutionCount  int
	ToolSuccessCount    int
	ToolFailureCount    int

	// Performance metrics
	AvgAPILatencyMS   float64
	TotalAPILatencyMS float64

	// JSON-encoded data
	ModelsUsed string // JSON array
	ToolsUsed  string // JSON object

	CreatedAt time.Time
	UpdatedAt time.Time
}

// UserStats represents aggregated statistics for a user within a time window
type UserStats struct {
	UserID         string
	OrganizationID string
	WindowStart    time.Time
	WindowEnd      time.Time
	WindowType     string // 'all-time', '7d', '30d', 'custom'

	// Aggregated metrics
	TotalSessions            int
	TotalCostUSD             float64
	TotalInputTokens         int64
	TotalOutputTokens        int64
	TotalCacheReadTokens     int64
	TotalCacheCreationTokens int64
	TotalActiveTimeSeconds   float64

	// Averages
	AvgCostPerSession            float64
	AvgTokensPerSession          float64
	AvgSessionDurationSeconds    float64

	// JSON-encoded preferences
	PreferredModels string // JSON array
	FavoriteTools   string // JSON array

	// Success metrics
	ToolSuccessRate float64

	LastSessionTime time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// OrgStats represents aggregated statistics for an organization within a time window
type OrgStats struct {
	OrganizationID string
	WindowStart    time.Time
	WindowEnd      time.Time
	WindowType     string

	// Aggregated metrics
	TotalUsers             int
	TotalSessions          int
	TotalCostUSD           float64
	TotalTokens            int64
	TotalActiveTimeSeconds float64

	// Averages
	AvgCostPerUser     float64
	AvgSessionsPerUser float64

	// JSON-encoded top users
	TopUsersByCost  string // JSON array
	TopUsersByUsage string // JSON array

	CreatedAt time.Time
	UpdatedAt time.Time
}

// SessionModelStats represents per-model statistics within a session
type SessionModelStats struct {
	SessionID             string
	Model                 string
	CostUSD               float64
	InputTokens           int64
	OutputTokens          int64
	CacheReadTokens       int64
	CacheCreationTokens   int64
	RequestCount          int
	TotalLatencyMS        float64
	AvgLatencyMS          float64
}

// SessionToolStats represents per-tool statistics within a session
type SessionToolStats struct {
	SessionID       string
	ToolName        string
	ExecutionCount  int
	SuccessCount    int
	FailureCount    int
	TotalDurationMS float64
	AvgDurationMS   float64
	MinDurationMS   float64
	MaxDurationMS   float64
}

// ProcessingState tracks the processing position for each JSONL file
type ProcessingState struct {
	FileName          string
	LastByteOffset    int64 // Byte position in file (for efficient seeking)
	LastProcessedTime time.Time
	FileSizeBytes     int64
	Inode             uint64 // File inode for rotation detection
	UpdatedAt         time.Time
}

// MetricRecord represents a parsed metric from the JSONL file
type MetricRecord struct {
	Timestamp      time.Time
	SessionID      string
	UserID         string
	OrganizationID string
	ServiceName    string
	MetricName     string
	MetricValue    interface{}
	Attributes     map[string]string
}

// LogRecord represents a parsed log from the JSONL file
type LogRecord struct {
	Timestamp      time.Time
	SessionID      string
	UserID         string
	OrganizationID string
	ServiceName    string
	SeverityText   string
	Body           string
	Attributes     map[string]interface{}
}

// TraceRecord represents a parsed trace/span from the JSONL file
type TraceRecord struct {
	Timestamp      time.Time
	SessionID      string
	UserID         string
	OrganizationID string
	ServiceName    string
	SpanName       string
	DurationMS     float64
	Attributes     map[string]string
}

// TimeWindow represents a time range for queries
type TimeWindow struct {
	Start time.Time
	End   time.Time
	Type  string // 'all-time', '7d', '30d', 'custom'
}

// Session represents a session in the new cleaner schema
type Session struct {
	SessionID      string
	OrganizationID string
	UserID         string
	StartTime      time.Time
	EndTime        time.Time

	// Summary stats
	TotalCostUSD             float64
	TotalInputTokens         int64
	TotalOutputTokens        int64
	TotalCacheReadTokens     int64
	TotalCacheCreationTokens int64
	ToolCallCount            int

	CreatedAt time.Time
	UpdatedAt time.Time
}

// SessionTool represents per-tool statistics within a session
type SessionTool struct {
	SessionID            string
	ToolName             string
	CallCount            int
	SuccessCount         int
	FailureCount         int
	TotalExecutionTimeMS float64

	// Decision tracking
	AutoApprovedCount    int
	UserApprovedCount    int
	RejectedCount        int
	TotalResultSizeBytes int64
}

// SessionPrompt represents a user prompt within a session
type SessionPrompt struct {
	ID           int64
	SessionID    string
	PromptText   string
	PromptLength int
	Timestamp    time.Time
}
