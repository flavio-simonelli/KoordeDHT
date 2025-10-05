package writer

import "time"

// Writer definisce l’interfaccia comune per i writer di risultati del tester.
type Writer interface {
	WriteRow(node, result string, delay time.Duration) error
	Flush() error
	Close() error
}
