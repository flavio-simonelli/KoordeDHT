package main

import (
	"KoordeDHT/internal/config"
	"KoordeDHT/internal/domain"
	"KoordeDHT/internal/logger"
	zapfactory "KoordeDHT/internal/logger/zap"
	"KoordeDHT/internal/node"
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
	defer lis.Close()
	addr := lis.Addr().String()
	lgr.Info("Indirizzo di bind", logger.F("addr", addr))
	// inizializza nodo
	id := domain.NewIdFromAddr(addr, cfg.DHT.IDBits)
	lgr.Info("ID del nodo", logger.F("id", id.ToHexString()))
	n := domain.Node{
		ID:   id,
		Addr: addr,
	}
	_, err = node.New(n, cfg.DHT.IDBits, cfg.DHT.DeBruijn.Degree, node.WithLogger(lgr.With(logger.F("component", "node"), logger.F("node_id", id.ToHexString()))))
	if err != nil {
		lgr.Error("Errore nell'inizializzare il nodo", logger.F("error", err.Error()))
		os.Exit(1)
	}
	// avvia server

	// avvia nodo creando o unendosi alla rete
}
