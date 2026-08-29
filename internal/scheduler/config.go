// Package scheduler runs Lua scripts on simple recurring schedules.
// It provides a long-running, single-threaded scheduler that executes
// project scripts sequentially with missed-run detection and graceful shutdown.
package scheduler

import (
	"bytes"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/filter"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"

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
	Name     string   `yaml:"name"`
	Script   string   `yaml:"script,omitempty"`
	Template string   `yaml:"template,omitempty"`
	Every    Schedule `yaml:"every"`

	// RunAs is the IDENTITY this task runs as — not a capability
	// (DEC-O59WM4). What the task may read is decided entirely by
	// acl.yaml: assignments map this principal to roles, exactly like a
	// human user. Naming a principal here grants nothing by itself.
	//
	// Empty (the default) means [principal.UserScheduler] — the fixed
	// "system:scheduler" identity every task shares. Set it to give a job
	// its own identity, which both narrows what it can read (via a scoped
	// role) and makes the audit log name the specific job rather than a
	// generic scheduler.
	//
	// A task whose principal has no read grants reads nothing: privileges
	// are granted in acl.yaml, never inferred from task config.
	RunAs string `yaml:"run_as"`

	// Capabilities declares which ambient capabilities this task's script may
	// reach — http, ai, write_file, named secrets (TKT-YH52OM). Omitting it
	// grants none.
	//
	// This is the capability half of the same split RunAs documents for
	// identity: RunAs says WHO the task is (and thus what it may read from the
	// graph, via acl.yaml), Capabilities says what it may reach OUTSIDE the
	// graph. Neither is inferred from the other — a task running as a
	// privileged principal still reaches no network unless it says so here.
	//
	// schedules.yaml is operator-authored, but a scheduled task is not an
	// operator SHELL: it runs unattended in the server process, so it gets the
	// declared grant rather than the trusted default `rela script` uses.
	Capabilities metamodel.Capabilities `yaml:"capabilities,omitempty"`

	// ForEach expands one calendar occurrence into independently retried jobs,
	// one per matching entity. Its selected entity supplies the execution
	// principal, so ForEach and RunAs are mutually exclusive.
	ForEach *ForEachConfig `yaml:"for_each,omitempty"`
}

// ForEachConfig selects the entities a scheduled task runs as.
type ForEachConfig struct {
	EntityType string   `yaml:"entity_type"`
	Where      []string `yaml:"where,omitempty"`
	Limit      int      `yaml:"limit,omitempty"`
}

const (
	defaultForEachLimit = 1000
	maxForEachLimit     = 10000
)

// EffectiveLimit returns the configured fan-out bound or its safe default.
func (c *ForEachConfig) EffectiveLimit() int {
	if c == nil || c.Limit == 0 {
		return defaultForEachLimit
	}
	return c.Limit
}

// Schedule represents a recurring schedule interval.
// Supported values: "day", a weekday name ("monday".."sunday"), "week" (alias
// for "monday"), or a duration like "30m", "2h", "1h30m".
type Schedule struct {
	kind     scheduleKind
	weekday  time.Weekday  // only for weekdayKind
	interval time.Duration // only for intervalKind
	set      bool          // true after successful parse
}

type scheduleKind int

const (
	dayKind scheduleKind = iota
	weekdayKind
	intervalKind
)

// IsDue returns true if enough time has passed since lastRun for the next
// execution. For day schedules, it checks whether the day changed. For weekday
// schedules, it checks whether the target weekday has occurred since lastRun.
func (s Schedule) IsDue(lastRun, now time.Time) bool {
	switch s.kind {
	case dayKind:
		return truncateToDay(now) != truncateToDay(lastRun)
	case weekdayKind:
		// Due if the target weekday has occurred between lastRun and now.
		// Find the most recent occurrence of the target weekday at midnight.
		target := mostRecentWeekday(now, s.weekday)
		return target.After(lastRun)
	case intervalKind:
		return now.Sub(lastRun) >= s.interval
	}
	return false
}

// mostRecentWeekday returns midnight (local time) of the most recent
// occurrence of the given weekday, on or before the given time.
func mostRecentWeekday(t time.Time, wd time.Weekday) time.Time {
	y, m, d := t.Date()
	today := time.Date(y, m, d, 0, 0, 0, 0, t.Location())
	daysBack := (int(today.Weekday()) - int(wd) + 7) % 7
	return today.AddDate(0, 0, -daysBack)
}

func truncateToDay(t time.Time) int {
	y, m, d := t.Date()
	return y*10000 + int(m)*100 + d
}

// weekdayNames maps lowercase day names to time.Weekday.
var weekdayNames = map[string]time.Weekday{
	"monday":    time.Monday,
	"tuesday":   time.Tuesday,
	"wednesday": time.Wednesday,
	"thursday":  time.Thursday,
	"friday":    time.Friday,
	"saturday":  time.Saturday,
	"sunday":    time.Sunday,
}

// String returns a human-readable representation of the schedule.
func (s Schedule) String() string {
	switch s.kind {
	case dayKind:
		return "day"
	case weekdayKind:
		return s.weekday.String()
	case intervalKind:
		return s.interval.String()
	}
	return "unknown"
}

// Occurrence returns the stable local-date identity for a calendar schedule.
// Interval schedules deliberately have no occurrence identity.
func (s Schedule) Occurrence(now time.Time) (string, bool) {
	switch s.kind {
	case dayKind:
		return now.Format(time.DateOnly), true
	case weekdayKind:
		return mostRecentWeekday(now, s.weekday).Format(time.DateOnly), true
	default:
		return "", false
	}
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
func (s Schedule) MarshalYAML() (any, error) {
	return s.String(), nil
}

func parseSchedule(raw string) (Schedule, error) {
	if raw == "day" {
		return Schedule{kind: dayKind, set: true}, nil
	}

	// "week" is an alias for "monday".
	if raw == "week" {
		return Schedule{kind: weekdayKind, weekday: time.Monday, set: true}, nil
	}

	// Check for weekday names (monday, tuesday, ..., sunday).
	if wd, ok := weekdayNames[raw]; ok {
		return Schedule{kind: weekdayKind, weekday: wd, set: true}, nil
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
		"invalid schedule %q: use \"day\", a weekday name, or a duration like \"30m\", \"2h\"",
		raw,
	)
}

// ParseConfig parses and validates scheduler configuration from YAML bytes.
func ParseConfig(data []byte) (*Config, error) {
	var cfg Config
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&cfg); err != nil {
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
		if err := validateTask(i, t, seen); err != nil {
			return err
		}
	}

	return nil
}

func validateTask(i int, t TaskConfig, seen map[string]struct{}) error {
	if t.Name == "" {
		return fmt.Errorf("task %d: name is required", i)
	}
	if _, dup := seen[t.Name]; dup {
		return fmt.Errorf("task %q: duplicate task name", t.Name)
	}
	seen[t.Name] = struct{}{}

	if (t.Script == "") == (t.Template == "") {
		return fmt.Errorf("task %q: exactly one of script or template is required", t.Name)
	}
	if !t.Every.set {
		return fmt.Errorf("task %q: every is required", t.Name)
	}
	if t.ForEach != nil {
		if t.RunAs != "" {
			return fmt.Errorf("task %q: run_as and for_each are mutually exclusive", t.Name)
		}
		if t.ForEach.EntityType == "" {
			return fmt.Errorf("task %q: for_each.entity_type is required", t.Name)
		}
		if _, ok := t.Every.Occurrence(time.Now()); !ok {
			return fmt.Errorf("task %q: for_each requires a calendar schedule (day, week, or weekday)", t.Name)
		}
		if t.ForEach.Limit < 0 || t.ForEach.Limit > maxForEachLimit {
			return fmt.Errorf("task %q: for_each.limit must be between 1 and %d", t.Name, maxForEachLimit)
		}
		if _, err := filter.ParseAll(t.ForEach.Where); err != nil {
			return fmt.Errorf("task %q: for_each.where: %w", t.Name, err)
		}
	}
	if t.Template != "" && t.ForEach == nil {
		return fmt.Errorf("task %q: template requires for_each", t.Name)
	}
	return nil
}

// ValidateMetamodel checks the schema-dependent part of for_each config.
func (c *Config) ValidateMetamodel(meta *metamodel.Metamodel) error {
	if meta == nil {
		return errors.New("scheduler: metamodel must not be nil")
	}
	for _, task := range c.Tasks {
		if task.ForEach == nil {
			continue
		}
		if _, ok := meta.GetEntityDef(task.ForEach.EntityType); !ok {
			return fmt.Errorf("task %q: for_each.entity_type %q is unknown", task.Name, task.ForEach.EntityType)
		}
	}
	return nil
}
