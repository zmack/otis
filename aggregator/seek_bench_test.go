package aggregator

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"testing"
)

// BenchmarkOldWay_SkipLines simulates the OLD implementation
// Reading from start and skipping N lines
func BenchmarkOldWay_Skip1K(b *testing.B) {
	benchmarkOldWay(b, 1000)
}

func BenchmarkOldWay_Skip10K(b *testing.B) {
	benchmarkOldWay(b, 10000)
}

func BenchmarkOldWay_Skip100K(b *testing.B) {
	benchmarkOldWay(b, 100000)
}

// BenchmarkNewWay_Seek simulates the NEW implementation
// Seeking to byte offset directly
func BenchmarkNewWay_Seek1K(b *testing.B) {
	benchmarkNewWay(b, 1000)
}

func BenchmarkNewWay_Seek10K(b *testing.B) {
	benchmarkNewWay(b, 10000)
}

func BenchmarkNewWay_Seek100K(b *testing.B) {
	benchmarkNewWay(b, 100000)
}

func benchmarkOldWay(b *testing.B, skipLines int) {
	// Create test file
	testFile := "./bench_skip_test.txt"
	f, _ := os.Create(testFile)
	
	// Write skipLines + 10 lines
	var totalBytes int64
	for i := 0; i < skipLines+10; i++ {
		line := fmt.Sprintf("This is test line number %d with some data\n", i)
		n, _ := f.WriteString(line)
		totalBytes += int64(n)
	}
	f.Close()
	defer os.Remove(testFile)
	
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		// OLD WAY: Open and skip lines
		f, _ := os.Open(testFile)
		scanner := bufio.NewScanner(f)
		
		// Skip already processed lines (THE SLOW PART!)
		lineNum := 0
		for lineNum < skipLines && scanner.Scan() {
			lineNum++
		}
		
		// Process remaining lines
		for scanner.Scan() {
			_ = scanner.Text()
		}
		
		f.Close()
	}
}

func benchmarkNewWay(b *testing.B, skipLines int) {
	// Create test file
	testFile := "./bench_seek_test.txt"
	f, _ := os.Create(testFile)
	
	// Write skipLines + 10 lines
	var byteOffsets []int64
	var currentOffset int64
	for i := 0; i < skipLines+10; i++ {
		byteOffsets = append(byteOffsets, currentOffset)
		line := fmt.Sprintf("This is test line number %d with some data\n", i)
		n, _ := f.WriteString(line)
		currentOffset += int64(n)
	}
	f.Close()
	defer os.Remove(testFile)
	
	// Get the byte offset after skipLines
	seekOffset := byteOffsets[skipLines]
	
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		// NEW WAY: Seek to position
		f, _ := os.Open(testFile)
		
		// Seek directly to where we left off (THE FAST PART!)
		f.Seek(seekOffset, io.SeekStart)
		scanner := bufio.NewScanner(f)
		
		// Process remaining lines
		for scanner.Scan() {
			_ = scanner.Text()
		}
		
		f.Close()
	}
}
