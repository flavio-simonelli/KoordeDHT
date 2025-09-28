package logger

import "KoordeDHT/internal/domain"

// Field represents a structured key-value field attached to a log entry.
type Field struct {
	Key string
	Val any
}

// Logger defines the minimal logging interface required by other components
// such as the routing table. It supports structured fields and hierarchical
// loggers (Named / With).
type Logger interface {
	Named(name string) Logger          // Named returns a new logger with the specified sub-scope name.
	With(fields ...Field) Logger       // With returns a new logger that includes the given structured fields.
	WithNode(n domain.Node) Logger     // WithNode returns a new logger with information about the provided node.
	Debug(msg string, fields ...Field) // Debug logs a debug-level message with optional structured fields.
	Info(msg string, fields ...Field)  // Info logs an info-level message with optional structured fields.
	Warn(msg string, fields ...Field)  // Warn logs a warning-level message with optional structured fields.
	Error(msg string, fields ...Field) // Error logs an error-level message with optional structured fields.

}

// F is a helper for creating a Field in a concise way.
func F(key string, val any) Field { return Field{Key: key, Val: val} }

// FNode serializes a *domain.Node into a structured field.
// If the pointer is nil, the field value is nil.
func FNode(key string, n *domain.Node) Field {
	if n == nil {
		return Field{Key: key, Val: nil}
	}
	return Field{
		Key: key,
		Val: map[string]any{
			"id":   n.ID.ToBinaryString(true),
			"addr": n.Addr,
		},
	}
}

// FResource serializes a domain.Resource into a structured field
// containing its key and value.
func FResource(key string, r domain.Resource) Field {
	return Field{
		Key: key,
		Val: map[string]any{
			"key":    r.Key.ToBinaryString(true),
			"rawKey": r.RawKey,
			"value":  r.Value,
		},
	}
}

// NopLogger ----------------------------------------------------------------
// NopLogger is a no-op implementation of Logger.
// All methods are implemented but do not perform any action.
// Useful for tests or when logging should be disabled.
type NopLogger struct{}

func (l *NopLogger) Named(name string) Logger          { return l }
func (l *NopLogger) With(fields ...Field) Logger       { return l }
func (l *NopLogger) WithNode(n domain.Node) Logger     { return l }
func (l *NopLogger) Debug(msg string, fields ...Field) {}
func (l *NopLogger) Info(msg string, fields ...Field)  {}
func (l *NopLogger) Warn(msg string, fields ...Field)  {}
func (l *NopLogger) Error(msg string, fields ...Field) {}
