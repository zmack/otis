package collector

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

type FileWriter struct {
	mu       sync.Mutex
	filePath string
}

func NewFileWriter(filePath string) (*FileWriter, error) {
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	return &FileWriter{
		filePath: filePath,
	}, nil
}

func (w *FileWriter) WriteJSON(data interface{}) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	f, err := os.OpenFile(w.filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open file %s: %w", w.filePath, err)
	}
	defer f.Close()

	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal data to JSON: %w", err)
	}

	if _, err := f.Write(append(jsonData, '\n')); err != nil {
		return fmt.Errorf("failed to write to file %s: %w", w.filePath, err)
	}

	return nil
}

func (w *FileWriter) WriteLine(s string) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	f, err := os.OpenFile(w.filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open file %s: %w", w.filePath, err)
	}
	defer f.Close()

	if _, err := f.WriteString(s + "\n"); err != nil {
		return fmt.Errorf("failed to write to file %s: %w", w.filePath, err)
	}

	return nil
}
