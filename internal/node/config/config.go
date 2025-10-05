package config

import (
	"KoordeDHT/internal/configloader"
	"KoordeDHT/internal/logger"
	"fmt"
	"net"
	"strings"
	"time"
)

type TracingConfig struct {
	Enabled  bool   `yaml:"enabled"`
	Exporter string `yaml:"exporter"`
	Endpoint string `yaml:"endpoint"`
}

type TelemetryConfig struct {
	Tracing TracingConfig `yaml:"tracing"`
}

type DeBruijnConfig struct {
	Degree      int           `yaml:"degree"`
	FixInterval time.Duration `yaml:"fixInterval"`
}

type FaultToleranceConfig struct {
	SuccessorListSize     int           `yaml:"successorListSize"`
	StabilizationInterval time.Duration `yaml:"stabilizationInterval"`
	FailureTimeout        time.Duration `yaml:"failureTimeout"`
}

type StorageConfig struct {
	FixInterval time.Duration `yaml:"fixInterval"`
}

type DHTConfig struct {
	IDBits         int                          `yaml:"idBits"`
	Mode           string                       `yaml:"mode"`
	DeBruijn       DeBruijnConfig               `yaml:"deBruijn"`
	FaultTolerance FaultToleranceConfig         `yaml:"faultTolerance"`
	Storage        StorageConfig                `yaml:"storage"`
	Bootstrap      configloader.BootstrapConfig `yaml:"bootstrap"`
}

type NodeConfig struct {
	Id   string `yaml:"id"`
	Bind string `yaml:"bind"`
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
}

type Config struct {
	Logger    configloader.LoggerConfig `yaml:"logger"`
	DHT       DHTConfig                 `yaml:"dht"`
	Node      NodeConfig                `yaml:"node"`
	Telemetry TelemetryConfig           `yaml:"telemetry"`
}

func LoadConfig(path string) (*Config, error) {
	cfg := &Config{}
	// Load from YAML file
	if err := configloader.LoadYAML(path, cfg); err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	// Override with environment variables
	configloader.OverrideString(&cfg.Node.Id, "NODE_ID")
	configloader.OverrideString(&cfg.Node.Bind, "NODE_BIND")
	configloader.OverrideString(&cfg.Node.Host, "NODE_HOST")
	configloader.OverrideInt(&cfg.Node.Port, "NODE_PORT")

	configloader.OverrideString(&cfg.DHT.Mode, "DHT_MODE")
	configloader.OverrideInt(&cfg.DHT.IDBits, "DHT_ID_BITS")

	configloader.OverrideInt(&cfg.DHT.DeBruijn.Degree, "DEBRUIJN_DEGREE")
	configloader.OverrideDuration(&cfg.DHT.DeBruijn.FixInterval, "DEBRUIJN_FIX_INTERVAL")

	configloader.OverrideInt(&cfg.DHT.FaultTolerance.SuccessorListSize, "SUCCESSOR_LIST_SIZE")
	configloader.OverrideDuration(&cfg.DHT.FaultTolerance.StabilizationInterval, "STABILIZATION_INTERVAL")
	configloader.OverrideDuration(&cfg.DHT.FaultTolerance.FailureTimeout, "FAILURE_TIMEOUT")

	configloader.OverrideDuration(&cfg.DHT.Storage.FixInterval, "STORAGE_FIX_INTERVAL")

	configloader.OverrideString(&cfg.DHT.Bootstrap.Mode, "BOOTSTRAP_MODE")
	configloader.OverrideStringSlice(&cfg.DHT.Bootstrap.Peers, "BOOTSTRAP_PEERS") // comma-separated list

	configloader.OverrideString(&cfg.DHT.Bootstrap.Route53.HostedZoneID, "ROUTE53_ZONE_ID")
	configloader.OverrideString(&cfg.DHT.Bootstrap.Route53.DomainSuffix, "ROUTE53_SUFFIX")
	configloader.OverrideInt64(&cfg.DHT.Bootstrap.Route53.TTL, "ROUTE53_TTL")
	configloader.OverrideString(&cfg.DHT.Bootstrap.Route53.Region, "ROUTE53_REGION")

	configloader.OverrideBool(&cfg.Telemetry.Tracing.Enabled, "TRACING_ENABLED")
	configloader.OverrideString(&cfg.Telemetry.Tracing.Exporter, "TRACING_EXPORTER")
	configloader.OverrideString(&cfg.Telemetry.Tracing.Endpoint, "TRACING_ENDPOINT")

	configloader.OverrideBool(&cfg.Logger.Active, "LOGGER_ENABLED")
	configloader.OverrideString(&cfg.Logger.Level, "LOGGER_LEVEL")
	configloader.OverrideString(&cfg.Logger.Encoding, "LOGGER_ENCODING")
	configloader.OverrideString(&cfg.Logger.Mode, "LOGGER_MODE")
	configloader.OverrideString(&cfg.Logger.File.Path, "LOGGER_FILE_PATH")
	configloader.OverrideInt(&cfg.Logger.File.MaxSize, "LOGGER_FILE_MAX_SIZE")
	configloader.OverrideInt(&cfg.Logger.File.MaxBackups, "LOGGER_FILE_MAX_BACKUPS")
	configloader.OverrideInt(&cfg.Logger.File.MaxAge, "LOGGER_FILE_MAX_AGE")
	configloader.OverrideBool(&cfg.Logger.File.Compress, "LOGGER_FILE_COMPRESS")

	// Apply defaults
	if cfg.Node.Bind == "" {
		cfg.Node.Bind = "0.0.0.0"
	}

	return cfg, nil
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
