package namespace

// Config contains configuration for the Namespace collector.
//
// RequiredLabels lists labels that every namespace is expected to have. A
// namespace that is missing one or more of these labels is reported by the
// collector.
type Config struct {
	RequiredLabels []string `yaml:"requiredLabels" env:"REQUIRED_LABELS" envSeparator:","`
}

// NewDefaultConfig returns the default configuration for the Namespace
// collector. With no required labels configured, the collector remains
// active but reports a zero count.
func NewDefaultConfig() *Config {
	return &Config{RequiredLabels: []string{}}
}
