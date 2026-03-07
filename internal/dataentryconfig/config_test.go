package dataentryconfig

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestUserDefaultsResolvePropertyDefault(t *testing.T) {
	ud := &UserDefaults{
		Defaults: map[string]string{
			"priority": "medium",
			"reporter": "jeroen",
		},
		Overrides: []DefaultOverride{
			{
				Types: []string{"ticket", "bug"},
				Defaults: map[string]string{
					"priority": "high",
					"status":   "triaged",
				},
			},
			{
				Types: []string{"decision"},
				Defaults: map[string]string{
					"status": "proposed",
				},
			},
		},
	}

	t.Run("global default", func(t *testing.T) {
		got := ud.ResolvePropertyDefault("component", "priority")
		if got != "medium" {
			t.Errorf("expected 'medium', got %q", got)
		}
	})

	t.Run("global default for reporter", func(t *testing.T) {
		got := ud.ResolvePropertyDefault("ticket", "reporter")
		if got != "jeroen" {
			t.Errorf("expected 'jeroen', got %q", got)
		}
	})

	t.Run("override takes precedence over global", func(t *testing.T) {
		got := ud.ResolvePropertyDefault("ticket", "priority")
		if got != "high" {
			t.Errorf("expected 'high', got %q", got)
		}
	})

	t.Run("override for second type in list", func(t *testing.T) {
		got := ud.ResolvePropertyDefault("bug", "priority")
		if got != "high" {
			t.Errorf("expected 'high', got %q", got)
		}
	})

	t.Run("override-only property", func(t *testing.T) {
		got := ud.ResolvePropertyDefault("ticket", "status")
		if got != "triaged" {
			t.Errorf("expected 'triaged', got %q", got)
		}
	})

	t.Run("different override group", func(t *testing.T) {
		got := ud.ResolvePropertyDefault("decision", "status")
		if got != "proposed" {
			t.Errorf("expected 'proposed', got %q", got)
		}
	})

	t.Run("unknown property returns empty", func(t *testing.T) {
		got := ud.ResolvePropertyDefault("ticket", "nonexistent")
		if got != "" {
			t.Errorf("expected empty, got %q", got)
		}
	})

	t.Run("nil UserDefaults returns empty", func(t *testing.T) {
		var nilUD *UserDefaults
		got := nilUD.ResolvePropertyDefault("ticket", "priority")
		if got != "" {
			t.Errorf("expected empty, got %q", got)
		}
	})
}

func TestUserDefaultsResolveRelationDefault(t *testing.T) {
	ud := &UserDefaults{
		RelationDefaults: map[string]string{
			"reported-by": "jeroen",
		},
		Overrides: []DefaultOverride{
			{
				Types: []string{"ticket"},
				RelationDefaults: map[string]string{
					"assigned-to": "jeroen",
				},
			},
		},
	}

	t.Run("global relation default", func(t *testing.T) {
		got := ud.ResolveRelationDefault("decision", "reported-by")
		if got != "jeroen" {
			t.Errorf("expected 'jeroen', got %q", got)
		}
	})

	t.Run("override relation default", func(t *testing.T) {
		got := ud.ResolveRelationDefault("ticket", "assigned-to")
		if got != "jeroen" {
			t.Errorf("expected 'jeroen', got %q", got)
		}
	})

	t.Run("no override for other entity types", func(t *testing.T) {
		got := ud.ResolveRelationDefault("decision", "assigned-to")
		if got != "" {
			t.Errorf("expected empty, got %q", got)
		}
	})

	t.Run("unknown relation returns empty", func(t *testing.T) {
		got := ud.ResolveRelationDefault("ticket", "nonexistent")
		if got != "" {
			t.Errorf("expected empty, got %q", got)
		}
	})

	t.Run("nil UserDefaults returns empty", func(t *testing.T) {
		var nilUD *UserDefaults
		got := nilUD.ResolveRelationDefault("ticket", "reported-by")
		if got != "" {
			t.Errorf("expected empty, got %q", got)
		}
	})
}

func TestFilterControlKey(t *testing.T) {
	t.Run("returns relation when set", func(t *testing.T) {
		fc := FilterControl{Relation: "belongs_to", Property: "status"}
		if got := fc.Key(); got != "belongs_to" {
			t.Errorf("expected 'belongs_to', got %q", got)
		}
	})

	t.Run("returns property when relation empty", func(t *testing.T) {
		fc := FilterControl{Property: "status"}
		if got := fc.Key(); got != "status" {
			t.Errorf("expected 'status', got %q", got)
		}
	})

	t.Run("returns empty when both empty", func(t *testing.T) {
		fc := FilterControl{}
		if got := fc.Key(); got != "" {
			t.Errorf("expected empty, got %q", got)
		}
	})
}

func TestFilterControlIsRelation(t *testing.T) {
	t.Run("true when relation set", func(t *testing.T) {
		fc := FilterControl{Relation: "belongs_to"}
		if !fc.IsRelation() {
			t.Error("expected true")
		}
	})

	t.Run("false when relation empty", func(t *testing.T) {
		fc := FilterControl{Property: "status"}
		if fc.IsRelation() {
			t.Error("expected false")
		}
	})
}

func TestFilterControlQueryParamKey(t *testing.T) {
	t.Run("property filter", func(t *testing.T) {
		fc := FilterControl{Property: "status"}
		if got := fc.QueryParamKey(); got != "filter_status" {
			t.Errorf("expected 'filter_status', got %q", got)
		}
	})

	t.Run("relation filter", func(t *testing.T) {
		fc := FilterControl{Relation: "belongs_to"}
		if got := fc.QueryParamKey(); got != "filter_belongs_to" {
			t.Errorf("expected 'filter_belongs_to', got %q", got)
		}
	})
}

func TestFilterControlCurrentValue(t *testing.T) {
	t.Run("returns value when present", func(t *testing.T) {
		fc := FilterControl{Property: "status"}
		query := map[string][]string{"filter_status": {"open"}}
		if got := fc.CurrentValue(query); got != "open" {
			t.Errorf("expected 'open', got %q", got)
		}
	})

	t.Run("returns empty when not present", func(t *testing.T) {
		fc := FilterControl{Property: "status"}
		query := map[string][]string{}
		if got := fc.CurrentValue(query); got != "" {
			t.Errorf("expected empty, got %q", got)
		}
	})

	t.Run("relation filter", func(t *testing.T) {
		fc := FilterControl{Relation: "belongs_to"}
		query := map[string][]string{"filter_belongs_to": {"category-1"}}
		if got := fc.CurrentValue(query); got != "category-1" {
			t.Errorf("expected 'category-1', got %q", got)
		}
	})
}

func TestDirection_UnmarshalYAML(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		want    Direction
		wantErr string
	}{
		{
			name: "empty defaults to outgoing",
			yaml: `direction: ""`,
			want: DirectionOutgoing,
		},
		{
			name: "outgoing",
			yaml: `direction: outgoing`,
			want: DirectionOutgoing,
		},
		{
			name: "incoming",
			yaml: `direction: incoming`,
			want: DirectionIncoming,
		},
		{
			name:    "invalid direction",
			yaml:    `direction: both`,
			wantErr: `invalid direction "both"`,
		},
		{
			name:    "invalid direction sideways",
			yaml:    `direction: sideways`,
			wantErr: `invalid direction "sideways"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var cfg struct {
				Direction Direction `yaml:"direction"`
			}
			err := yaml.Unmarshal([]byte(tt.yaml), &cfg)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("expected error containing %q, got %v", tt.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if cfg.Direction != tt.want {
				t.Errorf("got %q, want %q", cfg.Direction, tt.want)
			}
		})
	}
}

func TestDirection_IsIncoming(t *testing.T) {
	t.Run("incoming returns true", func(t *testing.T) {
		if !DirectionIncoming.IsIncoming() {
			t.Error("expected true")
		}
	})

	t.Run("outgoing returns false", func(t *testing.T) {
		if DirectionOutgoing.IsIncoming() {
			t.Error("expected false")
		}
	})

	t.Run("empty returns false", func(t *testing.T) {
		var d Direction
		if d.IsIncoming() {
			t.Error("expected false")
		}
	})
}
