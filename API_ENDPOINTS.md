# Otis Aggregator API Endpoints

## Base URL
`http://localhost:8080`

## Endpoints

### Health Check
```
GET /api/health
```
Returns service status and timestamp.

### Session Stats
```
GET /api/stats/session/{session_id}
```
Returns aggregated statistics for a specific session.

**NEW** - Per-Session Model Breakdown:
```
GET /api/stats/session/{session_id}/models
```
Returns cost, tokens, and latency breakdown by model for a session.

**NEW** - Per-Session Tool Breakdown:
```
GET /api/stats/session/{session_id}/tools
```
Returns execution count, success rate, and duration stats by tool for a session.

### User Stats
```
GET /api/stats/user/{user_id}?limit=10
```
Returns aggregated statistics across all sessions for a user.

### Organization Stats
```
GET /api/stats/org/{org_id}?limit=10
```
Returns aggregated statistics across all sessions for an organization.

### Global Model Analytics (NEW)
```
GET /api/stats/models?limit=50
```
Returns aggregated model statistics across all sessions:
- Total sessions using each model
- Total cost and requests per model
- Token usage breakdown (input, output, cache)
- Average cost per session
- Average latency

### Global Tool Analytics (NEW)
```
GET /api/stats/tools?limit=50
```
Returns aggregated tool statistics across all sessions:
- Total executions per tool
- Success/failure counts and rates
- Average duration
- Number of sessions using each tool

## Example Usage

```bash
# Get model breakdown for a session
curl http://localhost:8080/api/stats/session/abc123/models | jq .

# Get tool performance for a session
curl http://localhost:8080/api/stats/session/abc123/tools | jq .

# Get top 10 models by cost
curl http://localhost:8080/api/stats/models?limit=10 | jq '.models[] | {model, total_cost_usd, total_sessions}'

# Get tool success rates
curl http://localhost:8080/api/stats/tools | jq '.tools[] | {tool_name, success_rate, avg_duration_ms}'
```

## Response Formats

### Per-Session Models
```json
{
  "session_id": "abc123",
  "models": [
    {
      "model": "claude-sonnet-4-5",
      "cost_usd": 0.0042,
      "tokens": {
        "input": 1500,
        "output": 800,
        "cache_read": 500,
        "cache_creation": 200,
        "total": 3000
      },
      "request_count": 5,
      "avg_latency_ms": 1250.5
    }
  ]
}
```

### Per-Session Tools
```json
{
  "session_id": "abc123",
  "tools": [
    {
      "tool_name": "Read",
      "execution_count": 12,
      "success_count": 12,
      "failure_count": 0,
      "duration": {
        "avg_ms": 45.2,
        "min_ms": 12.3,
        "max_ms": 120.8,
        "total_ms": 542.4
      },
      "success_rate": 1.0
    }
  ]
}
```

### Global Models
```json
{
  "models": [
    {
      "model": "claude-sonnet-4-5",
      "total_sessions": 45,
      "total_cost_usd": 0.183,
      "total_requests": 230,
      "total_tokens": {
        "input": 125000,
        "output": 65000,
        "cache_read": 30000,
        "cache_creation": 5000,
        "total": 220000
      },
      "avg_cost_per_session": 0.0041,
      "avg_latency_ms": 1340.2
    }
  ]
}
```

### Global Tools
```json
{
  "tools": [
    {
      "tool_name": "Read",
      "total_executions": 540,
      "total_successes": 538,
      "total_failures": 2,
      "success_rate": 0.996,
      "avg_duration_ms": 42.3,
      "used_in_sessions": 45
    }
  ]
}
```
