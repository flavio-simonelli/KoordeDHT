package writer

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// CSVWriter gestisce la scrittura incrementale di log CSV (thread-safe).
type CSVWriter struct {
	mu      sync.Mutex
	file    *os.File
	writer  *csv.Writer
	flushed bool
}

// NewCSVWriter crea o apre un file CSV e scrive l’header se necessario.
func NewCSVWriter(filename string) (*CSVWriter, error) {
	// Crea la directory se non esiste
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

	// Se il file è nuovo, scrivi l’header
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

// WriteRow scrive una singola riga nel CSV
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

// Flush forza la scrittura su disco
func (cw *CSVWriter) Flush() error {
	cw.mu.Lock()
	defer cw.mu.Unlock()

	cw.writer.Flush()
	if err := cw.writer.Error(); err != nil {
		return fmt.Errorf("flush error: %w", err)
	}
	return nil
}

// Close effettua il flush e chiude il file
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
