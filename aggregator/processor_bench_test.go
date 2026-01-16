package aggregator

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// BenchmarkProcessFile_Small tests performance with 1000 lines
func BenchmarkProcessFile_Small(b *testing.B) {
	benchmarkProcessFile(b, 1000)
}

// BenchmarkProcessFile_Medium tests performance with 10,000 lines
func BenchmarkProcessFile_Medium(b *testing.B) {
	benchmarkProcessFile(b, 10000)
}

// BenchmarkProcessFile_Large tests performance with 100,000 lines
func BenchmarkProcessFile_Large(b *testing.B) {
	benchmarkProcessFile(b, 100000)
}

func benchmarkProcessFile(b *testing.B, lineCount int) {
	// Setup
	dbPath := "./bench_test.db"
	defer os.Remove(dbPath)
	
	store, err := NewStore(dbPath)
	if err != nil {
		b.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()
	
	engine := NewEngine(store)
	
	testDir := "./bench_test_data"
	os.MkdirAll(testDir, 0755)
	defer os.RemoveAll(testDir)
	
	testFile := filepath.Join(testDir, "metrics.jsonl")
	
	// Create test file with lineCount lines
	f, err := os.Create(testFile)
	if err != nil {
		b.Fatalf("Failed to create test file: %v", err)
	}
	
	// Write sample JSONL lines
	for i := 0; i < lineCount; i++ {
		fmt.Fprintf(f, `{"data": "{\"resourceMetrics\": [{\"resource\": {\"attributes\": [{\"key\": \"session.id\", \"value\": {\"stringValue\": \"bench-session\"}}]}, \"scopeMetrics\": [{\"metrics\": [{\"name\": \"claude_code.cost.usage\", \"sum\": {\"dataPoints\": [{\"asDouble\": 0.001, \"timeUnixNano\": \"1234567890\", \"attributes\": [{\"key\": \"model\", \"value\": {\"stringValue\": \"test-model\"}}]}]}}]}]}]}"}`+"\n")
	}
	f.Close()
	
	processor := NewProcessor(testDir, store, engine, 5)
	
	// Benchmark: Process the file multiple times
	// This simulates the "already processed N lines, process a few new ones" scenario
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		// Process the file (first iteration processes all, subsequent iterations process none)
		if err := processor.ProcessFile(testFile); err != nil {
			b.Fatalf("Failed to process file: %v", err)
		}
	}
}
