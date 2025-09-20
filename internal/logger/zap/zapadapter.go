package zap

import (
	"KoordeDHT/internal/logger"

	"go.uber.org/zap"
)

// ZapAdapter adatta *zap.Logger all'interfaccia nei package in internal che usano un logger.
type ZapAdapter struct {
	L *zap.Logger
}

func NewZapAdapter(l *zap.Logger) ZapAdapter { return ZapAdapter{L: l} }

func (z ZapAdapter) With(fields ...logger.Field) logger.Logger {
	return ZapAdapter{L: z.L.With(toZap(fields)...)}
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
