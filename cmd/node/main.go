package main

import (
	"KoordeDHT/internal/client"
	"KoordeDHT/internal/config"
	"KoordeDHT/internal/domain"
	"KoordeDHT/internal/logger"
	zapfactory "KoordeDHT/internal/logger/zap"
	"KoordeDHT/internal/node"
	"KoordeDHT/internal/routingtable"
	"KoordeDHT/internal/server"
	"KoordeDHT/internal/storage"
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc"
)

var configPath = "config/node/config.yaml"

func main() {
	// Load configuration
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		log.Fatalf("failed to load configuration from %q: %v", configPath, err)
	}
	if err := cfg.ValidateConfig(); err != nil {
		log.Fatalf("invalid configuration: %v", err)
	}

	// Initialize logger
	zapLog, err := zapfactory.New(cfg.Logger)
	if err != nil {
		log.Fatalf("failed to initialize logger: %v", err)
	}
	defer func() { _ = zapLog.Sync() }() // flush logger buffers before exit
	lgr := zapfactory.NewZapAdapter(zapLog)
	lgr.Info("configuration loaded and validated successfully", logger.F("path", configPath))

	// Initialize listener for incoming connections (to determine server address and port)
	lis, err := cfg.Listen()
	if err != nil {
		zapLog.Fatal("failed to bind listener", zap.Error(err))
	}
	defer func() { _ = lis.Close() }() // close listener on shutdown
	addr := lis.Addr().String()
	lgr.Info("listener initialized successfully", logger.F("addr", addr))

	// Initialize the identifier space
	space, err := domain.NewSpace(cfg.DHT.IDBits, cfg.DHT.DeBruijn.Degree)
	if err != nil {
		lgr.Error("failed to initialize identifier space", logger.F("id_bits", cfg.DHT.IDBits), logger.F("degree", cfg.DHT.DeBruijn.Degree), logger.F("err", err))
		os.Exit(1)
	}

	// Initialize the local node
	id := space.NewIdFromString(addr) // derive ID from address
	lgr.Info("node identifier assigned", logger.F("id", id.String()))
	domainNode := domain.Node{
		ID:   id,
		Addr: addr,
	}

	// Initialize the routing table
	rt := routingtable.New(
		&domainNode,
		space,
		cfg.DHT.FaultTolerance.SuccessorListSize,
		routingtable.WithLogger(lgr.Named("routingtable")),
	)

	// Initialize the client pool
	cp := client.New(
		addr,
		cfg.DHT.FaultTolerance.FailureTimeout,
		client.WithLogger(lgr.Named("clientpool")),
	)

	// Initialize the storage
	store := storage.NewMemoryStorage()

	// Initialize the node
	n := node.New(
		rt,
		cp,
		store,
		node.WithLogger(lgr.Named("node")),
	)

	// Initialize the gRPC server
	s, err := server.New(
		lis,
		n,
		[]grpc.ServerOption{},
		server.WithLogger(lgr.Named("server")),
	)
	if err != nil {
		lgr.Error("failed to initialize gRPC server", logger.F("err", err))
		os.Exit(1)
	}

	// Run server in background
	serveErr := make(chan error, 1)
	go func() { serveErr <- s.Start() }()
	lgr.Info("gRPC server started successfully", logger.F("addr", addr))

	// Join an existing DHT or create a new one
	if len(cfg.DHT.BootstrapPeers) != 0 {
		peer := cfg.DHT.BootstrapPeers[0] // TODO: use multiple peers
		lgr.Debug("joining DHT", logger.F("peer", peer))

		if err := n.Join(peer); err != nil {
			lgr.Error("failed to join DHT", logger.F("peer", peer), logger.F("err", err))
			// cleanup before exit
			n.Stop()
			s.Stop()
			os.Exit(1)
		}

		lgr.Info("successfully joined DHT", logger.F("peer", peer))
	} else {
		n.CreateNewDHT()
		lgr.Info("new DHT created successfully")
	}

	// Setup signal handler for graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Start periodic stabilization workers (run until ctx is canceled)
	n.StartStabilizers(ctx, cfg.DHT.FaultTolerance.StabilizationInterval)
	lgr.Info("Stabilization workers started")

	select {
	case <-ctx.Done():
		lgr.Info("shutdown signal received, stopping server gracefully...")

		n.Stop() // stop node

		// Allow some time for graceful stop
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		done := make(chan struct{})
		go func() {
			s.GracefulStop()
			close(done)
		}()

		select {
		case <-done:
			lgr.Info("server stopped gracefully")
		case <-shutdownCtx.Done():
			lgr.Warn("graceful stop timed out, forcing shutdown")
			s.Stop()
		}

	case err := <-serveErr:
		lgr.Error("gRPC server terminated unexpectedly", logger.F("err", err))
		n.Stop()
		os.Exit(1)
	}
}
