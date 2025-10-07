package main

import (
	"KoordeDHT/internal/bootstrap"
	"KoordeDHT/internal/client/tester"
	"KoordeDHT/internal/client/tester/writer"
	"KoordeDHT/internal/domain"
	"KoordeDHT/internal/logger"
	zapfactory "KoordeDHT/internal/logger/zap"
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

var defaultConfigPath = "config/tester/config.yaml"

func main() {
	// Parse command-line flags
	configPath := flag.String("config", defaultConfigPath, "path to configuration file")
	flag.Parse()

	// Load configuration
	cfg, err := tester.Load(*configPath)
	if err != nil {
		log.Fatalf("failed to load configuration from %q: %v", *configPath, err)
	}
	// Validate configuration
	if err := cfg.Validate(); err != nil {
		log.Fatalf("invalid configuration: %v", err)
	}

	// Initialize logger
	var lgr logger.Logger
	if cfg.Logger.Active {
		zapLog, err := zapfactory.New(cfg.Logger)
		if err != nil {
			log.Fatalf("failed to initialize logger: %v", err)
		}
		defer func() { _ = zapLog.Sync() }()   // flush logger buffers before exit
		lgr = zapfactory.NewZapAdapter(zapLog) // adapt zap.Logger to logger.Interface
	} else {
		lgr = &logger.NopLogger{} // no-op logger
	}
	// Log loaded configuration at INFO level
	cfg.LogConfig(lgr) // log loaded configuration at INFO level

	// initialize writer
	var w writer.Writer
	if cfg.CSV.Enabled {
		w, err = writer.NewCSVWriter(cfg.CSV.Path)
		if err != nil {
			lgr.Error("failed to initialize CSV writer", logger.F("err", err))
			return
		}
	} else {
		w = writer.NopWriter{}
	}
	defer w.Close()

	// initialize domain space
	space, err := domain.NewSpace(cfg.DHT.IDBits, 2, 2)
	if err != nil {
		lgr.Error("failed to initialize domain space", logger.F("err", err))
		return
	}

	// initialize bootstrap
	var boot bootstrap.Bootstrap
	if cfg.Bootstrap.Mode == "route53" {
		boot, err = bootstrap.NewRoute53Bootstrap(cfg.Bootstrap.Route53)
		if err != nil {
			lgr.Error("failed to initialize route53 bootstrap", logger.F("err", err))
			return
		}
	} else {
		boot = tester.NewDockerBootstrap(cfg.Bootstrap.Docker.ContainerSuffix, cfg.Bootstrap.Docker.Port, cfg.Bootstrap.Docker.Network)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle termination signals for graceful shutdown
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigs
		lgr.Warn("Received termination signal", logger.F("signal", sig.String()))
		cancel()
	}()

	// Initialize and run tester
	runner := tester.New(cfg, lgr.Named("runner"), w, boot, space)
	start := time.Now()
	if err := runner.Run(ctx); err != nil {
		lgr.Error("tester run failed", logger.F("err", err))
	}
	lgr.Info("tester finished", logger.F("elapsed", time.Since(start)))
}
