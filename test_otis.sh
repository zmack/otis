#!/bin/bash
set -e

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${YELLOW}=== Otis Integration Test ===${NC}\n"

# Check if otis binary exists
if [ ! -f "./otis" ]; then
    echo -e "${RED}Error: otis binary not found. Run 'go build' first.${NC}"
    exit 1
fi

# Clean up old data
echo -e "${YELLOW}Cleaning up old test data...${NC}"
rm -rf ./test_data ./test_db
mkdir -p ./test_data ./test_db

# Start Otis in the background
echo -e "${YELLOW}Starting Otis...${NC}"
export OTIS_PORT=4318
export OTIS_OUTPUT_DIR=./test_data
export OTIS_AGGREGATOR_ENABLED=true
export OTIS_AGGREGATOR_PORT=8080
export OTIS_DB_PATH=./test_db/otis.db
export OTIS_PROCESSING_INTERVAL=2

./otis > otis.log 2>&1 &
OTIS_PID=$!
echo -e "${GREEN}✓ Otis started (PID: $OTIS_PID)${NC}"

# Wait for Otis to start
sleep 3

# Function to cleanup on exit
cleanup() {
    echo -e "\n${YELLOW}Cleaning up...${NC}"
    if [ ! -z "$OTIS_PID" ]; then
        kill $OTIS_PID 2>/dev/null || true
        echo -e "${GREEN}✓ Otis stopped${NC}"
    fi
}
trap cleanup EXIT

# Check if Otis is running
if ! kill -0 $OTIS_PID 2>/dev/null; then
    echo -e "${RED}Error: Otis failed to start. Check otis.log${NC}"
    cat otis.log
    exit 1
fi

# Configure Claude Code to send telemetry to Otis
echo -e "\n${YELLOW}Configuring Claude Code telemetry...${NC}"
export CLAUDE_CODE_ENABLE_TELEMETRY=1
export OTEL_METRICS_EXPORTER=otlp
export OTEL_LOGS_EXPORTER=otlp
export OTEL_EXPORTER_OTLP_PROTOCOL=http/protobuf
export OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318
export OTEL_METRIC_EXPORT_INTERVAL=5000  # 5 seconds for faster testing
export OTEL_LOGS_EXPORT_INTERVAL=2000    # 2 seconds for faster testing
export OTEL_METRICS_INCLUDE_SESSION_ID=true
export OTEL_METRICS_INCLUDE_ACCOUNT_UUID=true
export OTEL_RESOURCE_ATTRIBUTES="environment=test,purpose=otis-validation"

echo -e "${GREEN}✓ Environment configured${NC}"

# Run Claude Code headless sessions to generate telemetry
echo -e "\n${YELLOW}Generating telemetry data with Claude Code...${NC}"
echo -e "${YELLOW}Running 3 test sessions...${NC}"

for i in {1..3}; do
    echo -e "  Session $i..."
    claude -p "What is $i + $i? Just give me the number." > /dev/null 2>&1 || true
    sleep 1
done

echo -e "${GREEN}✓ Test sessions completed${NC}"

# Wait for telemetry to flush and be processed
echo -e "\n${YELLOW}Waiting for telemetry to flush and be processed...${NC}"
sleep 10

# Check JSONL files
echo -e "\n${YELLOW}Checking data collection...${NC}"

check_file() {
    local file=$1
    local name=$2

    if [ -f "$file" ]; then
        local lines=$(wc -l < "$file" | tr -d ' ')
        if [ "$lines" -gt 0 ]; then
            echo -e "${GREEN}✓ $name: $lines lines${NC}"
            return 0
        else
            echo -e "${YELLOW}⚠ $name: file exists but empty${NC}"
            return 1
        fi
    else
        echo -e "${RED}✗ $name: file not found${NC}"
        return 1
    fi
}

METRICS_OK=0
LOGS_OK=0

check_file "./test_data/metrics.jsonl" "Metrics" && METRICS_OK=1 || true
check_file "./test_data/logs.jsonl" "Logs" && LOGS_OK=1 || true

# Show sample data
if [ $METRICS_OK -eq 1 ]; then
    echo -e "\n${YELLOW}Sample metric (first line):${NC}"
    head -n 1 ./test_data/metrics.jsonl | jq . 2>/dev/null || head -n 1 ./test_data/metrics.jsonl
fi

if [ $LOGS_OK -eq 1 ]; then
    echo -e "\n${YELLOW}Sample log (first line):${NC}"
    head -n 1 ./test_data/logs.jsonl | jq . 2>/dev/null || head -n 1 ./test_data/logs.jsonl
fi

# Check aggregator health
echo -e "\n${YELLOW}Checking aggregator API...${NC}"
HEALTH=$(curl -s http://localhost:8080/api/health 2>/dev/null || echo "")

if [ ! -z "$HEALTH" ]; then
    echo -e "${GREEN}✓ Aggregator API responding${NC}"
    echo "$HEALTH" | jq . 2>/dev/null || echo "$HEALTH"
else
    echo -e "${RED}✗ Aggregator API not responding${NC}"
fi

# Check database
echo -e "\n${YELLOW}Checking SQLite database...${NC}"
if [ -f "./test_db/otis.db" ]; then
    SESSION_COUNT=$(sqlite3 ./test_db/otis.db "SELECT COUNT(*) FROM session_stats;" 2>/dev/null || echo "0")
    echo -e "${GREEN}✓ Database exists with $SESSION_COUNT sessions${NC}"

    if [ "$SESSION_COUNT" -gt 0 ]; then
        echo -e "\n${YELLOW}Sample session data:${NC}"
        sqlite3 ./test_db/otis.db "SELECT session_id, user_id, organization_id, total_cost_usd FROM session_stats LIMIT 1;" 2>/dev/null || true
    fi
else
    echo -e "${YELLOW}⚠ Database not yet created${NC}"
fi

# Summary
echo -e "\n${YELLOW}=== Test Summary ===${NC}"

if [ $METRICS_OK -eq 1 ] && [ $LOGS_OK -eq 1 ]; then
    echo -e "${GREEN}✓ SUCCESS: Telemetry data is flowing through Otis!${NC}"
    echo -e "\n${YELLOW}Next steps:${NC}"
    echo "1. Check otis.log for detailed logs"
    echo "2. Explore data files in ./test_data/"
    echo "3. Query aggregator API: curl http://localhost:8080/api/health"
    echo "4. View database: sqlite3 ./test_db/otis.db 'SELECT * FROM session_stats;'"
    exit 0
else
    echo -e "${YELLOW}⚠ PARTIAL: Some telemetry signals missing${NC}"
    echo -e "\n${YELLOW}Troubleshooting:${NC}"
    echo "1. Check if Claude Code is authenticated (required for full telemetry)"
    echo "2. Review otis.log for errors"
    echo "3. Verify OTLP endpoint is reachable: curl http://localhost:4318"
    echo "4. Check Claude Code telemetry: echo \$CLAUDE_CODE_ENABLE_TELEMETRY"
    exit 1
fi
