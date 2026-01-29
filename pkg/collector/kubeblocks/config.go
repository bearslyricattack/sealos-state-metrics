// Package kubeblocks provides a collector for monitoring KubeBlocks Cluster resources.
package kubeblocks

import "time"

// Config holds the configuration for the KubeBlocks Cluster collector
type Config struct {
	// Namespaces to watch (empty means all namespaces)
	Namespaces []string `yaml:"namespaces" env:"NAMESPACES" envSeparator:","`

	// ResyncPeriod is the resync interval for the informer
	ResyncPeriod time.Duration `yaml:"resyncPeriod" env:"RESYNC_PERIOD"`
}

// NewDefaultConfig creates a new Config with default values
func NewDefaultConfig() *Config {
	return &Config{
		Namespaces:   []string{}, // Empty = all namespaces
		ResyncPeriod: 10 * time.Minute,
	}
}
