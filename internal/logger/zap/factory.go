package zap

import (
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"

	"KoordeDHT/internal/config"
)

func New(cfg config.LoggerConfig) (*zap.Logger, error) {
	// livello di log
	level := zap.NewAtomicLevel()
	if err := level.UnmarshalText([]byte(cfg.Level)); err != nil {
		// fallback info level
		level = zap.NewAtomicLevelAt(zap.InfoLevel)
	}
	// encoder (console o json)
	encCfg := zap.NewProductionEncoderConfig()
	encCfg.TimeKey = "ts"
	encCfg.EncodeTime = zapcore.ISO8601TimeEncoder
	encCfg.EncodeLevel = zapcore.LowercaseLevelEncoder
	encCfg.NameKey = "component" // <— così .Named() finisce in “component”
	var encoder zapcore.Encoder
	if cfg.Encoding == "console" {
		encCfg.EncodeLevel = zapcore.CapitalColorLevelEncoder
		encoder = zapcore.NewConsoleEncoder(encCfg)
	} else {
		encoder = zapcore.NewJSONEncoder(encCfg)
	}
	var ws zapcore.WriteSyncer
	switch cfg.Mode {
	case "stdout":
		ws = zapcore.AddSync(os.Stdout)
	case "file":
		ws = zapcore.AddSync(&lumberjack.Logger{
			Filename:   cfg.File.Path,
			MaxSize:    cfg.File.MaxSize,
			MaxBackups: cfg.File.MaxBackups,
			MaxAge:     cfg.File.MaxAge,
			Compress:   cfg.File.Compress,
		})
	default:
		ws = zapcore.AddSync(os.Stdout) // fallback console
	}
	core := zapcore.NewCore(encoder, ws, level)
	return zap.New(core, zap.AddCaller(), zap.AddStacktrace(zap.ErrorLevel)), nil
}
