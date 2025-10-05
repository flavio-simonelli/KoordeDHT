package configloader

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
