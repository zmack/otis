# Otis - Open Telemetry Intelligence Station

**OTIS**: **O**pen **T**elemetry **I**ntelligence **S**tation

A lightweight OpenTelemetry Protocol (OTLP) collector and aggregation system built in Go. Otis receives telemetry data (traces, metrics, logs) via OTLP/HTTP, stores it in JSON Lines format, and provides aggregated statistics through a REST API.

## Features

### OTLP Collector
- **OTLP/HTTP Protocol** - Standard port 4318
- **Multi-signal Support** - Traces, metrics, and logs
- **JSON Lines Output** - Human-readable, streamable format
- **Real-time Collection** - Zero-copy streaming to disk

### Aggregation System
- **SQLite Backend** - Embedded database, no external dependencies
- **Pre-computed Aggregates** - Fast query response times
- **Real-time Processing** - Incremental file processing with state tracking
- **Multi-level Analytics** - Session, user, and organization statistics
- **Time Windows** - Support for all-time, 7d, 30d, and custom ranges
- **REST API** - Query aggregated stats via HTTP

## Installation

### Prerequisites
- Go 1.21 or later
- SQLite support (via cgo)

### Build from Source

```bash
git clone <repository-url>
cd otis
go build
```

This creates the `otis` binary.

## Configuration

Otis is configured via environment variables with sensible defaults:

### Collector Settings

| Variable | Default | Description |
|----------|---------|-------------|
| `OTIS_PORT` | `4318` | OTLP/HTTP collector port |
| `OTIS_OUTPUT_DIR` | `./data` | Directory for JSONL output files |
| `OTIS_TRACE_FILE` | `traces.jsonl` | Trace data filename |
| `OTIS_METRIC_FILE` | `metrics.jsonl` | Metrics data filename |
| `OTIS_LOG_FILE` | `logs.jsonl` | Logs data filename |

### Aggregator Settings

| Variable | Default | Description |
|----------|---------|-------------|
| `OTIS_AGGREGATOR_ENABLED` | `true` | Enable/disable aggregator |
| `OTIS_AGGREGATOR_PORT` | `8080` | Aggregation API port |
| `OTIS_DB_PATH` | `./db/otis.db` | SQLite database path |
| `OTIS_PROCESSING_INTERVAL` | `5` | File check interval (seconds) |

### Example Configuration

```bash
# Custom ports and paths
export OTIS_PORT=4318
export OTIS_AGGREGATOR_PORT=8080
export OTIS_DB_PATH=/var/lib/otis/otis.db
export OTIS_OUTPUT_DIR=/var/log/otis

./otis
```

## Usage

### Starting Otis

```bash
./otis
```

On startup, you'll see:
```
Starting OTLP collector on port 4318
Trace endpoint: http://localhost:4318/v1/traces
Metrics endpoint: http://localhost:4318/v1/metrics
Logs endpoint: http://localhost:4318/v1/logs
Output directory: ./data

Starting aggregator...
Starting file processor...
Processed 112 new lines from metrics.jsonl (total: 112)
Starting aggregation API server on port 8080
Endpoints:
  GET http://localhost:8080/api/stats/session/{session_id}
  GET http://localhost:8080/api/stats/user/{user_id}?limit=10
  GET http://localhost:8080/api/stats/org/{org_id}?limit=10
  GET http://localhost:8080/api/health
```

### Sending Telemetry Data

Configure your OpenTelemetry SDK to export to Otis:

```go
// Example: Go SDK configuration
exporter, err := otlptracehttp.New(
    context.Background(),
    otlptracehttp.WithEndpoint("localhost:4318"),
    otlptracehttp.WithInsecure(),
)
```

```javascript
// Example: JavaScript SDK configuration
const exporter = new OTLPTraceExporter({
  url: 'http://localhost:4318/v1/traces',
});
```

## API Reference

### Health Check

```bash
GET /api/health
```

Response:
```json
{
  "status": "ok",
  "timestamp": "2025-12-31T12:00:00Z",
  "service": "otis-aggregator"
}
```

### Session Statistics

Get detailed statistics for a specific session:

```bash
GET /api/stats/session/{session_id}
```

Response:
```json
{
  "session_id": "24a401be-d357-4a1f-931e-da46d18e9fe6",
  "user_id": "user-123",
  "organization_id": "org-456",
  "window": {
    "start": "2025-12-31T08:00:00Z",
    "end": "2025-12-31T09:00:00Z",
    "duration_seconds": 3600
  },
  "costs": {
    "total_usd": 0.0234,
    "by_model": {
      "claude-3-5-sonnet": 0.0234
    }
  },
  "tokens": {
    "total": 15000,
    "input": 10000,
    "output": 3000,
    "cache_read": 2000,
    "cache_creation": 0
  },
  "activity": {
    "api_requests": 12,
    "user_prompts": 5,
    "tools_executed": 25,
    "tools_succeeded": 23,
    "tools_failed": 2,
    "active_time_seconds": 3420
  },
  "performance": {
    "avg_api_latency_ms": 1234.5
  },
  "tools": {
    "Read": 10,
    "Write": 5,
    "Bash": 8,
    "Edit": 2
  },
  "models": ["claude-3-5-sonnet"]
}
```

### User Statistics

Get aggregated statistics for a user across all sessions:

```bash
GET /api/stats/user/{user_id}?limit=10
```

Query Parameters:
- `limit` - Maximum number of sessions to include (default: 10, max: 100)

Response:
```json
{
  "user_id": "user-123",
  "organization_id": "org-456",
  "summary": {
    "total_sessions": 15,
    "first_session": "2025-01-01T10:00:00Z",
    "last_session": "2025-01-15T14:30:00Z",
    "total_active_time_seconds": 54000
  },
  "costs": {
    "total_usd": 12.50,
    "avg_per_session": 0.83
  },
  "tokens": {
    "total": 500000,
    "input": 300000,
    "output": 150000,
    "cache_read": 50000,
    "avg_per_session": 33333
  },
  "activity": {
    "total_api_requests": 180,
    "total_prompts": 75,
    "total_tool_execs": 375,
    "avg_api_per_session": 12
  },
  "models": {
    "claude-3-5-sonnet": 15
  },
  "tools": {
    "Read": 150,
    "Write": 75,
    "Bash": 100
  },
  "sessions": [...]
}
```

### Organization Statistics

Get aggregated statistics for an organization:

```bash
GET /api/stats/org/{org_id}?limit=10
```

Query Parameters:
- `limit` - Maximum number of sessions to include (default: 10, max: 100)

Response:
```json
{
  "organization_id": "org-456",
  "summary": {
    "total_users": 5,
    "total_sessions": 75,
    "first_session": "2025-01-01T08:00:00Z",
    "last_session": "2025-01-15T18:00:00Z",
    "total_active_time_seconds": 270000
  },
  "costs": {
    "total_usd": 62.50,
    "avg_per_session": 0.83,
    "avg_per_user": 12.50
  },
  "tokens": {
    "total": 2500000,
    "avg_per_session": 33333,
    "avg_per_user": 500000
  },
  "sessions": [...]
}
```

## Architecture

```
┌─────────────────────────────────────────────────────────┐
│                    OTLP Exporters                       │
│              (Apps, SDKs, Instrumentation)              │
└─────────────────────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────┐
│                   OTLP Collector                        │
│                    (Port 4318)                          │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐             │
│  │ /v1/traces│  │/v1/metrics│ │ /v1/logs │             │
│  └──────────┘  └──────────┘  └──────────┘             │
└─────────────────────────────────────────────────────────┘
                            │
                            ▼
                  ┌──────────────────┐
                  │  JSONL Files     │
                  │  (data/*.jsonl)  │
                  └──────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────┐
│                 File Processor                          │
│         (Monitors & parses JSONL files)                 │
└─────────────────────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────┐
│               Aggregation Engine                        │
│       (Computes session/user/org stats)                 │
│              In-memory cache + flush                    │
└─────────────────────────────────────────────────────────┘
                            │
                            ▼
                   ┌───────────────┐
                   │ SQLite DB     │
                   │ (otis.db)     │
                   └───────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────┐
│                  HTTP API Server                        │
│                    (Port 8080)                          │
│  GET /api/stats/session/{id}                            │
│  GET /api/stats/user/{id}?limit=10                      │
│  GET /api/stats/org/{id}?limit=10                       │
│  GET /api/health                                        │
└─────────────────────────────────────────────────────────┘
```

## Data Flow

1. **Collection**: OTLP exporters send telemetry to collector endpoints
2. **Storage**: Raw data written to JSONL files (one line per request)
3. **Processing**: File processor reads new lines incrementally
4. **Aggregation**: Engine computes statistics and caches in memory
5. **Persistence**: Periodic flush writes aggregates to SQLite
6. **Querying**: REST API serves pre-computed stats from database

## Development

### Running Tests

```bash
# Run all tests
go test ./...

# Run aggregator tests with verbose output
go test ./aggregator/... -v

# Run tests with coverage
go test ./aggregator/... -cover
```

### Project Structure

```
otis/
├── main.go              # Application entry point
├── config/
│   └── config.go        # Configuration management
├── collector/
│   ├── server.go        # HTTP server
│   ├── traces.go        # Trace handler
│   ├── metrics.go       # Metrics handler
│   └── logs.go          # Logs handler
├── aggregator/
│   ├── models.go        # Data models
│   ├── store.go         # SQLite operations
│   ├── processor.go     # File monitoring & parsing
│   ├── engine.go        # Aggregation logic
│   ├── api.go           # REST API handlers
│   ├── store_test.go    # Store tests
│   └── engine_test.go   # Engine tests
├── data/                # JSONL output directory
└── db/                  # SQLite database directory
```

### Adding New Metrics

To track additional metrics in aggregations:

1. Add fields to `SessionStats` struct in `aggregator/models.go`
2. Update database schema in `aggregator/store.go` (`InitSchema`)
3. Add processing logic in `aggregator/engine.go` (`ProcessMetric` or `ProcessLog`)
4. Update API response builders in `aggregator/api.go`

## Performance Considerations

- **File Processing**: Incremental processing avoids re-reading entire files
- **Caching**: In-memory session cache reduces database writes
- **Periodic Flush**: Default 10-second flush interval balances freshness vs. load
- **SQLite WAL Mode**: Enabled for concurrent reads during writes
- **Prepared Statements**: Used for frequent queries
- **Indexes**: On session_id, user_id, organization_id, and time fields

## Troubleshooting

### Collector not receiving data

- Check that OTLP exporters are configured to send to `http://localhost:4318`
- Verify the collector is running: `curl http://localhost:4318/api/health` (if health endpoint added)
- Check firewall rules allow port 4318

### No aggregated data in API

- Ensure telemetry data includes `session.id`, `user.id`, and `organization.id` resource attributes
- Check file processor logs for errors
- Verify JSONL files exist in the data directory with valid content
- Check database exists: `ls -la db/otis.db`

### High Memory Usage

- Reduce `OTIS_PROCESSING_INTERVAL` to flush cache more frequently
- Limit the number of concurrent sessions
- Consider archiving old JSONL files

## License

[Add your license here]

## Contributing

[Add contribution guidelines here]
