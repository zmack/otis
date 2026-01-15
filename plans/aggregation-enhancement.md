# Otis Aggregation Enhancement Plan

**Last Updated:** 2026-01-14
**Current Phase:** Phase 1 (95% complete - only histograms remaining)

## Last Session Summary (2026-01-14)

**What we did:**
1. ✅ Added 4 store query methods to aggregator/store.go:
   - `GetSessionModelStats(sessionID)` - per-session model breakdown
   - `GetSessionToolStats(sessionID)` - per-session tool breakdown
   - `GetAllModelStats(limit)` - global model aggregates
   - `GetAllToolStats(limit)` - global tool aggregates
2. ✅ Added 4 new API endpoints to aggregator/api.go:
   - `GET /api/stats/session/{id}/models` - per-session model stats
   - `GET /api/stats/session/{id}/tools` - per-session tool stats
   - `GET /api/stats/models` - global model analytics
   - `GET /api/stats/tools` - global tool analytics
3. ✅ All 12 unit tests passing
4. ✅ Manual API testing verified all endpoints work correctly

**Phase 1 is essentially COMPLETE!** Only optional histogram/percentile feature remains.

**Ready for next session:**
- Phase 1: Add latency histograms for p50/p95/p99 (optional enhancement)
- Phase 2: Start daily aggregations (user_daily_stats, org_daily_stats)
- Phase 3-5: Time-series, API enhancements, batch jobs

## Quick Status

**What's Working:**
- ✅ Per-model cost/tokens/latency tracking in database
- ✅ Per-tool execution/success/duration tracking in database
- ✅ Engine aggregates data correctly from OTEL events
- ✅ Data flushes every 10s to SQLite tables
- ✅ **NEW:** 4 API endpoints for model/tool analytics
- ✅ **NEW:** Store query methods for aggregated stats
- ✅ **NEW:** Per-session and global breakdowns working

**What's Missing (optional for Phase 1):**
- ⚠️ Latency histograms for percentile calculations (p50, p95, p99)

**Next Actions:**
- Option 1: Move to Phase 2 (daily aggregations)
- Option 2: Complete Phase 1 with histogram/percentile support

---

## Context

User wants comprehensive analytics with all granularities:
- **Goals:** Cost analysis, usage patterns, performance monitoring
- **Granularities:** Per-session, per-user, per-org, time-based trends

## Current State (from exploration)

### Already Implemented
- **Session-level aggregations** (aggregator/engine.go):
  - Cost, tokens (4 types), active time
  - API requests, user prompts, tool executions
  - Models used, tools used (JSON collections)
  - Average API latency

### Data Available But Not Aggregated
1. **Per-model breakdowns** - Cost/tokens by model (data exists in api_request events)
2. **Tool success rates** - Currently just counts, not rates or error analysis
3. **Performance metrics** - Trace spans contain duration data (not processed)
4. **User/Org rollups** - Schema exists but not populated
5. **Time-series data** - No daily/weekly aggregations
6. **Prompt content** - Available with OTEL_LOG_USER_PROMPTS=1

## Available Event Data

**Log Events:**
- `claude_code.user_prompt`: prompt_length, timestamp, prompt (if enabled)
- `claude_code.api_request`: model, tokens (4 types), cost_usd, duration_ms
- `claude_code.tool_decision`: decision, source, tool_name
- `claude_code.tool_result`: tool_name, success, duration_ms, tool_parameters, tool_result_size_bytes

**Metrics:**
- `claude_code.cost.usage`: Cost by model
- `claude_code.token.usage`: Tokens by type and model
- `claude_code.active_time.total`: Active time
- `claude_code.session.count`: Session tracking

## Current Implementation Status (2026-01-11)

### Phase 1: Enhanced Session Aggregations - 95% COMPLETE ✅
- [x] Database schema: `session_model_stats` and `session_tool_stats` tables
- [x] Data models: SessionModelStats, SessionToolStats in models.go
- [x] Per-model cost/token breakdown aggregation in engine.go
- [x] Per-tool success/failure tracking in engine.go
- [x] Flush logic writes to all dimension tables
- [x] Per-model latency tracking (total, avg)
- [x] Per-tool duration tracking (total, avg, min, max)
- [x] **Test coverage: 12 tests passing** (5 new tests added)
  - `TestSessionModelStatsUpsert` - store layer model stats
  - `TestSessionToolStatsUpsert` - store layer tool stats
  - `TestEnginePerModelTracking` - engine tracks per-model metrics
  - `TestEnginePerToolTracking` - engine tracks per-tool metrics
  - `TestEngineMultipleModels` - multiple models in one session
- [x] **Store query methods** (aggregator/store.go:530-708)
  - `GetSessionModelStats(sessionID)` - returns per-model breakdown for a session
  - `GetSessionToolStats(sessionID)` - returns per-tool breakdown for a session
  - `GetAllModelStats(limit)` - returns aggregated model stats across all sessions
  - `GetAllToolStats(limit)` - returns aggregated tool stats across all sessions
- [x] **API endpoints** (aggregator/api.go:450-617)
  - `GET /api/stats/session/{id}/models` - per-session model breakdown
  - `GET /api/stats/session/{id}/tools` - per-session tool breakdown
  - `GET /api/stats/models?limit=50` - global model analytics
  - `GET /api/stats/tools?limit=50` - global tool analytics
- [ ] **OPTIONAL:** Latency histogram for percentile approximation (p50, p95, p99)

### Phase 2: User & Org Aggregations - NOT STARTED
- [ ] User-level rollups from sessions
- [ ] Org-level rollups from users
- [ ] Top users/models/tools queries
- [ ] Daily aggregation tables (different from current user_stats/org_stats schemas)

### Phase 3: Time-Series Aggregations - NOT STARTED
- [ ] Daily aggregations
- [ ] Weekly aggregations
- [ ] Monthly aggregations
- [ ] Retention and archival strategy

### Phase 4: API Enhancements - 60% COMPLETE
- [x] Basic endpoints: /api/stats/session, /api/stats/user, /api/stats/org
- [x] GET /api/stats/models - model breakdown across sessions ✅
- [x] GET /api/stats/tools - tool performance analytics ✅
- [x] GET /api/stats/session/{id}/models - per-session model breakdown ✅
- [x] GET /api/stats/session/{id}/tools - per-session tool breakdown ✅
- [ ] **TODO:** Time window support (?window=7d, ?start=X&end=Y)
- [ ] **TODO:** Error analytics endpoints

## Critical Files

- `aggregator/models.go` - Data structures
- `aggregator/engine.go` - Aggregation logic
- `aggregator/store.go` - SQLite persistence
- `aggregator/processor.go` - Event processing
- `aggregator/api.go` - REST API endpoints

## Recommended Approach: Hybrid Schema with Incremental Aggregation

### Architecture Decision

**Hybrid approach** combining:
- Normalized dimension tables (`session_model_stats`, `session_tool_stats`)
- Pre-computed daily rollups (`user_daily_stats`, `org_daily_stats`)
- Histogram-based percentile approximation
- Real-time incremental updates with nightly reconciliation

**Rationale:**
- Balance query performance (fast dashboard queries) with maintainability
- SessionStats remains source of truth, other tables are derived/denormalized
- Incremental updates keep data fresh without batch job delays
- Fixed-size histograms provide good percentile approximation

### New Database Tables

```sql
-- Per-session model breakdown (normalized)
session_model_stats (
    session_id, model, cost_usd, input_tokens, output_tokens,
    cache_read_tokens, cache_creation_tokens, request_count,
    total_latency_ms, avg_latency_ms
    PRIMARY KEY (session_id, model)
)

-- Per-session tool breakdown (normalized)
session_tool_stats (
    session_id, tool_name, execution_count, success_count, failure_count,
    total_duration_ms, avg_duration_ms, min_duration_ms, max_duration_ms
    PRIMARY KEY (session_id, tool_name)
)

-- Latency histogram for percentile approximation
session_latency_histogram (
    session_id, metric_type, bucket_upper_ms, count
    PRIMARY KEY (session_id, metric_type, bucket_upper_ms)
    -- Buckets: [50, 100, 250, 500, 1000, 2500, 5000, 10000, ∞]
)

-- Daily user rollup
user_daily_stats (
    user_id, organization_id, date,
    session_count, total_cost_usd, total_tokens, total_active_time_seconds,
    api_request_count, tool_execution_count, tool_success_count,
    models_breakdown TEXT, -- JSON: {model: {cost, tokens, requests}}
    tools_breakdown TEXT,  -- JSON: {tool: {count, success, failed}}
    PRIMARY KEY (user_id, date)
)

-- Daily org rollup
org_daily_stats (
    organization_id, date,
    user_count, session_count, total_cost_usd, total_tokens,
    top_users_by_cost TEXT, -- JSON: [{user_id, cost}]
    models_breakdown TEXT, tools_breakdown TEXT,
    PRIMARY KEY (organization_id, date)
)
```

### Implementation Phases - Progress Tracking

**Phase 1: Enhanced Session Tracking** - 80% DONE ✅
- ✅ Added `session_model_stats` and `session_tool_stats` tables (store.go:145-178)
- ✅ Track per-model costs/tokens in ProcessMetric (engine.go:134-177)
- ✅ Track per-tool performance in ProcessLog (engine.go:264-288)
- ✅ Updated Engine.FlushCache() to write dimension tables (engine.go:57-79)
- ❌ **REMAINING:** Add store query methods and API endpoints (see Next Steps)
- ❌ **REMAINING:** Add latency histogram table and percentile calculation

**Phase 2: Daily Aggregations** - NOT STARTED
- ❌ Add `user_daily_stats` and `org_daily_stats` tables
- ❌ Implement UpdateDailyStats() called during flush
- ❌ Use UPSERT pattern for idempotent updates
- ❌ Add API endpoints for time-series queries

**Phase 3: Percentile Support** - NOT STARTED
- ❌ Add `session_latency_histogram` table
- ❌ Implement histogram bucket tracking (9 buckets)
- ❌ Add percentile calculation functions (p50, p95, p99)
- ❌ Include percentiles in API responses

**Phase 4: API Enhancements** - 20% DONE
- ✅ Basic endpoints exist: /api/stats/session, /api/stats/user, /api/stats/org (api.go)
- ❌ GET /api/stats/models - model breakdown across sessions
- ❌ GET /api/stats/tools - tool performance analytics
- ❌ GET /api/stats/user/{id}?window=7d - time window support
- ❌ GET /api/stats/org/{id}/trends - time-series data

**Phase 5: Batch Jobs & Optimization** - NOT STARTED
- ❌ Nightly reconciliation job (verify daily aggregates)
- ❌ Weekly/monthly rollup tables
- ❌ Data archival strategy (>90 days)
- ❌ Query performance optimization

### Migration Strategy

1. Add schema versioning table
2. Incremental migrations v1→v2→v3→v4→v5
3. Backfill script to rebuild aggregates from existing SessionStats
4. Zero-downtime deployment (new tables coexist with old)

### Key Trade-offs

**Write Amplification**
- Each session flush writes to 5-6 tables (vs. 1 currently)
- Mitigation: Use transactions, keep 10s flush interval

**Storage Growth**
- Daily tables add ~20% storage overhead
- Mitigation: Archive old data, partition by time

**Data Consistency**
- Multiple derived tables risk inconsistency
- Mitigation: SessionStats is source of truth, nightly reconciliation

**Query Performance**
- Pre-computed aggregates enable fast dashboard queries
- Trade: More complex writes for faster reads (OLAP pattern)

### Success Metrics

- Session stats queries: < 50ms
- User daily stats: < 100ms
- Org monthly stats: < 200ms
- Data accuracy: 100% match on reconciliation
- Percentile accuracy: within 5% of exact

## Verification & Testing

### Phase 1 Verification (Session-Level Breakdowns)

**Setup:**
```bash
# Update test script to enable prompt logging
export OTEL_LOG_USER_PROMPTS=1
./test_otis.sh
```

**Tests:**
1. Run 3 sessions with different models (vary by using different prompts)
2. Check `session_model_stats` table:
   ```sql
   SELECT session_id, model, cost_usd, request_count FROM session_model_stats;
   ```
3. Verify sum of per-model costs equals SessionStats.total_cost_usd
4. Check `session_tool_stats` for tool breakdown:
   ```sql
   SELECT tool_name, execution_count, success_count, avg_duration_ms
   FROM session_tool_stats WHERE session_id = ?;
   ```

### Phase 2 Verification (Daily Aggregations)

**Tests:**
1. Run sessions across multiple days (manipulate timestamps or wait)
2. Query daily stats:
   ```sql
   SELECT date, session_count, total_cost_usd FROM user_daily_stats
   WHERE user_id = ? ORDER BY date;
   ```
3. Verify daily totals match SessionStats for that day
4. Test time window queries via API:
   ```bash
   curl http://localhost:8080/api/stats/user/{id}?window=7d
   curl http://localhost:8080/api/stats/user/{id}?window=30d
   ```

### Phase 3 Verification (Percentiles)

**Tests:**
1. Generate sessions with varying API latencies
2. Check histogram buckets:
   ```sql
   SELECT bucket_upper_ms, count FROM session_latency_histogram
   WHERE session_id = ? AND metric_type = 'api_latency';
   ```
3. Compare calculated percentiles with actual latencies
4. Verify API response includes p50, p95, p99

### End-to-End Testing

**Complete workflow:**
```bash
# 1. Clean slate
rm -rf test_data test_db

# 2. Start Otis
./otis &

# 3. Generate diverse telemetry
export OTEL_LOG_USER_PROMPTS=1
for i in {1..10}; do
  claude -p "Test session $i"
  sleep 2
done

# 4. Wait for flush
sleep 15

# 5. Verify all tables
sqlite3 test_db/otis.db <<EOF
SELECT COUNT(*) FROM session_stats;
SELECT COUNT(*) FROM session_model_stats;
SELECT COUNT(*) FROM session_tool_stats;
SELECT COUNT(*) FROM user_daily_stats;
SELECT COUNT(*) FROM org_daily_stats;
EOF

# 6. Query APIs
curl http://localhost:8080/api/stats/models | jq .
curl http://localhost:8080/api/stats/tools | jq .
```

### Reconciliation Testing

**Nightly job verification:**
```bash
# Run reconciliation
go run cmd/reconcile/main.go --date=2026-01-11

# Check for discrepancies
sqlite3 test_db/otis.db <<EOF
SELECT
  user_id, date,
  session_count as daily_count,
  (SELECT COUNT(*) FROM session_stats WHERE user_id = uds.user_id
   AND DATE(start_time, 'unixepoch') = uds.date) as actual_count
FROM user_daily_stats uds
WHERE daily_count != actual_count;
EOF
```

---

## Critical Files to Modify

1. **aggregator/models.go** - Add new structs (ModelStats, ToolStats, HistogramBucket, UserDailyStats, OrgDailyStats)
2. **aggregator/store.go** - New tables, UPSERT methods, migration logic
3. **aggregator/engine.go** - Enhanced ProcessMetric/ProcessLog, updated FlushCache
4. **aggregator/api.go** - New endpoints for models, tools, time-series
5. **aggregator/processor.go** - Extract per-model attributes from metrics/logs

---

## Next Steps

### Immediate (Complete Phase 1):

1. **Add store query methods** (aggregator/store.go):
   - `GetSessionModelStats(sessionID string) ([]*SessionModelStats, error)`
   - `GetSessionToolStats(sessionID string) ([]*SessionToolStats, error)`
   - `GetAllModelStats(limit int) (map[string]*ModelAggregates, error)` - aggregate across all sessions
   - `GetAllToolStats(limit int) (map[string]*ToolAggregates, error)` - aggregate across all sessions

2. **Add API endpoints** (aggregator/api.go):
   - `GET /api/stats/session/{id}/models` - per-session model breakdown
   - `GET /api/stats/session/{id}/tools` - per-session tool breakdown
   - `GET /api/stats/models` - global model analytics
   - `GET /api/stats/tools` - global tool analytics

3. **Test Phase 1 completion**:
   - Run test_otis.sh to generate telemetry
   - Query new endpoints and verify data
   - Update TESTING.md with verification steps

### Later (Phase 2-5):

4. Add histogram tables and percentile calculations (Phase 1 completion)
5. Implement daily aggregations (Phase 2)
6. Add time-series endpoints (Phase 3-4)
7. Build reconciliation jobs (Phase 5)

---

## Example API Responses (Target for Phase 1 Completion)

### GET /api/stats/session/{id}/models
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
        "cache_creation": 200
      },
      "request_count": 5,
      "avg_latency_ms": 1250.5
    }
  ]
}
```

### GET /api/stats/session/{id}/tools
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
    },
    {
      "tool_name": "Bash",
      "execution_count": 3,
      "success_count": 2,
      "failure_count": 1,
      "duration": {
        "avg_ms": 250.5,
        "min_ms": 100.2,
        "max_ms": 500.1,
        "total_ms": 751.5
      },
      "success_rate": 0.67
    }
  ]
}
```

### GET /api/stats/models
```json
{
  "models": [
    {
      "model": "claude-sonnet-4-5",
      "total_sessions": 45,
      "total_cost_usd": 0.183,
      "total_requests": 230,
      "total_tokens": 125000,
      "avg_cost_per_session": 0.0041,
      "avg_latency_ms": 1340.2
    },
    {
      "model": "claude-opus-4-5",
      "total_sessions": 3,
      "total_cost_usd": 0.025,
      "total_requests": 8,
      "total_tokens": 8500,
      "avg_cost_per_session": 0.0083,
      "avg_latency_ms": 2100.5
    }
  ]
}
```

### GET /api/stats/tools
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
    },
    {
      "tool_name": "Bash",
      "total_executions": 85,
      "total_successes": 80,
      "total_failures": 5,
      "success_rate": 0.941,
      "avg_duration_ms": 320.5,
      "used_in_sessions": 32
    }
  ]
}
```
