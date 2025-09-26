package main

import (
	"KoordeDHT/internal/bootstrap"
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
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"google.golang.org/grpc"
)

var defaultConfigPath = "config/node/config.yaml"

func main() {
	// Parse command-line flags
	configPath := flag.String("config", defaultConfigPath, "path to configuration file")
	flag.Parse()
	// Load configuration
	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("failed to load configuration from %q: %v", *configPath, err)
	}
	// Validate configuration
	if err := cfg.ValidateConfig(); err != nil {
		log.Fatalf("invalid configuration: %v", err)
	}
	// Initialize logger
	zapLog, err := zapfactory.New(cfg.Logger)
	if err != nil {
		log.Fatalf("failed to initialize logger: %v", err)
	}
	defer func() { _ = zapLog.Sync() }() // flush logger buffers before exit
	var lgr logger.Logger
	lgr = zapfactory.NewZapAdapter(zapLog) // adapt zap.Logger to logger.Interface

	cfg.LogConfig(lgr) // log loaded configuration at DEBUG level

	// Initialize listener (to determine server address and port)
	lis, err := cfg.Listen()
	if err != nil {
		lgr.Error("Fatal: failed to initialize listener", logger.F("err", err))
		os.Exit(1)
	}
	defer func() { _ = lis.Close() }() // close listener on shutdown
	addr := lis.Addr().String()
	lgr.Debug("create listener", logger.F("addr", addr))

	// Initialize the identifier space
	space, err := domain.NewSpace(cfg.DHT.IDBits, cfg.DHT.DeBruijn.Degree)
	if err != nil {
		lgr.Error("failed to initialize identifier space", logger.F("id_bits", cfg.DHT.IDBits), logger.F("degree", cfg.DHT.DeBruijn.Degree), logger.F("err", err))
		os.Exit(1)
	}
	lgr.Debug("identifier space initialized", logger.F("id_bits", space.Bits), logger.F("degree", space.GraphGrade), logger.F("sizeByte", space.ByteLen))

	// Initialize the local node
	id := space.NewIdFromString(addr) // derive ID from address
	domainNode := domain.Node{
		ID:   id,
		Addr: addr,
	}
	lgr.Debug("generated node ID", logger.F("id", id.String()))
	lgr = lgr.Named("node").WithNode(domainNode)
	lgr.Info("New Node initializing")

	// Initialize the routing table
	rt := routingtable.New(
		&domainNode,
		space,
		cfg.DHT.FaultTolerance.SuccessorListSize,
		routingtable.WithLogger(lgr.Named("routingtable")),
	)
	lgr.Debug("initialize routing table")

	// Initialize the client pool
	cp := client.New(
		id,
		addr,
		cfg.DHT.FaultTolerance.FailureTimeout,
		client.WithLogger(lgr.Named("clientpool")),
	)
	lgr.Debug("initialize client pool")

	// Initialize the storage
	store := storage.NewMemoryStorage(
		lgr.Named("storage"),
	)
	lgr.Debug("initialize in-memory storage")

	// Initialize the node
	n := node.New(
		rt,
		cp,
		store,
		node.WithLogger(lgr),
	)
	lgr.Debug("initialize new struct node")

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
	lgr.Debug("initialized gRPC server")

	// Run server in background
	serveErr := make(chan error, 1)
	go func() { serveErr <- s.Start() }()
	lgr.Debug("server started")

	// Join an existing DHT or create a new one
	peers, err := bootstrap.ResolveBootstrap(cfg.DHT.Bootstrap)
	if err != nil {
		lgr.Error("failed to resolve bootstrap peers", logger.F("err", err))
		// cleanup before exit
		n.Stop()
		s.Stop()
		os.Exit(1)
	}
	lgr.Debug("resolved bootstrap peers", logger.F("peers", peers))
	if len(peers) != 0 {
		peer, err := bootstrap.PickRandom(peers)
		lgr.Debug("picked random peers used in like bootstrap", logger.F("peers", peer))
		if err != nil {
			lgr.Error("failed to pick a bootstrap peer", logger.F("err", err))
			// cleanup before exit
			n.Stop()
			s.Stop()
			os.Exit(1)
		}
		lgr.Debug("joining DHT", logger.F("peer", peer))

		if err := n.Join(peer); err != nil {
			lgr.Error("failed to join DHT", logger.F("peer", peer), logger.F("err", err))
			// cleanup before exit
			n.Stop()
			s.Stop()
			os.Exit(1)
		}
		lgr.Debug("joined DHT")
	} else {
		n.CreateNewDHT()
		lgr.Debug("new DHT created")
	}

	// Setup signal handler for graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Start periodic stabilization workers (run until ctx is canceled)
	n.StartStabilizers(ctx, cfg.DHT.FaultTolerance.StabilizationInterval)
	lgr.Debug("Stabilization workers started")

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
