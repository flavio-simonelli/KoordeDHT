package tester

import (
	"KoordeDHT/internal/configloader"
	"KoordeDHT/internal/logger"
	"fmt"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// SimulationConfig controls the overall test runtime.
type SimulationConfig struct {
	Duration time.Duration `yaml:"duration"`
}

// DHTConfig defines the Koorde DHT keyspace parameters used by the tester.
type DHTConfig struct {
	IDBits int `yaml:"idBits"` // number of bits in the identifier space
}

// DockerBootstrapConfig contains Docker-specific bootstrap parameters.
type DockerBootstrapConfig struct {
	ContainerSuffix string `yaml:"containerSuffix"`
	Network         string `yaml:"network"`
	Port            int    `yaml:"port"`
}

// BootstrapConfig defines the discovery mechanism.
type BootstrapConfig struct {
	Mode    string                     `yaml:"mode"` // docker | route53
	Route53 configloader.Route53Config `yaml:"route53"`
	Docker  DockerBootstrapConfig      `yaml:"docker"`
}

// CSVConfig defines CSV export options.
type CSVConfig struct {
	Enabled bool   `yaml:"enabled"`
	Path    string `yaml:"path"`
}

// ParallelismConfig defines how many concurrent workers are used.
type ParallelismConfig struct {
	MinWorkers int `yaml:"min"`
	MaxWorkers int `yaml:"max"`
}

// QueryConfig defines how queries are generated.
type QueryConfig struct {
	Rate        float64           `yaml:"rate"` // global requests per second
	Timeout     time.Duration     `yaml:"timeout"`
	Parallelism ParallelismConfig `yaml:"parallelism"` // worker concurrency
}

// Config is the root configuration for the KoordeDHT tester client.
type Config struct {
	Logger     configloader.LoggerConfig `yaml:"logger"`
	Simulation SimulationConfig          `yaml:"simulation"`
	DHT        DHTConfig                 `yaml:"dht"`
	Bootstrap  BootstrapConfig           `yaml:"bootstrap"`
	CSV        CSVConfig                 `yaml:"csv"`
	Query      QueryConfig               `yaml:"query"`
}

// Load reads the configuration file and applies environment overrides.
func Load(path string) (*Config, error) {
	cfg := &Config{}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse yaml: %w", err)
	}

	// Environment overrides
	configloader.OverrideBool(&cfg.Logger.Active, "LOGGER_ACTIVE")
	configloader.OverrideString(&cfg.Logger.Level, "LOGGER_LEVEL")
	configloader.OverrideString(&cfg.Logger.Encoding, "LOGGER_ENCODING")
	configloader.OverrideString(&cfg.Logger.Mode, "LOGGER_MODE")
	configloader.OverrideString(&cfg.Logger.File.Path, "LOGGER_FILE_PATH")
	configloader.OverrideInt(&cfg.Logger.File.MaxSize, "LOGGER_FILE_MAXSIZE")
	configloader.OverrideInt(&cfg.Logger.File.MaxBackups, "LOGGER_FILE_MAXBACKUPS")
	configloader.OverrideInt(&cfg.Logger.File.MaxAge, "LOGGER_FILE_MAXAGE")
	configloader.OverrideBool(&cfg.Logger.File.Compress, "LOGGER_FILE_COMPRESS")

	configloader.OverrideDuration(&cfg.Simulation.Duration, "SIM_DURATION")
	configloader.OverrideInt(&cfg.DHT.IDBits, "DHT_ID_BITS")

	configloader.OverrideString(&cfg.Bootstrap.Mode, "BOOTSTRAP_MODE")

	configloader.OverrideString(&cfg.Bootstrap.Docker.ContainerSuffix, "DOCKER_SUFFIX")
	configloader.OverrideString(&cfg.Bootstrap.Docker.Network, "DOCKER_NETWORK")
	configloader.OverrideInt(&cfg.Bootstrap.Docker.Port, "DOCKER_PORT")

	configloader.OverrideString(&cfg.Bootstrap.Route53.HostedZoneID, "ROUTE53_ZONE_ID")
	configloader.OverrideString(&cfg.Bootstrap.Route53.DomainSuffix, "ROUTE53_DOMAIN_SUFFIX")
	configloader.OverrideInt64(&cfg.Bootstrap.Route53.TTL, "ROUTE53_TTL")
	configloader.OverrideString(&cfg.Bootstrap.Route53.Region, "ROUTE53_REGION")

	configloader.OverrideBool(&cfg.CSV.Enabled, "CSV_ENABLED")
	configloader.OverrideString(&cfg.CSV.Path, "CSV_PATH")

	configloader.OverrideFloat(&cfg.Query.Rate, "QUERY_RATE")
	configloader.OverrideDuration(&cfg.Query.Timeout, "QUERY_TIMEOUT")
	configloader.OverrideInt(&cfg.Query.Parallelism.MinWorkers, "QUERY_PARALLELISM_MIN")
	configloader.OverrideInt(&cfg.Query.Parallelism.MaxWorkers, "QUERY_PARALLELISM_MAX")

	return cfg, nil
}

func (c *Config) Validate() error {
	var errs []string

	// Logger
	if c.Logger.Active {
		switch c.Logger.Level {
		case "debug", "info", "warn", "error":
		default:
			errs = append(errs, fmt.Sprintf("logger.level must be one of [debug, info, warn, error], got %q", c.Logger.Level))
		}
		if c.Logger.Mode == "file" && c.Logger.File.Path == "" {
			errs = append(errs, "logger.file.path must be set when logger.mode = file")
		}
	}

	// Simulation
	if c.Simulation.Duration <= 0 {
		errs = append(errs, fmt.Sprintf("simulation.duration must be > 0 (got %v)", c.Simulation.Duration))
	}

	// DHT
	if c.DHT.IDBits <= 0 {
		errs = append(errs, fmt.Sprintf("dht.idBits must be > 0 (got %d)", c.DHT.IDBits))
	}

	// Bootstrap
	switch c.Bootstrap.Mode {
	case "docker":
		d := c.Bootstrap.Docker
		if d.ContainerSuffix == "" {
			errs = append(errs, "bootstrap.docker.containerSuffix must not be empty when mode = docker")
		}
		if d.Port <= 0 {
			errs = append(errs, fmt.Sprintf("bootstrap.docker.port must be > 0 (got %d)", d.Port))
		}
	case "route53":
		r := c.Bootstrap.Route53
		if r.HostedZoneID == "" {
			errs = append(errs, "bootstrap.route53.hostedZoneId must not be empty when mode = route53")
		}
		if r.DomainSuffix == "" {
			errs = append(errs, "bootstrap.route53.domainSuffix must not be empty when mode = route53")
		}
		if r.Region == "" {
			errs = append(errs, "bootstrap.route53.region must not be empty when mode = route53")
		}
	default:
		errs = append(errs, fmt.Sprintf("bootstrap.mode must be one of [docker, route53], got %q", c.Bootstrap.Mode))
	}

	// CSV
	if c.CSV.Enabled && c.CSV.Path == "" {
		errs = append(errs, "csv.path must be set when csv.enabled = true")
	}

	// Query
	if c.Query.Rate <= 0 {
		errs = append(errs, fmt.Sprintf("query.rate must be > 0 (got %f)", c.Query.Rate))
	}
	if c.Query.Parallelism.MinWorkers <= 0 {
		errs = append(errs, fmt.Sprintf("query.parallelism.min must be > 0 (got %d)", c.Query.Parallelism.MinWorkers))
	}
	if c.Query.Parallelism.MaxWorkers < c.Query.Parallelism.MinWorkers {
		errs = append(errs, fmt.Sprintf("query.parallelism.max must be >= min (got %d < %d)",
			c.Query.Parallelism.MaxWorkers, c.Query.Parallelism.MinWorkers))
	}

	if len(errs) > 0 {
		return fmt.Errorf("configuration errors:\n  - %s", strings.Join(errs, "\n  - "))
	}
	return nil
}

func (cfg *Config) LogConfig(lgr logger.Logger) {
	lgr.Info("Loaded tester configuration",
		logger.F("logger.active", cfg.Logger.Active),
		logger.F("logger.level", cfg.Logger.Level),
		logger.F("logger.encoding", cfg.Logger.Encoding),
		logger.F("logger.mode", cfg.Logger.Mode),

		logger.F("simulation.duration", cfg.Simulation.Duration.String()),

		logger.F("dht.idBits", cfg.DHT.IDBits),

		logger.F("bootstrap.mode", cfg.Bootstrap.Mode),
		logger.F("bootstrap.docker.suffix", cfg.Bootstrap.Docker.ContainerSuffix),
		logger.F("bootstrap.docker.network", cfg.Bootstrap.Docker.Network),
		logger.F("bootstrap.docker.port", cfg.Bootstrap.Docker.Port),

		logger.F("csv.enabled", cfg.CSV.Enabled),
		logger.F("csv.path", cfg.CSV.Path),

		logger.F("query.rate", cfg.Query.Rate),
		logger.F("query.parallelism.min", cfg.Query.Parallelism.MinWorkers),
		logger.F("query.parallelism.max", cfg.Query.Parallelism.MaxWorkers),
	)
}
