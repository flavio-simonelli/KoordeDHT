package config

import (
	"fmt"
	"net"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type FileLoggerConfig struct {
	Path       string `yaml:"path"`
	MaxSize    int    `yaml:"maxSize"`
	MaxBackups int    `yaml:"maxBackups"`
	MaxAge     int    `yaml:"maxAge"`
	Compress   bool   `yaml:"compress"`
}

type LoggerConfig struct {
	Level    string           `yaml:"level"`
	Encoding string           `yaml:"encoding"` // console | json
	Mode     string           `yaml:"mode"`     // stdout | file
	File     FileLoggerConfig `yaml:"file"`
}

type DeBruijnConfig struct {
	Degree      int           `yaml:"degree"`      // grado del grafo de Bruijn
	BackupSize  int           `yaml:"backupSize"`  // backup per fault tolerance
	FixInterval time.Duration `yaml:"fixInterval"` // intervallo aggiornamento puntatori
}

type FaultToleranceConfig struct {
	SuccessorListSize     int           `yaml:"successorListSize"`
	StabilizationInterval time.Duration `yaml:"stabilizationInterval"`
	FailureTimeout        time.Duration `yaml:"failureTimeout"`
}

type BootstrapConfig struct {
	Mode    string   `yaml:"mode"` // "dns" | "static"
	DNSName string   `yaml:"dnsName"`
	SRV     bool     `yaml:"srv"`
	Port    int      `yaml:"port"`  // usato solo se SRV=false
	Peers   []string `yaml:"peers"` // usato solo se mode=static
}

type DHTConfig struct {
	IDBits         int                  `yaml:"idBits"`
	Mode           string               `yaml:"mode"` // public | private
	DeBruijn       DeBruijnConfig       `yaml:"deBruijn"`
	FaultTolerance FaultToleranceConfig `yaml:"faultTolerance"`
	Bootstrap      BootstrapConfig      `yaml:"bootstrap"`
}

type ServerConfig struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
}

type NodeConfig struct {
	Logger LoggerConfig `yaml:"logger"`
	DHT    DHTConfig    `yaml:"dht"`
	Server ServerConfig `yaml:"server"`
}

func LoadConfig(path string) (*NodeConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg NodeConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func (cfg *NodeConfig) ValidateConfig() error {
	// per ora valida solamente il bootstrap
	b := cfg.DHT.Bootstrap

	switch b.Mode {
	case "dns":
		if b.DNSName == "" {
			return fmt.Errorf("bootstrap.dnsName is required in mode=dns")
		}
		if !b.SRV && b.Port <= 0 {
			return fmt.Errorf("bootstrap.port must be > 0 when using A/AAAA (srv=false)")
		}

	case "static":
		if len(b.Peers) == 0 {
			return fmt.Errorf("bootstrap.peers cannot be empty in mode=static")
		}
		for _, p := range b.Peers {
			if _, _, err := net.SplitHostPort(p); err != nil {
				return fmt.Errorf("invalid peer address %q in bootstrap.peers: %w", p, err)
			}
		}

	case "init":
		// modalità speciale per il primo nodo della rete (nessun bootstrap)
		// non richiede parametri specifici

	default:
		return fmt.Errorf("invalid bootstrap.mode: %s (must be 'dns' or 'static')", b.Mode)
	}

	return nil
}
