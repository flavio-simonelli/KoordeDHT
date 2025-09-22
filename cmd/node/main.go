package main

import (
	"KoordeDHT/internal/config"
	"KoordeDHT/internal/domain"
	"KoordeDHT/internal/logger"
	zapfactory "KoordeDHT/internal/logger/zap"
	"KoordeDHT/internal/node"
	"KoordeDHT/internal/server"
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
	// istanzia il logger
	zapLog, err := zapfactory.New(cfg.Logger)
	if err != nil {
		log.Fatalf("Errore nel caricamento del file di logger: %v", err)
	}
	defer func() { _ = zapLog.Sync() }()    // prima di chiudere il nodo svuotiamo il buffer del logger
	lgr := zapfactory.NewZapAdapter(zapLog) // inizializza il logger globale
	// inizializza ip:port
	lis, err := cfg.Listen()
	if err != nil {
		zapLog.Fatal("Errore nel risolvere l'indirizzo di bind", zap.Error(err))
	}
	defer func() { _ = lis.Close() }() // chiude il listener alla fine del main
	addr := lis.Addr().String()
	lgr.Info("Indirizzo di bind", logger.F("addr", addr))
	// inizializza nodo
	id := domain.NewIdFromAddr(addr, cfg.DHT.IDBits)
	lgr.Info("ID del nodo", logger.F("id", id.ToHexString()))
	domainNode := domain.Node{
		ID:   id,
		Addr: addr,
	}
	n, err := node.New(domainNode, cfg.DHT.IDBits, cfg.DHT.DeBruijn.Degree, node.WithLogger(lgr.Named("node")))
	if err != nil {
		lgr.Error("Errore nell'inizializzare il nodo", logger.F("error", err.Error()))
		os.Exit(1)
	}
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
		lgr.Info("Created new DHT")
	}
	n.StartBackgroundTasks()
	select {
	case err := <-serveErr:
		lgr.Error("Server errore", logger.F("error", err.Error()))
		os.Exit(1) //TODO: grateful stop
	}
}
