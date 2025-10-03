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
	"KoordeDHT/internal/telemetry"
	"KoordeDHT/internal/telemetry/lookuptrace"
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
	cfg.ApplyEnvOverrides()
	// Validate configuration
	if err := cfg.ValidateConfig(); err != nil {
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
	// Log loaded configuration at DEBUG level
	cfg.LogConfig(lgr) // log loaded configuration at DEBUG level

	// Initialize listener (to determine server address and port)
	lis, advertised, err := server.Listen(cfg.DHT.Mode, cfg.Node.Bind, cfg.Node.Host, cfg.Node.Port)
	if err != nil {
		lgr.Error("Fatal: failed to initialize listener", logger.F("err", err))
		os.Exit(1)
	}
	defer func() { _ = lis.Close() }() // close listener on shutdown
	addr := lis.Addr().String()
	lgr.Debug("create listener", logger.F("addr", addr))

	// Initialize the identifier space
	space, err := domain.NewSpace(cfg.DHT.IDBits, cfg.DHT.DeBruijn.Degree, cfg.DHT.FaultTolerance.SuccessorListSize)
	if err != nil {
		lgr.Error("failed to initialize identifier space", logger.F("err", err))
		os.Exit(1)
	}
	lgr.Debug("identifier space initialized", logger.F("id_bits", space.Bits), logger.F("degree", space.GraphGrade), logger.F("sizeByte", space.ByteLen), logger.F("SuccessorListSize", space.SuccListSize))

	// Initialize the local node
	var id domain.ID
	if cfg.Node.Id == "" {
		id = space.NewIdFromString(addr) // derive ID from address
	} else {
		id, err = space.FromHexString(cfg.Node.Id) // use configured ID
		if err != nil {
			lgr.Error("invalid node ID in configuration", logger.F("err", err))
			os.Exit(1)
		}
	}
	domainNode := domain.Node{
		ID:   id,
		Addr: advertised,
	}
	lgr.Debug("generated node ID", logger.F("id", id.ToHexString(true)))
	lgr = lgr.Named("node").WithNode(domainNode)
	lgr.Info("New Node initializing")

	// Initialize Telemetry (if enabled)
	shutdown := telemetry.InitTracer(cfg.Telemetry, "KoordeDHT-Node", id)
	defer shutdown(context.Background())

	// Initialize the routing table
	rt := routingtable.New(
		&domainNode,
		space,
		routingtable.WithLogger(lgr.Named("routingtable")),
	)
	lgr.Debug("initialized routing table")

	// Initialize the client pool
	cp := client.New(
		id,
		addr,
		cfg.DHT.FaultTolerance.FailureTimeout,
		client.WithLogger(lgr.Named("clientpool")),
	)
	lgr.Debug("initialized client pool")

	// Initialize the storage
	store := storage.NewMemoryStorage(
		lgr.Named("storage"),
	)
	lgr.Debug("initialized in-memory storage")

	// Initialize the node
	n := node.New(
		rt,
		cp,
		store,
		node.WithLogger(lgr),
	)
	lgr.Debug("initialized new struct node")

	// Initialize the gRPC server
	var grpcOpts []grpc.ServerOption
	if cfg.Telemetry.Tracing.Enabled {
		grpcOpts = append(grpcOpts,
			grpc.ChainUnaryInterceptor(
				lookuptrace.ServerInterceptor(),
			),
		)
		lgr.Debug("gRPC tracing enabled (lookup-only)")
	}

	s, err := server.New(
		lis,
		n,
		grpcOpts,
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

	// resolve host and port for bootstrap
	var register bootstrap.Bootstrap
	if cfg.DHT.Bootstrap.Mode == "route53" {
		register, err = bootstrap.NewRoute53Bootstrap(cfg.DHT.Bootstrap.Route53)
		if err != nil {
			lgr.Error("failed to initialize Route53 bootstrap", logger.F("err", err))
			// cleanup before exit
			s.Stop()
			n.Stop()
			os.Exit(1)
		}
	} else if cfg.DHT.Bootstrap.Mode == "static" {
		register = bootstrap.NewStaticBootstrap(cfg.DHT.Bootstrap.Peers)
	} else {
		lgr.Error("unsupported bootstrap mode", logger.F("mode", cfg.DHT.Bootstrap.Mode))
		// cleanup before exit
		s.Stop()
		n.Stop()
		os.Exit(1)
	}

	// Join an existing DHT or create a new one
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	peers, err := register.Discover(ctx)
	cancel()
	if err != nil {
		lgr.Error("failed to resolve bootstrap peers", logger.F("err", err))
		// cleanup before exit
		s.Stop()
		n.Stop()
		os.Exit(1)
	}
	lgr.Info("resolved bootstrap peers", logger.F("peers", peers))
	if len(peers) != 0 {
		if err := n.Join(peers); err != nil {
			lgr.Error("failed to join DHT", logger.F("err", err))
			// cleanup before exit
			s.Stop()
			n.Stop()
			os.Exit(1)
		}
		lgr.Debug("joined DHT")
	} else {
		n.CreateNewDHT()
		lgr.Debug("new DHT created")
	}

	// Register node
	ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
	err = register.Register(ctx, &domainNode)
	cancel()
	if err != nil {
		lgr.Error("failed to register DHT", logger.F("err", err))
	} else {
		lgr.Info("node registered successfully")
		defer func() {
			// Deregister node on shutdown
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			err := register.Deregister(ctx, &domainNode)
			cancel()
			if err != nil {
				lgr.Warn("failed to deregister node", logger.F("err", err))
			}
		}()
	}

	// Setup signal handler for graceful shutdown
	ctx, stabilizerStop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)

	// Start periodic stabilization workers (run until ctx is canceled)
	n.StartStabilizers(ctx, cfg.DHT.FaultTolerance.StabilizationInterval, cfg.DHT.DeBruijn.FixInterval, cfg.DHT.Storage.FixInterval)
	lgr.Debug("Stabilization workers started")

	select {
	case <-ctx.Done():
		lgr.Info("shutdown signal received, stopping server gracefully...")

		stabilizerStop() // stop stabilization workers

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
		}

		n.Stop() // stop node

	case err := <-serveErr:
		lgr.Error("gRPC server terminated unexpectedly", logger.F("err", err))
		stabilizerStop()
		n.Stop()
		os.Exit(1)
	}
}
