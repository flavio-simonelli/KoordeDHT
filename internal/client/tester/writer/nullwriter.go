package writer

import "time"

// NopWriter è un writer che ignora tutte le operazioni (no-op).
type NopWriter struct{}

// WriteRow ignora l'input e non scrive nulla.
func (NopWriter) WriteRow(node, result string, delay time.Duration) error { return nil }

// Flush non fa nulla.
func (NopWriter) Flush() error { return nil }

// Close non fa nulla.
func (NopWriter) Close() error { return nil }
