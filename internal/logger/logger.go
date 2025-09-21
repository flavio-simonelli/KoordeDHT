package logger

import "KoordeDHT/internal/domain"

// Field rappresenta un campo strutturato (key:value).
type Field struct {
	Key string
	Val any
}

// Logger è l'interfaccia minima richiesta da routingtable.
type Logger interface {
	Named(name string) Logger
	With(fields ...Field) Logger
	Debug(msg string, fields ...Field)
	Info(msg string, fields ...Field)
	Warn(msg string, fields ...Field)
	Error(msg string, fields ...Field)
}

// F è un helper per creare un Field in modo conciso.
func F(key string, val any) Field { return Field{Key: key, Val: val} }

// FNode serializza un domain.Node in un campo strutturato leggibile.
func FNode(key string, n domain.Node) Field {
	return Field{
		Key: key,
		Val: map[string]any{
			"id":   n.ID.ToHexString(),
			"addr": n.Addr,
		},
	}
}

// ----------------------------------------------------------------
// NopLogger è un'implementazione di Logger che non fa nulla.
type NopLogger struct{}

func (l *NopLogger) Named(name string) Logger          { return l }
func (l *NopLogger) With(fields ...Field) Logger       { return l }
func (l *NopLogger) Debug(msg string, fields ...Field) {}
func (l *NopLogger) Info(msg string, fields ...Field)  {}
func (l *NopLogger) Warn(msg string, fields ...Field)  {}
func (l *NopLogger) Error(msg string, fields ...Field) {}
