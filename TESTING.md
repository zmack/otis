# Testing Otis with Claude Code

This guide explains how to test Otis with Claude Code telemetry.

## Quick Test

Run the automated test script:

```bash
./test_otis.sh
```

This will:
1. Start Otis with test-specific directories
2. Configure Claude Code to send telemetry
3. Run 3 test Claude Code sessions
4. Verify data collection and aggregation
5. Show sample data and statistics

## Manual Testing

### 1. Start Otis

```bash
# Use custom directories for testing
export OTIS_PORT=4318
export OTIS_OUTPUT_DIR=./test_data
export OTIS_DB_PATH=./test_db/otis.db
export OTIS_PROCESSING_INTERVAL=2

./otis
```

### 2. Configure Claude Code Telemetry

**IMPORTANT**: Use `http/protobuf` protocol, not `http/json`. Otis currently only supports protobuf format.

```bash
export CLAUDE_CODE_ENABLE_TELEMETRY=1
export OTEL_METRICS_EXPORTER=otlp
export OTEL_LOGS_EXPORTER=otlp
export OTEL_EXPORTER_OTLP_PROTOCOL=http/protobuf  # ← Must be protobuf!
export OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318

# Optional: Faster flush intervals for testing
export OTEL_METRIC_EXPORT_INTERVAL=5000   # 5 seconds (default: 60 seconds)
export OTEL_LOGS_EXPORT_INTERVAL=2000     # 2 seconds (default: 5 seconds)
```

### 3. Run Claude Code

Use the `-p` flag for non-interactive mode:

```bash
claude -p "What is 2+2?"
```

### 4. Wait for Telemetry

Metrics flush every 5 seconds (with the config above), so wait at least 5-10 seconds after Claude finishes.

### 5. Verify Data Collection

**Check JSONL files:**
```bash
ls -lh test_data/
wc -l test_data/*.jsonl
```

**Check database:**
```bash
sqlite3 test_db/otis.db "SELECT COUNT(*) FROM session_stats;"
sqlite3 test_db/otis.db "SELECT session_id, user_id, total_cost_usd FROM session_stats;"
```

**Query API:**
```bash
# Health check
curl http://localhost:8080/api/health | jq .

# Get session ID
SESSION_ID=$(sqlite3 test_db/otis.db "SELECT session_id FROM session_stats LIMIT 1;")

# Query session stats
curl http://localhost:8080/api/stats/session/$SESSION_ID | jq .
```

## Expected Data Structure

### Metrics Data Point Attributes

Claude Code sends these attributes **in the metric data points**, not resource attributes:

- `session.id` - Unique session identifier
- `user.id` - Hashed user identifier
- `organization.id` - Organization UUID
- `user.email` - User email address
- `user.account_uuid` - Account UUID
- `terminal.type` - Terminal emulator name (e.g., "ghostty")
- `model` - Model used (e.g., "claude-sonnet-4-5-20250929")

### Custom Resource Attributes

You can add custom attributes via `OTEL_RESOURCE_ATTRIBUTES`:

```bash
export OTEL_RESOURCE_ATTRIBUTES="environment=test,team=platform,cost_center=eng-123"
```

These appear in resource attributes and are also extracted by the aggregator.

## Troubleshooting

### No telemetry data

1. **Check protocol**: Must use `http/protobuf`, not `http/json`
2. **Check authentication**: Claude Code must be logged in for full telemetry
3. **Check flush interval**: Default is 60 seconds for metrics
4. **Check Otis logs**: Look for "Received and stored metrics data"

### Data not aggregating

1. **Check for session.id**: Otis requires `session.id`, `user.id`, and `organization.id` attributes
2. **Check processor logs**: Look for "Processed X new lines from metrics.jsonl"
3. **Check engine logs**: Look for "Flushed X session stats to database"

### Database empty

1. Wait for flush interval (default 10 seconds)
2. Check that JSONL files have content with required attributes
3. Review aggregator logs for parsing errors

## Implementation Notes

### Fixed: Data Point Attributes

Initially, the aggregator only looked for `session.id`, `user.id`, etc. in **resource attributes**.

Claude Code actually sends these in **metric data point attributes** (inside each data point, not at the resource level).

The fix (in `aggregator/processor.go`):
- Extract attributes from data points
- Merge with resource attributes (data point attributes take precedence)
- Use merged attributes for session tracking

This allows Otis to correctly track Claude Code sessions, users, and costs.
