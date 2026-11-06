package metamodel

import (
	"strings"
	"testing"
)

func TestParse_ReservedPrefix(t *testing.T) {
	const base = `
version: "1.0"
entities:
  %ENTITY%:
    label: Task
    id_prefix: "TASK-"
    %ALIASES%
    properties:
      %PROP%:
        type: string
relations:
  %REL%:
    label: blocks
    from: [%ENTITY%]
    to: [%ENTITY%]
    %INVERSE%
    properties:
      %RELPROP%:
        type: string
`
	render := func(repl map[string]string) string {
		s := base
		defaults := map[string]string{
			"%ENTITY%": "task", "%ALIASES%": "", "%PROP%": "title", "%REL%": "blocks",
			"%INVERSE%": "", "%RELPROP%": "note",
		}
		for k, v := range defaults {
			if r, ok := repl[k]; ok {
				v = r
			}
			s = strings.ReplaceAll(s, k, v)
		}
		return s
	}
	tests := []struct {
		name string
		repl map[string]string
		want string
	}{
		{"entity type", map[string]string{"%ENTITY%": "_task"}, `entity type "_task"`},
		{"alias", map[string]string{"%ALIASES%": "aliases: [_t]"}, `alias "_t"`},
		{"entity property", map[string]string{"%PROP%": "_title"}, `property "_title"`},
		{"relation type", map[string]string{"%REL%": "_blocks"}, `relation type "_blocks"`},
		{"inverse", map[string]string{"%INVERSE%": "inverse: { id: _blocked-by }"}, `inverse "_blocked-by"`},
		{"relation property", map[string]string{"%RELPROP%": "_note"}, `property "_note"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse([]byte(render(tt.repl)))
			if err == nil {
				t.Fatal("expected a load error for a reserved name")
			}
			if !strings.Contains(err.Error(), tt.want) || !strings.Contains(err.Error(), "reserved for rela") {
				t.Errorf("error %q does not name %s as reserved", err.Error(), tt.want)
			}
		})
	}
	t.Run("plain names load", func(t *testing.T) {
		if _, err := Parse([]byte(render(nil))); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	t.Run("underscore inside a name is allowed", func(t *testing.T) {
		repl := map[string]string{"%PROP%": "due_date", "%REL%": "is_blocking"}
		if _, err := Parse([]byte(render(repl))); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestIsReservedName(t *testing.T) {
	for name, want := range map[string]bool{
		"_flow-run": true, "isms._run": true, "task": false, "isms.task": false, "a_b": false,
	} {
		if got := IsReservedName(name); got != want {
			t.Errorf("IsReservedName(%q) = %v; want %v", name, got, want)
		}
	}
}
