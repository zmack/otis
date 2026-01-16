package aggregator

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Processor struct {
	dataDir  string
	store    *Store
	engine   *Engine
	interval time.Duration
	stopChan chan bool
}

// NewProcessor creates a new file processor
func NewProcessor(dataDir string, store *Store, engine *Engine, intervalSeconds int) *Processor {
	return &Processor{
		dataDir:  dataDir,
		store:    store,
		engine:   engine,
		interval: time.Duration(intervalSeconds) * time.Second,
		stopChan: make(chan bool),
	}
}

// Start begins monitoring and processing files
func (p *Processor) Start() {
	log.Println("Starting file processor...")

	// Process existing data once at startup
	p.processAllFiles()

	// Then monitor for changes
	ticker := time.NewTicker(p.interval)
	go func() {
		for {
			select {
			case <-ticker.C:
				p.processAllFiles()
			case <-p.stopChan:
				ticker.Stop()
				log.Println("File processor stopped")
				return
			}
		}
	}()
}

// Stop stops the file processor
func (p *Processor) Stop() {
	close(p.stopChan)
}

// processAllFiles processes all JSONL files in the data directory
func (p *Processor) processAllFiles() {
	files := []string{"metrics.jsonl", "logs.jsonl", "traces.jsonl"}

	for _, filename := range files {
		filePath := filepath.Join(p.dataDir, filename)
		if err := p.ProcessFile(filePath); err != nil {
			log.Printf("Error processing %s: %v", filename, err)
		}
	}
}

// ProcessFile processes new lines from a specific file
func (p *Processor) ProcessFile(filePath string) error {
	// Get file info
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // File doesn't exist yet, skip
		}
		return fmt.Errorf("failed to stat file: %w", err)
	}

	filename := filepath.Base(filePath)

	// Get processing state
	state, err := p.store.GetProcessingState(filename)
	if err != nil {
		return fmt.Errorf("failed to get processing state: %w", err)
	}

	// Detect file rotation/truncation (file size decreased)
	if fileInfo.Size() < state.FileSizeBytes {
		log.Printf("File %s was rotated or truncated (size decreased from %d to %d), resetting position",
			filename, state.FileSizeBytes, fileInfo.Size())
		state.LastByteOffset = 0
		state.FileSizeBytes = 0
	}

	// Check if file has new data
	if fileInfo.Size() <= state.FileSizeBytes {
		return nil // No new data
	}

	// Open file
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Seek to last processed position (PERFORMANCE FIX!)
	_, err = file.Seek(state.LastByteOffset, 0)
	if err != nil {
		return fmt.Errorf("failed to seek to position %d: %w", state.LastByteOffset, err)
	}

	scanner := bufio.NewScanner(file)
	newLinesProcessed := 0
	currentOffset := state.LastByteOffset

	// Process new lines (starting from where we left off)
	for scanner.Scan() {
		line := scanner.Text()

		if strings.TrimSpace(line) == "" {
			// Track offset even for empty lines
			currentOffset += int64(len(line) + 1) // +1 for newline
			continue
		}

		if err := p.processLine(filename, line); err != nil {
			log.Printf("Error processing line in %s at offset %d: %v", filename, currentOffset, err)
			// Continue processing even on error
		}

		newLinesProcessed++
		currentOffset += int64(len(line) + 1) // +1 for newline

		// Update processing state periodically (every 100 lines)
		if newLinesProcessed%100 == 0 {
			if err := p.store.UpdateProcessingState(filename, currentOffset, fileInfo.Size()); err != nil {
				log.Printf("Error updating processing state: %v", err)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading file: %w", err)
	}

	// Final state update
	if newLinesProcessed > 0 {
		if err := p.store.UpdateProcessingState(filename, currentOffset, fileInfo.Size()); err != nil {
			return fmt.Errorf("failed to update processing state: %w", err)
		}
		log.Printf("Processed %d new lines from %s (now at byte offset %d)", newLinesProcessed, filename, currentOffset)
	}

	return nil
}

// processLine processes a single JSONL line
func (p *Processor) processLine(filename, line string) error {
	// Parse the wrapper object that contains "data" field
	var wrapper map[string]interface{}
	if err := json.Unmarshal([]byte(line), &wrapper); err != nil {
		return fmt.Errorf("failed to unmarshal wrapper: %w", err)
	}

	// Get the "data" field which contains the JSON string
	dataStr, ok := wrapper["data"].(string)
	if !ok {
		return fmt.Errorf("no 'data' field found in wrapper")
	}

	// Parse the actual data
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(dataStr), &data); err != nil {
		return fmt.Errorf("failed to unmarshal data: %w", err)
	}

	// Route to appropriate handler based on filename
	switch filename {
	case "metrics.jsonl":
		return p.processMetricData(data)
	case "logs.jsonl":
		return p.processLogData(data)
	case "traces.jsonl":
		return p.processTraceData(data)
	default:
		return fmt.Errorf("unknown file type: %s", filename)
	}
}

// processMetricData processes metric data
func (p *Processor) processMetricData(data map[string]interface{}) error {
	// Extract resource metrics
	resourceMetrics, ok := data["resourceMetrics"].([]interface{})
	if !ok {
		return nil
	}

	for _, rm := range resourceMetrics {
		rmMap, ok := rm.(map[string]interface{})
		if !ok {
			continue
		}

		// Extract resource attributes
		attrs := extractResourceAttributes(rmMap)

		// Extract scope metrics
		scopeMetrics, ok := rmMap["scopeMetrics"].([]interface{})
		if !ok {
			continue
		}

		for _, sm := range scopeMetrics {
			smMap, ok := sm.(map[string]interface{})
			if !ok {
				continue
			}

			// Extract metrics
			metrics, ok := smMap["metrics"].([]interface{})
			if !ok {
				continue
			}

			for _, m := range metrics {
				mMap, ok := m.(map[string]interface{})
				if !ok {
					continue
				}

				// Extract all data points from this metric
				records := extractMetricRecords(mMap, attrs)
				for _, record := range records {
					p.engine.ProcessMetric(record)
				}
			}
		}
	}

	return nil
}

// processLogData processes log data
func (p *Processor) processLogData(data map[string]interface{}) error {
	// Extract resource logs
	resourceLogs, ok := data["resourceLogs"].([]interface{})
	if !ok {
		return nil
	}

	for _, rl := range resourceLogs {
		rlMap, ok := rl.(map[string]interface{})
		if !ok {
			continue
		}

		// Extract resource attributes
		attrs := extractResourceAttributes(rlMap)

		// Extract scope logs
		scopeLogs, ok := rlMap["scopeLogs"].([]interface{})
		if !ok {
			continue
		}

		for _, sl := range scopeLogs {
			slMap, ok := sl.(map[string]interface{})
			if !ok {
				continue
			}

			// Extract log records
			logRecords, ok := slMap["logRecords"].([]interface{})
			if !ok {
				continue
			}

			for _, lr := range logRecords {
				lrMap, ok := lr.(map[string]interface{})
				if !ok {
					continue
				}

				record := extractLogRecord(lrMap, attrs)
				if record != nil {
					p.engine.ProcessLog(record)
				}
			}
		}
	}

	return nil
}

// processTraceData processes trace data
func (p *Processor) processTraceData(data map[string]interface{}) error {
	// Extract resource spans
	resourceSpans, ok := data["resourceSpans"].([]interface{})
	if !ok {
		return nil
	}

	for _, rs := range resourceSpans {
		rsMap, ok := rs.(map[string]interface{})
		if !ok {
			continue
		}

		// Extract resource attributes
		attrs := extractResourceAttributes(rsMap)

		// Extract scope spans
		scopeSpans, ok := rsMap["scopeSpans"].([]interface{})
		if !ok {
			continue
		}

		for _, ss := range scopeSpans {
			ssMap, ok := ss.(map[string]interface{})
			if !ok {
				continue
			}

			// Extract spans
			spans, ok := ssMap["spans"].([]interface{})
			if !ok {
				continue
			}

			for _, s := range spans {
				sMap, ok := s.(map[string]interface{})
				if !ok {
					continue
				}

				record := extractTraceRecord(sMap, attrs)
				if record != nil {
					p.engine.ProcessTrace(record)
				}
			}
		}
	}

	return nil
}

// Helper functions to extract data from OTLP structures

func extractResourceAttributes(resourceMap map[string]interface{}) map[string]string {
	attrs := make(map[string]string)

	resource, ok := resourceMap["resource"].(map[string]interface{})
	if !ok {
		return attrs
	}

	attributes, ok := resource["attributes"].([]interface{})
	if !ok {
		return attrs
	}

	for _, attr := range attributes {
		attrMap, ok := attr.(map[string]interface{})
		if !ok {
			continue
		}

		key, _ := attrMap["key"].(string)
		valueMap, ok := attrMap["value"].(map[string]interface{})
		if !ok {
			continue
		}

		// Extract string value (could be enhanced to handle other types)
		if strValue, ok := valueMap["stringValue"].(string); ok {
			attrs[key] = strValue
		}
	}

	return attrs
}

func extractMetricRecords(metricMap map[string]interface{}, resourceAttrs map[string]string) []*MetricRecord {
	name, _ := metricMap["name"].(string)
	if name == "" {
		return nil
	}

	var records []*MetricRecord

	// Try to extract from sum
	if sum, ok := metricMap["sum"].(map[string]interface{}); ok {
		if dataPoints, ok := sum["dataPoints"].([]interface{}); ok {
			// Process ALL data points (important for metrics like token.usage which have multiple points)
			for _, dpInterface := range dataPoints {
				dp, ok := dpInterface.(map[string]interface{})
				if !ok {
					continue
				}

				var timestamp time.Time
				var value interface{}
				dataPointAttrs := make(map[string]string)

				// Extract data point attributes (session.id, user.id, etc. are here in Claude Code metrics)
				if attributes, ok := dp["attributes"].([]interface{}); ok {
					for _, attr := range attributes {
						attrMap, ok := attr.(map[string]interface{})
						if !ok {
							continue
						}
						key, _ := attrMap["key"].(string)
						valueMap, ok := attrMap["value"].(map[string]interface{})
						if !ok {
							continue
						}
						if strValue, ok := valueMap["stringValue"].(string); ok {
							dataPointAttrs[key] = strValue
						}
					}
				}

				if timeStr, ok := dp["timeUnixNano"].(string); ok {
					// Parse nanoseconds timestamp
					var nanos int64
					fmt.Sscanf(timeStr, "%d", &nanos)
					timestamp = time.Unix(0, nanos)
				}
				if asInt, ok := dp["asInt"].(string); ok {
					var intVal int64
					fmt.Sscanf(asInt, "%d", &intVal)
					value = intVal
				} else if asDouble, ok := dp["asDouble"].(float64); ok {
					value = asDouble
				}

				// Merge resource attrs and data point attrs, with data point taking precedence
				allAttrs := make(map[string]string)
				for k, v := range resourceAttrs {
					allAttrs[k] = v
				}
				for k, v := range dataPointAttrs {
					allAttrs[k] = v
				}

				records = append(records, &MetricRecord{
					Timestamp:      timestamp,
					SessionID:      allAttrs["session.id"],
					UserID:         allAttrs["user.id"],
					OrganizationID: allAttrs["organization.id"],
					ServiceName:    allAttrs["service.name"],
					MetricName:     name,
					MetricValue:    value,
					Attributes:     allAttrs,
				})
			}
		}
	}

	return records
}

func extractLogRecord(logMap map[string]interface{}, resourceAttrs map[string]string) *LogRecord {
	var timestamp time.Time
	if timeStr, ok := logMap["timeUnixNano"].(string); ok {
		var nanos int64
		fmt.Sscanf(timeStr, "%d", &nanos)
		timestamp = time.Unix(0, nanos)
	}

	severityText, _ := logMap["severityText"].(string)

	var body string
	if bodyMap, ok := logMap["body"].(map[string]interface{}); ok {
		body, _ = bodyMap["stringValue"].(string)
	}

	// Extract log attributes
	logAttrs := make(map[string]interface{})
	if attributes, ok := logMap["attributes"].([]interface{}); ok {
		for _, attr := range attributes {
			attrMap, ok := attr.(map[string]interface{})
			if !ok {
				continue
			}
			key, _ := attrMap["key"].(string)
			if valueMap, ok := attrMap["value"].(map[string]interface{}); ok {
				// Store the whole value map
				logAttrs[key] = valueMap
			}
		}
	}

	return &LogRecord{
		Timestamp:      timestamp,
		SessionID:      resourceAttrs["session.id"],
		UserID:         resourceAttrs["user.id"],
		OrganizationID: resourceAttrs["organization.id"],
		ServiceName:    resourceAttrs["service.name"],
		SeverityText:   severityText,
		Body:           body,
		Attributes:     logAttrs,
	}
}

func extractTraceRecord(spanMap map[string]interface{}, resourceAttrs map[string]string) *TraceRecord {
	name, _ := spanMap["name"].(string)
	if name == "" {
		return nil
	}

	var timestamp time.Time
	var durationMS float64

	if startTimeStr, ok := spanMap["startTimeUnixNano"].(string); ok {
		var nanos int64
		fmt.Sscanf(startTimeStr, "%d", &nanos)
		timestamp = time.Unix(0, nanos)
	}

	if endTimeStr, ok := spanMap["endTimeUnixNano"].(string); ok {
		var startNanos, endNanos int64
		if startTimeStr, ok := spanMap["startTimeUnixNano"].(string); ok {
			fmt.Sscanf(startTimeStr, "%d", &startNanos)
			fmt.Sscanf(endTimeStr, "%d", &endNanos)
			durationMS = float64(endNanos-startNanos) / 1e6 // Convert to milliseconds
		}
	}

	return &TraceRecord{
		Timestamp:      timestamp,
		SessionID:      resourceAttrs["session.id"],
		UserID:         resourceAttrs["user.id"],
		OrganizationID: resourceAttrs["organization.id"],
		ServiceName:    resourceAttrs["service.name"],
		SpanName:       name,
		DurationMS:     durationMS,
		Attributes:     resourceAttrs,
	}
}
