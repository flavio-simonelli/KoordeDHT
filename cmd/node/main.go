package main

import (
	"KoordeDHT/internal/config"
	"KoordeDHT/internal/domain"
	"KoordeDHT/internal/logger"
	zapfactory "KoordeDHT/internal/logger/zap"
	"KoordeDHT/internal/node"
	"KoordeDHT/internal/routingtable"
	"KoordeDHT/internal/server"
	"context"
	"log"
	"os"

	"go.uber.org/zap"
)

var configPath = "config/node/config.yaml"

func main() {
	// carica la configurazione
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		log.Fatalf("Errore nel caricamento del file di configurazione: %v", err)
	}
	// valida la configurazione
	err = cfg.ValidateConfig()
	if err != nil {
		log.Fatalf("Errore nella validazione della configurazione: %v", err)
	}
	// istanzia il logger
	zapLog, err := zapfactory.New(cfg.Logger)
	if err != nil {
		log.Fatalf("Errore nel caricamento del file di logger: %v", err)
	}
	defer func() { _ = zapLog.Sync() }()    // prima di chiudere il nodo svuotiamo il buffer del logger
	lgr := zapfactory.NewZapAdapter(zapLog) // inizializza il logger globale
	// inizializza la listen socket
	lis, err := cfg.Listen()
	if err != nil {
		zapLog.Fatal("Errore nel risolvere l'indirizzo di bind", zap.Error(err))
	}
	defer func() { _ = lis.Close() }() // chiude il listener alla fine del main
	addr := lis.Addr().String()
	lgr.Info("Indirizzo di bind", logger.F("addr", addr))
	// inizializza lo spazio degli ID
	space, err := domain.NewSpace(cfg.DHT.IDBits, cfg.DHT.DeBruijn.Degree)
	if err != nil {
		lgr.Error("Errore nell'inizializzare lo spazio degli ID", logger.F("error", err.Error()))
		os.Exit(1)
	}
	// inizializza nodo
	id := space.NewIdFromAddr(addr)
	lgr.Info("ID del nodo", logger.F("id", id.String()))
	domainNode := domain.Node{
		ID:   id,
		Addr: addr,
	}
	// inizializza la tabella di routing
	rt := routingtable.New(&domainNode, space, cfg.DHT.FaultTolerance.SuccessorListSize, routingtable.WithLogger(lgr.Named("routingtable")))
	n := node.New(rt, node.WithLogger(lgr.Named("node")))
	// avvia server
	s := server.New(n)
	serveErr := make(chan error, 1)
	go func() { serveErr <- s.Run(lis) }()
	// check se il server ha errori
	select {
	case err := <-serveErr:
		lgr.Error("Server errore", logger.F("error", err.Error()))
		os.Exit(1) //TODO: grateful stop
	default:
		// nessun errore ancora
	}
	lgr.Debug("Server started correctly")
	// join in dht or create a new one
	if len(cfg.DHT.BootstrapPeers) != 0 {
		// join
		peer := cfg.DHT.BootstrapPeers[0] //TODO: per ora uso solo il primo
		lgr.Debug("Joining DHT", logger.F("peer", peer))
		err = n.Join(peer)
		if err != nil {
			lgr.Error("Errore nel join alla DHT", logger.F("error", err.Error()))
			os.Exit(1) //TODO: grateful stop
		}
		lgr.Info("Join avvenuto con successo", logger.F("peer", peer))
	} else {
		// crea nuova dht
		n.CreateNewDHT()
		lgr.Info("Nuova DHT creata con successo")
	}
	// avvia i worker di stabilizzazione
	ctx, _ := context.WithCancel(context.Background())
	n.StartStabilizer(ctx, cfg.DHT.FaultTolerance.StabilizationInterval)
	select {
	case err := <-serveErr:
		lgr.Error("Server errore", logger.F("error", err.Error()))
		os.Exit(1) //TODO: grateful stop
	}
}
