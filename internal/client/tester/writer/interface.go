package writer

import "time"

// Writer is the interface for writing test results.
type Writer interface {
	WriteRow(node, result string, delay time.Duration) error
	Flush() error
	Close() error
}
