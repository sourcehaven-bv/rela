// Package scheduler runs Lua scripts on cron-like schedules.
// It provides a long-running scheduler that executes project scripts at
// configured intervals, with missed-run detection, overlap prevention,
// and graceful shutdown.
package scheduler

import (
	"fmt"
	"time"

	"github.com/robfig/cron/v3"
	"gopkg.in/yaml.v3"
)

// ConfigFile is the name of the scheduler configuration file in the project root.
const ConfigFile = "schedules.yaml"

// Config is the top-level scheduler configuration loaded from schedules.yaml.
type Config struct {
	Tasks []TaskConfig `yaml:"tasks"`
}

// TaskConfig defines a single scheduled task.
type TaskConfig struct {
	Name     string        `yaml:"name"`
	Script   string        `yaml:"script"`
	Schedule string        `yaml:"schedule"`
	Timeout  time.Duration `yaml:"timeout"`
}

// DefaultTimeout is the default execution timeout for a task when none is configured.
const DefaultTimeout = 5 * time.Minute

// ParseConfig parses and validates scheduler configuration from YAML bytes.
func ParseConfig(data []byte) (*Config, error) {
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse schedules.yaml: %w", err)
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func (c *Config) validate() error {
	if len(c.Tasks) == 0 {
		return nil // empty config is valid — scheduler runs but does nothing
	}

	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	seen := make(map[string]struct{}, len(c.Tasks))

	for i, t := range c.Tasks {
		if t.Name == "" {
			return fmt.Errorf("task %d: name is required", i)
		}
		if _, dup := seen[t.Name]; dup {
			return fmt.Errorf("task %q: duplicate task name", t.Name)
		}
		seen[t.Name] = struct{}{}

		if t.Script == "" {
			return fmt.Errorf("task %q: script is required", t.Name)
		}
		if t.Schedule == "" {
			return fmt.Errorf("task %q: schedule is required", t.Name)
		}
		if _, err := parser.Parse(t.Schedule); err != nil {
			return fmt.Errorf("task %q: invalid cron expression %q: %w", t.Name, t.Schedule, err)
		}
		if t.Timeout < 0 {
			return fmt.Errorf("task %q: timeout must not be negative", t.Name)
		}
	}

	return nil
}
