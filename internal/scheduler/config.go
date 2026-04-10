// Package scheduler runs Lua scripts on simple recurring schedules.
// It provides a long-running, single-threaded scheduler that executes
// project scripts sequentially with missed-run detection and graceful shutdown.
package scheduler

import (
	"fmt"
	"regexp"
	"strconv"
	"time"

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
	Name   string   `yaml:"name"`
	Script string   `yaml:"script"`
	Every  Schedule `yaml:"every"`
}

// Schedule represents a recurring schedule interval.
// Supported values: "day", "week", or a duration like "30m", "2h", "1h30m".
type Schedule struct {
	kind     scheduleKind
	interval time.Duration // only for intervalKind
	set      bool          // true after successful parse
}

type scheduleKind int

const (
	dayKind scheduleKind = iota
	weekKind
	intervalKind
)

// IsDue returns true if enough time has passed since lastRun for the next
// execution. For day/week schedules, it checks whether the calendar boundary
// (midnight local time / Monday midnight) has been crossed.
func (s Schedule) IsDue(lastRun, now time.Time) bool {
	switch s.kind {
	case dayKind:
		// Due if now is on a different day than lastRun (local time).
		return truncateToDay(now) != truncateToDay(lastRun)
	case weekKind:
		// Due if now is in a different ISO week than lastRun.
		return truncateToWeek(now) != truncateToWeek(lastRun)
	case intervalKind:
		return now.Sub(lastRun) >= s.interval
	}
	return false
}

func truncateToDay(t time.Time) int {
	y, m, d := t.Date()
	return y*10000 + int(m)*100 + d
}

func truncateToWeek(t time.Time) int {
	y, w := t.ISOWeek()
	return y*100 + w
}

// String returns a human-readable representation of the schedule.
func (s Schedule) String() string {
	switch s.kind {
	case dayKind:
		return "day"
	case weekKind:
		return "week"
	case intervalKind:
		return s.interval.String()
	}
	return "unknown"
}

var durationRe = regexp.MustCompile(`^\d+[mhMH]`)

// UnmarshalYAML implements yaml.Unmarshaler for Schedule.
func (s *Schedule) UnmarshalYAML(value *yaml.Node) error {
	var raw string
	if err := value.Decode(&raw); err != nil {
		return err
	}
	parsed, err := parseSchedule(raw)
	if err != nil {
		return err
	}
	*s = parsed
	return nil
}

// MarshalYAML implements yaml.Marshaler for Schedule.
func (s Schedule) MarshalYAML() (interface{}, error) {
	return s.String(), nil
}

func parseSchedule(raw string) (Schedule, error) {
	switch raw {
	case "day":
		return Schedule{kind: dayKind, set: true}, nil
	case "week":
		return Schedule{kind: weekKind, set: true}, nil
	}

	// Try Go duration (e.g. "30m", "2h", "1h30m")
	if durationRe.MatchString(raw) {
		d, err := time.ParseDuration(raw)
		if err != nil {
			return Schedule{}, fmt.Errorf("invalid schedule %q: %w", raw, err)
		}
		if d <= 0 {
			return Schedule{}, fmt.Errorf("invalid schedule %q: must be positive", raw)
		}
		return Schedule{kind: intervalKind, interval: d, set: true}, nil
	}

	// Try bare number as minutes (e.g. "30" = 30m)
	if n, err := strconv.Atoi(raw); err == nil {
		if n <= 0 {
			return Schedule{}, fmt.Errorf("invalid schedule %q: must be positive", raw)
		}
		return Schedule{kind: intervalKind, interval: time.Duration(n) * time.Minute, set: true}, nil
	}

	return Schedule{}, fmt.Errorf(
		"invalid schedule %q: use \"day\", \"week\", or a duration like \"30m\", \"2h\"",
		raw,
	)
}

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
		return nil // empty config is valid
	}

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
		if !t.Every.set {
			return fmt.Errorf("task %q: every is required", t.Name)
		}
	}

	return nil
}
