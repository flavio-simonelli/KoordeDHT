package writer

import "time"

// NopWriter is a writer that does nothing.
type NopWriter struct{}

// WriteRow non fa nulla.
func (NopWriter) WriteRow(node, result string, delay time.Duration) error { return nil }

// Flush non fa nulla.
func (NopWriter) Flush() error { return nil }

// Close non fa nulla.
func (NopWriter) Close() error { return nil }
