package dataentryconfig

import (
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

func scopeMeta(t *testing.T) *metamodel.Metamodel {
	t.Helper()
	m, err := metamodel.Parse([]byte(`version: "1.0"
namespace: https://example.org/test#
entities:
  taak:
    label: Taak
    id_prefix: TAAK
    properties: {status: {type: string}}
    query_scopes:
      default: "entity.status ~= 'gearchiveerd'"
      archief: "entity.status == 'gearchiveerd'"
  notitie:
    label: Notitie
    id_prefix: NOTE
    properties: {status: {type: string}}
`))
	if err != nil {
		t.Fatalf("parse schema: %v", err)
	}
	return m
}

func TestValidateQueryScopes(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *Config
		wantErr string
	}{
		{
			name: "a declared scope resolves",
			cfg: &Config{Lists: map[string]List{
				"a": {EntityType: "taak", QueryScope: "archief"},
			}},
		},
		{
			name: "an absent scope resolves (the type's default applies)",
			cfg: &Config{Lists: map[string]List{
				"a": {EntityType: "taak"},
			}},
		},
		{
			name: "the implicit all resolves even though it is not declared",
			cfg: &Config{Lists: map[string]List{
				"a": {EntityType: "taak", QueryScope: "all"},
			}},
		},
		{
			name: "all resolves for a type declaring no scopes at all",
			cfg: &Config{Lists: map[string]List{
				"a": {EntityType: "notitie", QueryScope: "all"},
			}},
		},
		{
			name: "an undeclared name is refused",
			cfg: &Config{Lists: map[string]List{
				"a": {EntityType: "taak", QueryScope: "archieff"},
			}},
			wantErr: `list "a": query_scope "archieff" is not declared on entity type "taak"`,
		},
		{
			name: "a scope declared on ANOTHER type is refused",
			cfg: &Config{Lists: map[string]List{
				"a": {EntityType: "notitie", QueryScope: "archief"},
			}},
			wantErr: `entity type "notitie" declares no query_scopes:`,
		},
		{
			name: "a kanban is checked too",
			cfg: &Config{Kanbans: map[string]Kanban{
				"b": {EntityType: "taak", QueryScope: "nope"},
			}},
			wantErr: `kanban "b": query_scope "nope" is not declared`,
		},
	}

	meta := scopeMeta(t)
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			errs := validateQueryScopes(tc.cfg, meta)
			joined := strings.Join(errs, "\n")
			if tc.wantErr == "" {
				if len(errs) != 0 {
					t.Fatalf("want no errors, got:\n%s", joined)
				}
				return
			}
			if !strings.Contains(joined, tc.wantErr) {
				t.Errorf("errors =\n%s\nwant one containing %q", joined, tc.wantErr)
			}
		})
	}
}

// TestValidateQueryScopes_NamesTheAlternatives pins that the diagnostic lists
// what IS declared. An operator who mistypes a scope name needs to see the
// real ones; "not declared" alone sends them back to the schema to find out.
func TestValidateQueryScopes_NamesTheAlternatives(t *testing.T) {
	errs := validateQueryScopes(&Config{Lists: map[string]List{
		"a": {EntityType: "taak", QueryScope: "typo"},
	}}, scopeMeta(t))
	joined := strings.Join(errs, "\n")
	for _, want := range []string{"archief", "default", `implicit "all"`} {
		if !strings.Contains(joined, want) {
			t.Errorf("diagnostic does not mention %q; got:\n%s", want, joined)
		}
	}
}

// TestValidateQueryScopes_UnknownTypeIsSilent pins that a scope on an unknown
// entity type adds no error here: validateLists already reports the unknown
// type, and a second error about a scope on a type that does not exist is
// noise that buries the real one.
func TestValidateQueryScopes_UnknownTypeIsSilent(t *testing.T) {
	errs := validateQueryScopes(&Config{Lists: map[string]List{
		"a": {EntityType: "geenidee", QueryScope: "archief"},
	}}, scopeMeta(t))
	if len(errs) != 0 {
		t.Errorf("want no errors for an unknown entity type, got:\n%s", strings.Join(errs, "\n"))
	}
}
