package cert

import (
	"github.com/caarlos0/env/v9"
)

// Config contains configuration for the Certificate collector
type Config struct {
	Enabled           bool     `yaml:"enabled" env:"ENABLED" envDefault:"true"`
	Namespaces        []string `yaml:"namespaces" env:"NAMESPACES" envSeparator:","`
	ExpiryWarningDays int      `yaml:"expiryWarningDays" env:"EXPIRY_WARNING_DAYS" envDefault:"30"`
}

// NewDefaultConfig returns the default configuration for Certificate collector
func NewDefaultConfig() *Config {
	cfg := &Config{}
	_ = env.ParseWithOptions(cfg, env.Options{Prefix: "CERT_COLLECTOR_"})
	return cfg
}
