package main

import (
	"KoordeDHT/internal/config"
	"KoordeDHT/internal/logger"
	"log"

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
	zapLog, err := logger.New(cfg.Logger)
	if err != nil {
		log.Fatalf("Errore nel caricamento del file di logger: %v", err)
	}
	defer zapLog.Sync() // prima di chiudere il nodo svuotiamo il buffer del logger
	// log di esempio
	zapLog.Info("Nodo avviato", zap.String("env", "dev"))
}
