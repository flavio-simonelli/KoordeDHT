package zap

import (
	"KoordeDHT/internal/domain"
	"KoordeDHT/internal/logger"

	"go.uber.org/zap"
)

// ZapAdapter adatta *zap.Logger all'interfaccia nei package in internal che usano un logger.
type ZapAdapter struct {
	L *zap.Logger
}

// Applica lo skip di 1 frame per evitare che il caller sia zapadapter.go:XX
func NewZapAdapter(l *zap.Logger) ZapAdapter {
	return ZapAdapter{L: l.WithOptions(zap.AddCallerSkip(1))}
}

func (z ZapAdapter) With(fields ...logger.Field) logger.Logger {
	return ZapAdapter{L: z.L.With(toZap(fields)...)}
}

func (z ZapAdapter) Named(name string) logger.Logger {
	return ZapAdapter{L: z.L.Named(name)}
}

func (z ZapAdapter) WithNode(n domain.Node) logger.Logger {
	return ZapAdapter{L: z.L.With(
		zap.Any("self", map[string]any{
			"id":   n.ID.String(),
			"addr": n.Addr,
		}),
	)}
}

func (z ZapAdapter) Debug(msg string, fields ...logger.Field) {
	if ce := z.L.Check(zap.DebugLevel, msg); ce != nil {
		ce.Write(toZap(fields)...)
	}
}
func (z ZapAdapter) Info(msg string, fields ...logger.Field) {
	if ce := z.L.Check(zap.InfoLevel, msg); ce != nil {
		ce.Write(toZap(fields)...)
	}
}
func (z ZapAdapter) Warn(msg string, fields ...logger.Field) {
	if ce := z.L.Check(zap.WarnLevel, msg); ce != nil {
		ce.Write(toZap(fields)...)
	}
}
func (z ZapAdapter) Error(msg string, fields ...logger.Field) {
	if ce := z.L.Check(zap.ErrorLevel, msg); ce != nil {
		ce.Write(toZap(fields)...)
	}
}

func toZap(fs []logger.Field) []zap.Field {
	if len(fs) == 0 {
		return nil
	}
	out := make([]zap.Field, 0, len(fs))
	for _, f := range fs {
		out = append(out, zap.Any(f.Key, f.Val))
	}
	return out
}
