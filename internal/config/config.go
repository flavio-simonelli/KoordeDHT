package config

import (
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

type DHTConfig struct {
	IDBits         int                  `yaml:"idBits"`
	BootstrapPeers []string             `yaml:"bootstrapPeers"`
	DeBruijn       DeBruijnConfig       `yaml:"deBruijn"`
	FaultTolerance FaultToleranceConfig `yaml:"faultTolerance"`
}

type NodeConfig struct {
	Logger LoggerConfig `yaml:"logger"`
	DHT    DHTConfig    `yaml:"dht"`
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
