package scheduler

import (
	"testing"
	"time"
)

func TestParseConfig_valid(t *testing.T) {
	t.Parallel()

	data := []byte(`
tasks:
  - name: daily-report
    script: reports/daily.lua
    every: day
  - name: hourly-check
    script: checks/orphans.lua
    every: 30m
`)
	cfg, err := ParseConfig(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Tasks) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(cfg.Tasks))
	}

	if cfg.Tasks[0].Name != "daily-report" {
		t.Errorf("name = %q, want %q", cfg.Tasks[0].Name, "daily-report")
	}
	if cfg.Tasks[0].Every.String() != "day" {
		t.Errorf("every = %q, want %q", cfg.Tasks[0].Every, "day")
	}
	if cfg.Tasks[1].Every.String() != "30m0s" {
		t.Errorf("every = %q, want %q", cfg.Tasks[1].Every, "30m0s")
	}
}

func TestParseConfig_empty(t *testing.T) {
	t.Parallel()

	cfg, err := ParseConfig([]byte("tasks: []\n"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Tasks) != 0 {
		t.Fatalf("expected 0 tasks, got %d", len(cfg.Tasks))
	}
}

func TestParseConfig_duplicateName(t *testing.T) {
	t.Parallel()

	data := []byte(`
tasks:
  - name: foo
    script: a.lua
    every: day
  - name: foo
    script: b.lua
    every: day
`)
	_, err := ParseConfig(data)
	if err == nil {
		t.Fatal("expected error for duplicate name")
	}
}

func TestParseConfig_missingName(t *testing.T) {
	t.Parallel()

	data := []byte(`
tasks:
  - script: a.lua
    every: day
`)
	_, err := ParseConfig(data)
	if err == nil {
		t.Fatal("expected error for missing name")
	}
}

func TestParseConfig_missingScript(t *testing.T) {
	t.Parallel()

	data := []byte(`
tasks:
  - name: foo
    every: day
`)
	_, err := ParseConfig(data)
	if err == nil {
		t.Fatal("expected error for missing script")
	}
}

func TestParseConfig_missingEvery(t *testing.T) {
	t.Parallel()

	data := []byte(`
tasks:
  - name: foo
    script: a.lua
`)
	_, err := ParseConfig(data)
	if err == nil {
		t.Fatal("expected error for missing every")
	}
}

func TestParseConfig_invalidSchedule(t *testing.T) {
	t.Parallel()

	data := []byte(`
tasks:
  - name: foo
    script: a.lua
    every: "not-valid"
`)
	_, err := ParseConfig(data)
	if err == nil {
		t.Fatal("expected error for invalid schedule")
	}
}

func TestParseConfig_invalidYAML(t *testing.T) {
	t.Parallel()

	_, err := ParseConfig([]byte("{{{{not yaml"))
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestParseSchedule_day(t *testing.T) {
	t.Parallel()
	s, err := parseSchedule("day")
	if err != nil {
		t.Fatal(err)
	}
	if s.kind != dayKind {
		t.Errorf("kind = %v, want dayKind", s.kind)
	}
}

func TestParseSchedule_week(t *testing.T) {
	t.Parallel()
	s, err := parseSchedule("week")
	if err != nil {
		t.Fatal(err)
	}
	if s.kind != weekKind {
		t.Errorf("kind = %v, want weekKind", s.kind)
	}
}

func TestParseSchedule_duration(t *testing.T) {
	t.Parallel()
	s, err := parseSchedule("2h30m")
	if err != nil {
		t.Fatal(err)
	}
	if s.kind != intervalKind || s.interval != 2*time.Hour+30*time.Minute {
		t.Errorf("got %v, want 2h30m interval", s)
	}
}

func TestParseSchedule_bareMinutes(t *testing.T) {
	t.Parallel()
	s, err := parseSchedule("15")
	if err != nil {
		t.Fatal(err)
	}
	if s.interval != 15*time.Minute {
		t.Errorf("got %v, want 15m", s.interval)
	}
}

func TestParseSchedule_negativeInterval(t *testing.T) {
	t.Parallel()
	_, err := parseSchedule("-5m")
	if err == nil {
		t.Fatal("expected error for negative interval")
	}
}

func TestParseSchedule_zeroInterval(t *testing.T) {
	t.Parallel()
	_, err := parseSchedule("0")
	if err == nil {
		t.Fatal("expected error for zero interval")
	}
}

func TestScheduleIsDue_day(t *testing.T) {
	t.Parallel()

	s := Schedule{kind: dayKind, set: true}
	yesterday := time.Date(2026, 4, 9, 23, 0, 0, 0, time.Local)
	today := time.Date(2026, 4, 10, 0, 5, 0, 0, time.Local)

	if !s.IsDue(yesterday, today) {
		t.Error("expected due: different day")
	}
	if s.IsDue(today, today.Add(30*time.Minute)) {
		t.Error("expected not due: same day")
	}
}

func TestScheduleIsDue_week(t *testing.T) {
	t.Parallel()

	s := Schedule{kind: weekKind, set: true}
	// Sunday April 5 2026 is in week 14, Monday April 6 is week 15.
	lastWeek := time.Date(2026, 4, 5, 12, 0, 0, 0, time.Local)
	thisWeek := time.Date(2026, 4, 6, 0, 5, 0, 0, time.Local)

	if !s.IsDue(lastWeek, thisWeek) {
		t.Error("expected due: different week")
	}
	if s.IsDue(thisWeek, thisWeek.Add(24*time.Hour)) {
		t.Error("expected not due: same week")
	}
}

func TestScheduleIsDue_interval(t *testing.T) {
	t.Parallel()

	s := Schedule{kind: intervalKind, interval: 30 * time.Minute, set: true}
	lastRun := time.Date(2026, 4, 10, 9, 0, 0, 0, time.UTC)

	if s.IsDue(lastRun, lastRun.Add(29*time.Minute)) {
		t.Error("expected not due: only 29m elapsed")
	}
	if !s.IsDue(lastRun, lastRun.Add(30*time.Minute)) {
		t.Error("expected due: 30m elapsed")
	}
}
