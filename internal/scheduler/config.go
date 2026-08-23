// Package scheduler runs Lua scripts on simple recurring schedules.
// It provides a long-running, single-threaded scheduler that executes
// project scripts sequentially with missed-run detection and graceful shutdown.
package scheduler

import (
	"fmt"
	"regexp"
	"strconv"
	"time"

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
	Name   string   `yaml:"name"`
	Script string   `yaml:"script"`
	Every  Schedule `yaml:"every"`

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

// NextRun returns when the schedule fires again after lastRun.
//
// This is the scheduler's half of the cadence-to-deadline mapping: the job
// queue deliberately knows nothing about schedules, so a recurring task
// expresses "do not keep retrying past my next run" by passing this value as
// the job's deadline. A 60s task therefore stops retrying just before the next
// tick re-submits it, while a daily task gets a real backoff window — one
// generic primitive, both behaviors, no cadence concept in the queue.
//
// It is the inverse of [Schedule.IsDue]: NextRun(lastRun) is the earliest time
// for which IsDue(lastRun, t) holds.
func (s Schedule) NextRun(lastRun time.Time) time.Time {
	// The zero Schedule has kind == dayKind (iota 0), so an unparsed value
	// would otherwise be treated as a daily schedule. `set` is the only
	// reliable "was this parsed" signal.
	if !s.set {
		return time.Time{}
	}
	switch s.kind {
	case dayKind:
		// IsDue fires once the calendar day changes, so the next run is the
		// following local midnight.
		y, m, d := lastRun.Date()
		return time.Date(y, m, d, 0, 0, 0, 0, lastRun.Location()).AddDate(0, 0, 1)
	case weekdayKind:
		// The next occurrence of the target weekday STRICTLY after lastRun.
		// Landing exactly on lastRun would yield a deadline already reached,
		// which would suppress every retry.
		y, m, d := lastRun.Date()
		day := time.Date(y, m, d, 0, 0, 0, 0, lastRun.Location())
		ahead := (int(s.weekday) - int(day.Weekday()) + 7) % 7
		if ahead == 0 {
			ahead = 7
		}
		return day.AddDate(0, 0, ahead)
	case intervalKind:
		return lastRun.Add(s.interval)
	}
	// Unknown kind. The zero time means "no deadline" to the queue, which is
	// the safe direction — a job keeps its own retry budget rather than being
	// silently cancelled before its first attempt.
	return time.Time{}
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
