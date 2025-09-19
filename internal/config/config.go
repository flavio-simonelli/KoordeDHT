package config

import (
	"os"

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
	Encoding string           `yaml:"encoding"`
	Mode     string           `yaml:"mode"` // console o file
	File     FileLoggerConfig `yaml:"file"`
}

type AppConfig struct {
	Logger LoggerConfig `yaml:"logger"`
}

func LoadConfig(path string) (*AppConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg AppConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}
