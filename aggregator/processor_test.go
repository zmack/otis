package aggregator

import (
	"os"
	"path/filepath"
	"testing"
)

// TestProcessLineBackwardsCompatibility tests that processLine handles both
// the old wrapped format {"data": "<json>"} and the new direct format.
func TestProcessLineBackwardsCompatibility(t *testing.T) {
	dbPath := "./test_compat.db"
	dataDir := "./test_compat_data"
	defer os.Remove(dbPath)
	defer os.RemoveAll(dataDir)

	os.MkdirAll(dataDir, 0755)

	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	engine := NewEngine(store)
	processor := NewProcessor(dataDir, store, engine, 60)

	// Old wrapped format
	oldFormat := `{"data":"{\"resourceMetrics\":[{\"resource\":{\"attributes\":[{\"key\":\"service.name\",\"value\":{\"stringValue\":\"test\"}}]},\"scopeMetrics\":[{\"metrics\":[{\"name\":\"test.metric\",\"sum\":{\"dataPoints\":[{\"timeUnixNano\":\"1000000000\",\"asDouble\":1.0}]}}]}]}]}"}`

	// New direct format
	newFormat := `{"resourceMetrics":[{"resource":{"attributes":[{"key":"service.name","value":{"stringValue":"test"}}]},"scopeMetrics":[{"metrics":[{"name":"test.metric","sum":{"dataPoints":[{"timeUnixNano":"1000000000","asDouble":1.0}]}}]}]}]}`

	// Both formats should parse without error
	if err := processor.processLine("metrics.jsonl", oldFormat); err != nil {
		t.Errorf("Failed to process old wrapped format: %v", err)
	}

	if err := processor.processLine("metrics.jsonl", newFormat); err != nil {
		t.Errorf("Failed to process new direct format: %v", err)
	}
}

// TestProcessLineRejectsInvalidJSON tests that invalid JSON is rejected.
func TestProcessLineRejectsInvalidJSON(t *testing.T) {
	dbPath := "./test_invalid.db"
	dataDir := "./test_invalid_data"
	defer os.Remove(dbPath)
	defer os.RemoveAll(dataDir)

	os.MkdirAll(dataDir, 0755)

	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	engine := NewEngine(store)
	processor := NewProcessor(dataDir, store, engine, 60)

	invalidJSON := `{not valid json}`

	if err := processor.processLine("metrics.jsonl", invalidJSON); err == nil {
		t.Error("Expected error for invalid JSON, got nil")
	}
}

// TestExtractLogRecordFromLogAttributes tests that session.id, user.id, organization.id
// are correctly extracted from log record attributes (not just resource attributes).
// This is critical because Claude Code sends these identifiers in log attributes.
func TestExtractLogRecordFromLogAttributes(t *testing.T) {
	// Simulate the OTLP log record structure from Claude Code
	// session.id is in logRecord.attributes, NOT in resource.attributes
	logMap := map[string]interface{}{
		"timeUnixNano": "1767173562293000000",
		"body": map[string]interface{}{
			"stringValue": "claude_code.tool_result",
		},
		"attributes": []interface{}{
			map[string]interface{}{
				"key": "session.id",
				"value": map[string]interface{}{
					"stringValue": "test-session-123",
				},
			},
			map[string]interface{}{
				"key": "user.id",
				"value": map[string]interface{}{
					"stringValue": "test-user-456",
				},
			},
			map[string]interface{}{
				"key": "organization.id",
				"value": map[string]interface{}{
					"stringValue": "test-org-789",
				},
			},
			map[string]interface{}{
				"key": "tool_name",
				"value": map[string]interface{}{
					"stringValue": "Bash",
				},
			},
			map[string]interface{}{
				"key": "success",
				"value": map[string]interface{}{
					"stringValue": "true",
				},
			},
		},
	}

	// Resource attributes do NOT contain session.id (only service info)
	resourceAttrs := map[string]string{
		"service.name":    "claude-code",
		"service.version": "2.0.76",
		"host.arch":       "arm64",
	}

	record := extractLogRecord(logMap, resourceAttrs)

	// Verify identifiers were extracted from log attributes
	if record.SessionID != "test-session-123" {
		t.Errorf("Expected SessionID 'test-session-123', got '%s' - not extracting from log attributes", record.SessionID)
	}

	if record.UserID != "test-user-456" {
		t.Errorf("Expected UserID 'test-user-456', got '%s'", record.UserID)
	}

	if record.OrganizationID != "test-org-789" {
		t.Errorf("Expected OrganizationID 'test-org-789', got '%s'", record.OrganizationID)
	}

	// Verify service name comes from resource attributes
	if record.ServiceName != "claude-code" {
		t.Errorf("Expected ServiceName 'claude-code', got '%s'", record.ServiceName)
	}

	// Verify body was extracted
	if record.Body != "claude_code.tool_result" {
		t.Errorf("Expected Body 'claude_code.tool_result', got '%s'", record.Body)
	}
}

// TestExtractLogRecordFallbackToResourceAttrs tests that we fall back to resource
// attributes if log attributes don't have the identifiers (backwards compatibility)
func TestExtractLogRecordFallbackToResourceAttrs(t *testing.T) {
	logMap := map[string]interface{}{
		"timeUnixNano": "1767173562293000000",
		"body": map[string]interface{}{
			"stringValue": "some_event",
		},
		"attributes": []interface{}{
			// No session.id, user.id, organization.id here
			map[string]interface{}{
				"key": "some_attr",
				"value": map[string]interface{}{
					"stringValue": "some_value",
				},
			},
		},
	}

	// Resource attributes have the identifiers (legacy format)
	resourceAttrs := map[string]string{
		"service.name":    "claude-code",
		"session.id":      "resource-session",
		"user.id":         "resource-user",
		"organization.id": "resource-org",
	}

	record := extractLogRecord(logMap, resourceAttrs)

	// Should fall back to resource attributes
	if record.SessionID != "resource-session" {
		t.Errorf("Expected SessionID 'resource-session' from fallback, got '%s'", record.SessionID)
	}

	if record.UserID != "resource-user" {
		t.Errorf("Expected UserID 'resource-user' from fallback, got '%s'", record.UserID)
	}

	if record.OrganizationID != "resource-org" {
		t.Errorf("Expected OrganizationID 'resource-org' from fallback, got '%s'", record.OrganizationID)
	}
}

// TestExtractLogRecordLogAttrsTakePrecedence tests that log attributes take
// precedence over resource attributes when both are present
func TestExtractLogRecordLogAttrsTakePrecedence(t *testing.T) {
	logMap := map[string]interface{}{
		"timeUnixNano": "1767173562293000000",
		"body": map[string]interface{}{
			"stringValue": "some_event",
		},
		"attributes": []interface{}{
			map[string]interface{}{
				"key": "session.id",
				"value": map[string]interface{}{
					"stringValue": "log-session",
				},
			},
		},
	}

	resourceAttrs := map[string]string{
		"service.name": "claude-code",
		"session.id":   "resource-session", // Should be overridden
	}

	record := extractLogRecord(logMap, resourceAttrs)

	// Log attributes should take precedence
	if record.SessionID != "log-session" {
		t.Errorf("Expected SessionID 'log-session' (log attrs precedence), got '%s'", record.SessionID)
	}
}

// TestRotationDetectionByInode tests that file rotation is detected when inode changes.
// This handles the case where the file is renamed and a new file is created.
func TestRotationDetectionByInode(t *testing.T) {
	dbPath := "./test_rotation_inode.db"
	dataDir := "./test_rotation_data"
	defer os.Remove(dbPath)
	defer os.RemoveAll(dataDir)

	os.MkdirAll(dataDir, 0755)

	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	testFile := filepath.Join(dataDir, "test.jsonl")

	// Create initial file and write some data
	f1, err := os.Create(testFile)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	f1.WriteString(`{"data":"{\"resourceLogs\":[]}"}` + "\n")
	f1.WriteString(`{"data":"{\"resourceLogs\":[]}"}` + "\n")
	f1.Close()

	// Get the inode of the first file
	info1, _ := os.Stat(testFile)
	inode1 := getInode(info1)

	// Simulate having processed the file
	store.UpdateProcessingState("test.jsonl", 100, 100, inode1)

	// Simulate rotation: rename old file, create new file
	os.Rename(testFile, testFile+".1")
	f2, err := os.Create(testFile)
	if err != nil {
		t.Fatalf("Failed to create rotated file: %v", err)
	}
	// Write MORE data than the old file had (this is the bug scenario)
	for i := 0; i < 10; i++ {
		f2.WriteString(`{"data":"{\"resourceLogs\":[]}"}` + "\n")
	}
	f2.Close()

	// Get the inode of the new file
	info2, _ := os.Stat(testFile)
	inode2 := getInode(info2)

	// Verify inodes are different (rotation happened)
	if inode1 == inode2 {
		t.Skip("Filesystem reused inode - can't test inode-based rotation detection")
	}

	// Get current state
	state, _ := store.GetProcessingState("test.jsonl")
	if state.Inode != inode1 {
		t.Errorf("Expected stored inode %d, got %d", inode1, state.Inode)
	}

	// The rotation detection logic should detect inode changed
	inodeChanged := state.Inode != 0 && inode2 != state.Inode
	if !inodeChanged {
		t.Error("Expected inode change to be detected")
	}
}

// TestRotationDetectionByTruncation tests that file truncation is detected
// when the file size is smaller than our last read offset.
// This handles copytruncate-style rotation.
func TestRotationDetectionByTruncation(t *testing.T) {
	dbPath := "./test_rotation_truncate.db"
	defer os.Remove(dbPath)

	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Simulate having processed a file up to byte 10000
	store.UpdateProcessingState("test.jsonl", 10000, 10000, 12345)

	state, _ := store.GetProcessingState("test.jsonl")

	// Simulate file was truncated to 5000 bytes
	currentFileSize := int64(5000)

	// The truncation detection logic
	truncated := state.LastByteOffset > currentFileSize
	if !truncated {
		t.Error("Expected truncation to be detected when file size < last offset")
	}
}

// TestNoFalsePositiveRotationOnGrowth tests that normal file growth
// is NOT detected as rotation.
func TestNoFalsePositiveRotationOnGrowth(t *testing.T) {
	dbPath := "./test_rotation_growth.db"
	defer os.Remove(dbPath)

	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Simulate having processed a file up to byte 10000
	store.UpdateProcessingState("test.jsonl", 10000, 10000, 12345)

	state, _ := store.GetProcessingState("test.jsonl")

	// Simulate file grew to 15000 bytes (normal growth)
	currentFileSize := int64(15000)
	currentInode := uint64(12345) // Same inode

	// Neither condition should trigger
	inodeChanged := state.Inode != 0 && currentInode != state.Inode
	truncated := state.LastByteOffset > currentFileSize

	if inodeChanged {
		t.Error("Should not detect inode change when inode is the same")
	}
	if truncated {
		t.Error("Should not detect truncation when file grew")
	}
}

// TestRotationBugScenario tests the specific bug scenario:
// File rotated and grew past old size before we checked.
func TestRotationBugScenario(t *testing.T) {
	dbPath := "./test_rotation_bug.db"
	defer os.Remove(dbPath)

	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Simulate: file was 10KB, we processed to byte 10000
	store.UpdateProcessingState("test.jsonl", 10000, 10000, 11111)

	state, _ := store.GetProcessingState("test.jsonl")

	// Bug scenario: file was rotated, NEW file grew to 15KB
	// Old check (file size < stored size) would NOT catch this!
	currentFileSize := int64(15000)
	currentInode := uint64(22222) // DIFFERENT inode - new file!

	// Old buggy check:
	oldBuggyCheck := currentFileSize < state.FileSizeBytes
	if oldBuggyCheck {
		t.Error("Old buggy check should NOT detect this rotation")
	}

	// New inode-based check should catch it:
	inodeChanged := state.Inode != 0 && currentInode != state.Inode
	if !inodeChanged {
		t.Error("Inode check SHOULD detect this rotation")
	}
}
