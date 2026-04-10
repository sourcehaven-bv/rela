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
    schedule: "0 9 * * *"
    timeout: 5m
  - name: hourly-check
    script: checks/orphans.lua
    schedule: "7 * * * *"
`)
	cfg, err := ParseConfig(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Tasks) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(cfg.Tasks))
	}

	task := cfg.Tasks[0]
	if task.Name != "daily-report" {
		t.Errorf("name = %q, want %q", task.Name, "daily-report")
	}
	if task.Script != "reports/daily.lua" {
		t.Errorf("script = %q, want %q", task.Script, "reports/daily.lua")
	}
	if task.Schedule != "0 9 * * *" {
		t.Errorf("schedule = %q, want %q", task.Schedule, "0 9 * * *")
	}
	if task.Timeout != 5*time.Minute {
		t.Errorf("timeout = %v, want %v", task.Timeout, 5*time.Minute)
	}
}

func TestParseConfig_empty(t *testing.T) {
	t.Parallel()

	data := []byte("tasks: []\n")
	cfg, err := ParseConfig(data)
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
    schedule: "* * * * *"
  - name: foo
    script: b.lua
    schedule: "* * * * *"
`)
	_, err := ParseConfig(data)
	if err == nil {
		t.Fatal("expected error for duplicate name")
	}
	if got := err.Error(); got != `task "foo": duplicate task name` {
		t.Errorf("error = %q, want duplicate name error", got)
	}
}

func TestParseConfig_missingName(t *testing.T) {
	t.Parallel()

	data := []byte(`
tasks:
  - script: a.lua
    schedule: "* * * * *"
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
    schedule: "* * * * *"
`)
	_, err := ParseConfig(data)
	if err == nil {
		t.Fatal("expected error for missing script")
	}
}

func TestParseConfig_missingSchedule(t *testing.T) {
	t.Parallel()

	data := []byte(`
tasks:
  - name: foo
    script: a.lua
`)
	_, err := ParseConfig(data)
	if err == nil {
		t.Fatal("expected error for missing schedule")
	}
}

func TestParseConfig_invalidCron(t *testing.T) {
	t.Parallel()

	data := []byte(`
tasks:
  - name: foo
    script: a.lua
    schedule: "not a cron"
`)
	_, err := ParseConfig(data)
	if err == nil {
		t.Fatal("expected error for invalid cron")
	}
}

func TestParseConfig_negativeTimeout(t *testing.T) {
	t.Parallel()

	data := []byte(`
tasks:
  - name: foo
    script: a.lua
    schedule: "* * * * *"
    timeout: -5m
`)
	_, err := ParseConfig(data)
	if err == nil {
		t.Fatal("expected error for negative timeout")
	}
}

func TestParseConfig_invalidYAML(t *testing.T) {
	t.Parallel()

	data := []byte("{{{{not yaml")
	_, err := ParseConfig(data)
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}
