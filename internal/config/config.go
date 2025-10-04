package config

import (
	"KoordeDHT/internal/logger"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type TracingConfig struct {
	Enabled  bool   `yaml:"enabled"`
	Exporter string `yaml:"exporter"`
	Endpoint string `yaml:"endpoint"`
}

type TelemetryConfig struct {
	Tracing TracingConfig `yaml:"tracing"`
}

type FileLoggerConfig struct {
	Path       string `yaml:"path"`
	MaxSize    int    `yaml:"maxSize"`
	MaxBackups int    `yaml:"maxBackups"`
	MaxAge     int    `yaml:"maxAge"`
	Compress   bool   `yaml:"compress"`
}

type LoggerConfig struct {
	Active   bool             `yaml:"active"`
	Level    string           `yaml:"level"`
	Encoding string           `yaml:"encoding"`
	Mode     string           `yaml:"mode"`
	File     FileLoggerConfig `yaml:"file"`
}

type DeBruijnConfig struct {
	Degree      int           `yaml:"degree"`
	BackupSize  int           `yaml:"backupSize"`
	FixInterval time.Duration `yaml:"fixInterval"`
}

type FaultToleranceConfig struct {
	SuccessorListSize     int           `yaml:"successorListSize"`
	StabilizationInterval time.Duration `yaml:"stabilizationInterval"`
	FailureTimeout        time.Duration `yaml:"failureTimeout"`
}

type Route53Config struct {
	HostedZoneID string `yaml:"hostedZoneId"`
	DomainSuffix string `yaml:"domainSuffix"`
	TTL          int64  `yaml:"ttl"`
	Region       string `yaml:"region"`
}

type BootstrapConfig struct {
	Mode    string        `yaml:"mode"`
	Peers   []string      `yaml:"peers"`
	Route53 Route53Config `yaml:"route53"`
}

type StorageConfig struct {
	FixInterval time.Duration `yaml:"fixInterval"`
}

type DHTConfig struct {
	IDBits         int                  `yaml:"idBits"`
	Mode           string               `yaml:"mode"`
	DeBruijn       DeBruijnConfig       `yaml:"deBruijn"`
	FaultTolerance FaultToleranceConfig `yaml:"faultTolerance"`
	Storage        StorageConfig        `yaml:"storage"`
	Bootstrap      BootstrapConfig      `yaml:"bootstrap"`
}

type NodeConfig struct {
	Id   string `yaml:"id"`
	Bind string `yaml:"bind"`
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
}

type Config struct {
	Logger    LoggerConfig    `yaml:"logger"`
	DHT       DHTConfig       `yaml:"dht"`
	Node      NodeConfig      `yaml:"node"`
	Telemetry TelemetryConfig `yaml:"telemetry"`
}

// LoadConfig loads the configuration from a YAML file at the given path.
//
// Behavior:
//   - Reads the file contents from disk.
//   - Unmarshals the YAML data into a Config struct.
//   - Returns the parsed configuration or an error if reading or parsing fails.
//
// This function performs only syntactic parsing of the YAML file.
// To validate the configuration structure and check for missing or invalid
// fields, call cfg.ValidateConfig() after loading.
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// ApplyEnvOverrides applies environment variable overrides to the configuration.
//
// Behavior:
//   - This method modifies only selected fields of the Config struct that are
//     commonly node-specific or deployment-dependent.
//   - For each supported field, if a corresponding environment variable is set,
//     its value overrides the value loaded from the YAML configuration file.
//
// Supported overrides include:
//
//		NODE_ID             -> cfg.Node.Id
//		NODE_BIND           -> cfg.Node.Bind
//		NODE_HOST           -> cfg.Node.Host
//		NODE_PORT           -> cfg.Node.Port
//
//		BOOTSTRAP_MODE      -> cfg.DHT.Bootstrap.Mode ("static" or "route53")
//		BOOTSTRAP_PEERS     -> cfg.DHT.Bootstrap.Peers (comma-separated list, used only in mode=static)
//
//		ROUTE53_ZONE_ID     -> cfg.DHT.Bootstrap.Route53.HostedZoneID
//		ROUTE53_SUFFIX      -> cfg.DHT.Bootstrap.Route53.DomainSuffix
//		ROUTE53_TTL         -> cfg.DHT.Bootstrap.Route53.TTL
//	 	ROUTE53_REGION 		-> cfg.DHT.Bootstrap.Route53.Region
//
//		TRACE_ENABLED       -> cfg.Telemetry.Tracing.Enabled
//		TRACE_EXPORTER      -> cfg.Telemetry.Tracing.Exporter
//		TRACE_ENDPOINT      -> cfg.Telemetry.Tracing.Endpoint
//
//		LOGGER_ENABLED      -> cfg.Logger.Active
//		LOGGER_LEVEL        -> cfg.Logger.Level
//		LOGGER_ENCODING     -> cfg.Logger.Encoding
//		LOGGER_MODE         -> cfg.Logger.Mode
//		LOGGER_FILE_PATH    -> cfg.Logger.File.Path
//
// Type conversions:
//   - Integer fields (e.g., NODE_PORT, ROUTE53_TTL) are parsed using strconv.Atoi / ParseInt;
//     invalid values are ignored.
//   - Boolean fields (e.g., TRACE_ENABLED, LOGGER_ENABLED) accept "true", "1", or "yes"
//     (case-insensitive) as true; any other non-empty value is treated as false.
//   - Lists such as BOOTSTRAP_PEERS are parsed by splitting the string on commas.
//
// Usage example:
//
//	cfg, _ := LoadConfig("config.yaml")
//	cfg.ApplyEnvOverrides()
func (cfg *Config) ApplyEnvOverrides() {
	if v := os.Getenv("NODE_ID"); v != "" {
		cfg.Node.Id = v
	}
	if v := os.Getenv("NODE_BIND"); v != "" {
		cfg.Node.Bind = v
	} else {
		cfg.Node.Bind = "0.0.0.0" // default
	}
	if v := os.Getenv("NODE_HOST"); v != "" {
		cfg.Node.Host = v
	}
	if v := os.Getenv("NODE_PORT"); v != "" {
		if port, err := strconv.Atoi(v); err == nil {
			cfg.Node.Port = port
		}
	}
	if v := os.Getenv("DHT_MODE"); v != "" {
		cfg.DHT.Mode = v
	}
	if v := os.Getenv("BOOTSTRAP_MODE"); v != "" {
		cfg.DHT.Bootstrap.Mode = v
	}
	if v := os.Getenv("BOOTSTRAP_PEERS"); v != "" {
		cfg.DHT.Bootstrap.Peers = strings.Split(v, ",")
	}
	if v := os.Getenv("TRACE_ENABLED"); v != "" {
		v = strings.ToLower(v)
		cfg.Telemetry.Tracing.Enabled = v == "true" || v == "1" || v == "yes"
	}
	if v := os.Getenv("TRACE_EXPORTER"); v != "" {
		cfg.Telemetry.Tracing.Exporter = v
	}
	if v := os.Getenv("TRACE_ENDPOINT"); v != "" {
		cfg.Telemetry.Tracing.Endpoint = v
	}
	if v := os.Getenv("ROUTE53_ZONE_ID"); v != "" {
		cfg.DHT.Bootstrap.Route53.HostedZoneID = v
	}
	if v := os.Getenv("ROUTE53_SUFFIX"); v != "" {
		cfg.DHT.Bootstrap.Route53.DomainSuffix = v
	}
	if v := os.Getenv("ROUTE53_TTL"); v != "" {
		if ttl, err := strconv.ParseInt(v, 10, 64); err == nil {
			cfg.DHT.Bootstrap.Route53.TTL = ttl
		}
	}
	if v := os.Getenv("ROUTE53_REGION"); v != "" {
		cfg.DHT.Bootstrap.Route53.Region = v
	}
	if v := os.Getenv("LOGGER_ENABLED"); v != "" {
		v = strings.ToLower(v)
		cfg.Logger.Active = v == "true" || v == "1" || v == "yes"
	}
	if v := os.Getenv("LOGGER_LEVEL"); v != "" {
		cfg.Logger.Level = v
	}
	if v := os.Getenv("LOGGER_ENCODING"); v != "" {
		cfg.Logger.Encoding = v
	}
	if v := os.Getenv("LOGGER_MODE"); v != "" {
		cfg.Logger.Mode = v
	}
	if v := os.Getenv("LOGGER_FILE_PATH"); v != "" {
		cfg.Logger.File.Path = v
	}
}

// ValidateConfig performs structural validation of the loaded configuration.
//
// The validation checks only the syntactic and structural correctness of the
// configuration file, not the semantic correctness of protocol parameters.
// For example, it verifies that required fields are present, values are within
// valid ranges (e.g., port numbers, durations), and enum-like fields contain
// supported values, but it does not check whether the de Bruijn degree is a
// power of two or whether ID bits are consistent with the hash function.
//
// All detected issues are accumulated and returned as a single error. If the
// configuration is valid, the method returns nil.
func (cfg *Config) ValidateConfig() error {
	var errs []string

	// --- Logger ---
	switch cfg.Logger.Level {
	case "debug", "info", "warn", "error":
	default:
		errs = append(errs, fmt.Sprintf("invalid logger.level: %s", cfg.Logger.Level))
	}
	switch cfg.Logger.Encoding {
	case "console", "json":
	default:
		errs = append(errs, fmt.Sprintf("invalid logger.encoding: %s", cfg.Logger.Encoding))
	}
	switch cfg.Logger.Mode {
	case "stdout":
	case "file":
		if cfg.Logger.File.Path == "" {
			errs = append(errs, "logger.file.path is required when mode=file")
		}
		if cfg.Logger.File.MaxSize < 0 || cfg.Logger.File.MaxBackups < 0 || cfg.Logger.File.MaxAge < 0 {
			errs = append(errs, "logger.file.* values must be non-negative")
		}
	default:
		errs = append(errs, fmt.Sprintf("invalid logger.mode: %s", cfg.Logger.Mode))
	}

	// --- DHT ---
	if cfg.DHT.IDBits <= 0 {
		errs = append(errs, "dht.idBits must be > 0")
	}
	switch cfg.DHT.Mode {
	case "public", "private":
	default:
		errs = append(errs, fmt.Sprintf("invalid dht.mode: %s", cfg.DHT.Mode))
	}
	if cfg.DHT.DeBruijn.Degree <= 0 {
		errs = append(errs, "dht.deBruijn.degree must be > 0")
	}
	if cfg.DHT.DeBruijn.FixInterval <= 0 {
		errs = append(errs, "dht.deBruijn.fixInterval must be > 0")
	}
	if cfg.DHT.FaultTolerance.SuccessorListSize <= 0 {
		errs = append(errs, "dht.faultTolerance.successorListSize must be > 0")
	}
	if cfg.DHT.FaultTolerance.StabilizationInterval <= 0 {
		errs = append(errs, "dht.faultTolerance.stabilizationInterval must be > 0")
	}
	if cfg.DHT.FaultTolerance.FailureTimeout <= 0 {
		errs = append(errs, "dht.faultTolerance.failureTimeout must be > 0")
	}
	if cfg.DHT.DeBruijn.Degree > cfg.DHT.FaultTolerance.SuccessorListSize {
		errs = append(errs, "dht.deBruijn.degree must be <= dht.faultTolerance.successorListSize")
	}
	if cfg.DHT.IDBits%cfg.DHT.DeBruijn.Degree != 0 {
		errs = append(errs, "dht.idBits must be multiple of dht.deBruijn.degree")
	}

	// --- Bootstrap ---
	b := cfg.DHT.Bootstrap
	switch b.Mode {
	case "route53":
		if b.Route53.HostedZoneID == "" {
			errs = append(errs, "bootstrap.route53.hostedZoneId is required in mode=route53")
		}
		if b.Route53.DomainSuffix == "" {
			errs = append(errs, "bootstrap.route53.domainSuffix is required in mode=route53")
		}
		if b.Route53.TTL <= 0 {
			errs = append(errs, "bootstrap.route53.ttl must be > 0 in mode=route53")
		}
		if b.Route53.Region == "" {
			errs = append(errs, "bootstrap.route53.region is required in mode=route53")
		}
	case "static":
		for _, p := range b.Peers {
			if _, _, err := net.SplitHostPort(p); err != nil {
				errs = append(errs, fmt.Sprintf("invalid peer address %q in bootstrap.peers: %v", p, err))
			}
		}
	default:
		errs = append(errs, fmt.Sprintf("invalid bootstrap.mode: %s (must be dns, static or init)", b.Mode))
	}

	// --- Node ---
	if cfg.Node.Port < 0 || cfg.Node.Port > 65535 {
		errs = append(errs, fmt.Sprintf("node.port must be in [0,65535], got %d", cfg.Node.Port))
	}

	// --- Telemetry ---
	if cfg.Telemetry.Tracing.Enabled {
		switch cfg.Telemetry.Tracing.Exporter {
		case "stdout", "jaeger", "otlp":
		default:
			errs = append(errs, fmt.Sprintf("invalid telemetry.tracing.exporter: %s", cfg.Telemetry.Tracing.Exporter))
		}
		if cfg.Telemetry.Tracing.Endpoint == "" {
			errs = append(errs, "telemetry.tracing.endpoint is required")
		}
	}

	// --- Return result ---
	if len(errs) > 0 {
		return fmt.Errorf("configuration errors:\n  - %s", strings.Join(errs, "\n  - "))
	}
	return nil
}

// LogConfig prints the loaded configuration at DEBUG level.
// This is useful for debugging startup issues and verifying
// that the configuration file has been parsed correctly.
func (cfg *Config) LogConfig(lgr logger.Logger) {
	lgr.Info("Loaded configuration",
		// Logger
		logger.F("logger.active", cfg.Logger.Active),
		logger.F("logger.level", cfg.Logger.Level),
		logger.F("logger.encoding", cfg.Logger.Encoding),
		logger.F("logger.mode", cfg.Logger.Mode),
		logger.F("logger.file.path", cfg.Logger.File.Path),
		logger.F("logger.file.maxSizeMB", cfg.Logger.File.MaxSize),
		logger.F("logger.file.maxBackups", cfg.Logger.File.MaxBackups),
		logger.F("logger.file.maxAgeDays", cfg.Logger.File.MaxAge),
		logger.F("logger.file.compress", cfg.Logger.File.Compress),

		// DHT
		logger.F("dht.idBits", cfg.DHT.IDBits),
		logger.F("dht.mode", cfg.DHT.Mode),

		// de Bruijn
		logger.F("dht.deBruijn.degree", cfg.DHT.DeBruijn.Degree),
		logger.F("dht.deBruijn.backupSize", cfg.DHT.DeBruijn.BackupSize),
		logger.F("dht.deBruijn.fixInterval", cfg.DHT.DeBruijn.FixInterval.String()),
		logger.F("dht.deBruijn.fixIntervalMs", cfg.DHT.DeBruijn.FixInterval.Milliseconds()),

		// storage
		logger.F("dht.storage.fixInterval", cfg.DHT.Storage.FixInterval.String()),
		logger.F("dht.storage.fixIntervalMs", cfg.DHT.Storage.FixInterval.Milliseconds()),

		// fault tolerance
		logger.F("dht.faultTolerance.successorListSize", cfg.DHT.FaultTolerance.SuccessorListSize),
		logger.F("dht.faultTolerance.stabilizationInterval", cfg.DHT.FaultTolerance.StabilizationInterval.String()),
		logger.F("dht.faultTolerance.stabilizationIntervalMs", cfg.DHT.FaultTolerance.StabilizationInterval.Milliseconds()),
		logger.F("dht.faultTolerance.failureTimeout", cfg.DHT.FaultTolerance.FailureTimeout.String()),
		logger.F("dht.faultTolerance.failureTimeoutMs", cfg.DHT.FaultTolerance.FailureTimeout.Milliseconds()),

		// bootstrap
		logger.F("dht.bootstrap.mode", cfg.DHT.Bootstrap.Mode),
		logger.F("dht.bootstrap.peers", cfg.DHT.Bootstrap.Peers),

		// route53
		logger.F("dht.bootstrap.register.hostedZoneId", cfg.DHT.Bootstrap.Route53.HostedZoneID),
		logger.F("dht.bootstrap.register.domainSuffix", cfg.DHT.Bootstrap.Route53.DomainSuffix),
		logger.F("dht.bootstrap.register.ttl", cfg.DHT.Bootstrap.Route53.TTL),
		logger.F("dht.bootstrap.register.region", cfg.DHT.Bootstrap.Route53.Region),

		// Node
		logger.F("node.id", cfg.Node.Id),
		logger.F("node.host", cfg.Node.Bind),
		logger.F("node.port", cfg.Node.Port),

		// Telemetry
		logger.F("telemetry.tracing.enabled", cfg.Telemetry.Tracing.Enabled),
		logger.F("telemetry.tracing.exporter", cfg.Telemetry.Tracing.Exporter),
		logger.F("telemetry.tracing.endpoint", cfg.Telemetry.Tracing.Endpoint),
	)
}
