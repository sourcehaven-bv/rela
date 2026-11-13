package scalerun

import (
	"strings"
	"testing"
)

const validConfig = `
iterations: 5
warmup: 1
principals: {manager: alice@x, reader: carol@x}
fixtures_as: manager
ratio: {base_scale: 1, max: 2, floor_ms: 10}
bench: {max_regression: 0.2}
endpoints:
  - {name: list, method: GET, path: /api/v1/tasks, as: [manager, reader], expect: 200, p95_ms: {"1": 50}}
  - {name: create, method: POST, path: /api/v1/tasks, body: '{"id":"{new}"}', as: [manager], expect: 201}
  - {name: delete, method: DELETE, path: "/api/v1/tasks/{created}", as: [manager], expect: 204}
`

func TestParseConfig_Valid(t *testing.T) {
	c, err := ParseConfig([]byte(validConfig))
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Endpoints) != 3 || c.Endpoints[0].P95MS["1"] != 50 {
		t.Fatalf("unexpected config: %+v", c)
	}
}

// The checked-in config must load: the workflow reads it.
func TestLoadConfig_CheckedIn(t *testing.T) {
	c, err := LoadConfig("scalerun.yaml")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range c.Endpoints {
		for _, s := range []string{"1", "5"} {
			if _, ok := e.P95MS[s]; !ok {
				t.Errorf("%s has no p95 limit for scale %s", e.Name, s)
			}
		}
	}
}

func TestParseConfig_Invalid(t *testing.T) {
	tests := []struct {
		name, from, to, want string
	}{
		{"few iterations", "iterations: 5", "iterations: 1", "iterations"},
		{"negative warmup", "warmup: 1", "warmup: -1", "warmup"},
		{"unknown fixtures principal", "fixtures_as: manager", "fixtures_as: nobody", "fixtures_as"},
		{"ratio max", "max: 2", "max: 1", "ratio"},
		{"bench", "max_regression: 0.2", "max_regression: 0", "bench"},
		{"bad name", "name: list", "name: List", "name must"},
		{"duplicate name", "name: create", "name: list", "name must"},
		{"method", "method: GET", "method: PUT", "method"},
		{"path", "path: /api/v1/tasks,", "path: api/v1/tasks,", "path must"},
		{"expect", "expect: 200", "expect: 2000", "expect"},
		{"no principal", "as: [manager, reader]", "as: []", "at least one"},
		{"unknown principal", "as: [manager, reader]", "as: [manager, ghost]", "ghost"},
		{"limit", `{"1": 50}`, `{"one": 50}`, "p95_ms"},
		{"created without new", `body: '{"id":"{new}"}'`, `body: '{}'`, "{created}"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			raw := strings.Replace(validConfig, tc.from, tc.to, 1)
			if raw == validConfig {
				t.Fatalf("replacement %q not found", tc.from)
			}
			_, err := ParseConfig([]byte(raw))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("want error containing %q, got %v", tc.want, err)
			}
		})
	}
}

func TestParseConfig_RatioOverride(t *testing.T) {
	raw := strings.Replace(validConfig, `p95_ms: {"1": 50}`, `p95_ms: {"1": 50}, ratio_max: 1`, 1)
	if _, err := ParseConfig([]byte(raw)); err == nil {
		t.Fatal("ratio_max 1 accepted")
	}
	if _, err := ParseConfig([]byte("iterations: [")); err == nil {
		t.Fatal("bad YAML accepted")
	}
}
