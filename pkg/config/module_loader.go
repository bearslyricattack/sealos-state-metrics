package config

import (
	"fmt"
	"os"

	"github.com/mitchellh/mapstructure"
	"gopkg.in/yaml.v3"
	"k8s.io/klog/v2"
)

// ModuleConfigLoader loads module-specific configuration from YAML files
type ModuleConfigLoader struct {
	configFile string
	tagName    string
	decodeHook mapstructure.DecodeHookFunc
}

// ModuleLoaderOption is a function that configures ModuleConfigLoader
type ModuleLoaderOption func(*ModuleConfigLoader)

// WithModuleTagName sets the struct tag name to use for decoding
// Default is "yaml"
func WithModuleTagName(tagName string) ModuleLoaderOption {
	return func(l *ModuleConfigLoader) {
		l.tagName = tagName
	}
}

// WithModuleDecodeHook sets a custom decode hook function for type conversions
// Example: WithModuleDecodeHook(mapstructure.StringToTimeDurationHookFunc())
// For multiple hooks, use: WithModuleDecodeHook(mapstructure.ComposeDecodeHookFunc(hook1, hook2, ...))
func WithModuleDecodeHook(hook mapstructure.DecodeHookFunc) ModuleLoaderOption {
	return func(l *ModuleConfigLoader) {
		l.decodeHook = hook
	}
}

// NewModuleConfigLoader creates a new module config loader
func NewModuleConfigLoader(configFile string, opts ...ModuleLoaderOption) *ModuleConfigLoader {
	loader := &ModuleConfigLoader{
		configFile: configFile,
		tagName:    "yaml",
		decodeHook: nil, // No default decode hook
	}

	for _, opt := range opts {
		opt(loader)
	}

	return loader
}

// LoadModuleConfig loads module-specific configuration from YAML file
// moduleKey is the path in YAML like "collectors.node"
// target is the struct to decode into
func (l *ModuleConfigLoader) LoadModuleConfig(moduleKey string, target interface{}) error {
	if l.configFile == "" {
		klog.V(4).InfoS("No config file provided, skipping module config load", "module", moduleKey)
		return nil
	}

	data, err := os.ReadFile(l.configFile)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	// Parse full config
	var fullConfig map[string]interface{}
	if err := yaml.Unmarshal(data, &fullConfig); err != nil {
		return fmt.Errorf("failed to unmarshal YAML: %w", err)
	}

	// Navigate to module section (e.g., "collectors.node")
	moduleData, ok := navigateToKey(fullConfig, moduleKey)
	if !ok {
		klog.V(4).InfoS("Config key not found", "module", moduleKey)
		return nil
	}

	// Decode using mapstructure
	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		TagName:          l.tagName,
		Result:           target,
		DecodeHook:       l.decodeHook,       // Apply decode hook for custom types (e.g., time.Duration)
		WeaklyTypedInput: true,               // Allow flexible basic type conversions
	})
	if err != nil {
		return fmt.Errorf("failed to create decoder: %w", err)
	}

	if err := decoder.Decode(moduleData); err != nil {
		return fmt.Errorf("failed to decode module config: %w", err)
	}

	klog.V(4).InfoS("Module config loaded from file", "module", moduleKey, "file", l.configFile)
	return nil
}

// navigateToKey navigates through nested map to find the value at the given key path
func navigateToKey(data map[string]interface{}, key string) (map[string]interface{}, bool) {
	keys := splitKey(key)
	current := data

	for _, k := range keys {
		next, exists := current[k]
		if !exists {
			return nil, false
		}

		// Navigate deeper
		m, ok := next.(map[string]interface{})
		if !ok {
			return nil, false
		}
		current = m
	}

	return current, true
}

// splitKey splits a dot-separated key like "collectors.node" into ["collectors", "node"]
func splitKey(key string) []string {
	if key == "" {
		return []string{}
	}

	result := []string{}
	current := ""
	for _, c := range key {
		if c == '.' {
			if current != "" {
				result = append(result, current)
				current = ""
			}
		} else {
			current += string(c)
		}
	}
	if current != "" {
		result = append(result, current)
	}
	return result
}
