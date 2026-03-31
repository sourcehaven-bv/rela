package project

import (
	"os"

	"gopkg.in/yaml.v3"
)

// ConfigFile is the conventional filename for project configuration.
const ConfigFile = "config.yaml"

// Config holds project-level settings stored in .rela/config.yaml.
type Config struct {
	// Formatting configures how markdown content is formatted.
	Formatting FormattingConfig `yaml:"formatting,omitempty"`
}

// FormattingConfig holds settings for markdown formatting.
type FormattingConfig struct {
	// LineWidth is the maximum line width for paragraph wrapping.
	// Default is 80 if not specified.
	LineWidth int `yaml:"line_width,omitempty"`
}

// DefaultConfig returns a Config with default values.
func DefaultConfig() *Config {
	return &Config{
		Formatting: FormattingConfig{
			LineWidth: 80,
		},
	}
}

// LoadConfig loads the project configuration from .rela/config.yaml.
// If the file doesn't exist, returns default configuration.
func LoadConfig(ctx *Context) (*Config, error) {
	return LoadConfigFromPath(ctx.ConfigPath())
}

// LoadConfigFromPath loads project configuration from a specific path.
// If the file doesn't exist, returns default configuration.
func LoadConfigFromPath(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultConfig(), nil
		}
		return nil, err
	}

	cfg := DefaultConfig()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	// Apply defaults for zero values
	if cfg.Formatting.LineWidth <= 0 {
		cfg.Formatting.LineWidth = 80
	}

	return cfg, nil
}

// ConfigPath returns the path to the project configuration file.
func (c *Context) ConfigPath() string {
	return c.CacheDir + "/" + ConfigFile
}
