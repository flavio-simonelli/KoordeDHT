package writer

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// CSVWriter handles incremental writing of CSV logs (thread-safe).
type CSVWriter struct {
	mu      sync.Mutex
	file    *os.File
	writer  *csv.Writer
	flushed bool
}

// NewCSVWriter creates or opens a CSV file and writes the header if necessary.
func NewCSVWriter(filename string) (*CSVWriter, error) {
	// Create directory if it doesn't exist
	dir := filepath.Dir(filename)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("cannot create directory %q: %w", dir, err)
	}

	fileExists := false
	if _, err := os.Stat(filename); err == nil {
		fileExists = true
	}

	file, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, fmt.Errorf("cannot open csv file: %w", err)
	}

	w := csv.NewWriter(file)

	// Write header if file is new
	if !fileExists {
		header := []string{"timestamp", "node", "result", "delay_ms"}
		if err := w.Write(header); err != nil {
			file.Close()
			return nil, fmt.Errorf("cannot write header: %w", err)
		}
		w.Flush()
	}

	return &CSVWriter{
		file:   file,
		writer: w,
	}, nil
}

// WriteRow writes a single row to the CSV file in a thread-safe manner.
func (cw *CSVWriter) WriteRow(node, result string, delay time.Duration) error {
	cw.mu.Lock()
	defer cw.mu.Unlock()

	if cw.flushed {
		return fmt.Errorf("cannot write: writer already closed")
	}

	record := []string{
		time.Now().Format(time.RFC3339Nano),
		node,
		result,
		fmt.Sprintf("%.3f", float64(delay.Milliseconds())), // delay in ms
	}

	if err := cw.writer.Write(record); err != nil {
		return fmt.Errorf("csv write error: %w", err)
	}

	return nil
}

// Flush flushes the CSV writer buffer to the file.
func (cw *CSVWriter) Flush() error {
	cw.mu.Lock()
	defer cw.mu.Unlock()

	cw.writer.Flush()
	if err := cw.writer.Error(); err != nil {
		return fmt.Errorf("flush error: %w", err)
	}
	return nil
}

// Close closes the CSV file after flushing any remaining data.
func (cw *CSVWriter) Close() error {
	cw.mu.Lock()
	defer cw.mu.Unlock()

	if cw.flushed {
		return nil
	}

	cw.writer.Flush()
	cw.flushed = true

	if err := cw.writer.Error(); err != nil {
		_ = cw.file.Close()
		return fmt.Errorf("flush error: %w", err)
	}

	return cw.file.Close()
}
