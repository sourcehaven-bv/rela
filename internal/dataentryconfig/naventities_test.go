package dataentryconfig

import (
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

func navEntitiesMeta(t *testing.T) *metamodel.Metamodel {
	t.Helper()
	m, err := metamodel.Parse([]byte(`version: "1.0"
namespace: https://example.org/test#
entities:
  project:
    label: Project
    id_prefix: PRJ
    properties: {title: {type: string}, status: {type: string}}
    default_sort:
      - property: title
    query_scopes:
      active: "entity.status == 'active'"
  note:
    label: Note
    id_prefix: NOTE
    properties: {title: {type: string}}
`))
	if err != nil {
		t.Fatalf("parse schema: %v", err)
	}
	return m
}

func inGroup(items ...NavigationEntry) []NavigationEntry {
	return []NavigationEntry{{Group: "G", Items: items}}
}

// TestValidateNavEntities covers every load-time rule for an `entities:`
// navigation entry, through ValidateConfig so the wiring is pinned too.
func TestValidateNavEntities(t *testing.T) {
	tests := []struct {
		name    string
		nav     []NavigationEntry
		wantErr string // empty = valid
	}{
		{name: "type only", nav: inGroup(NavigationEntry{Entities: "project"})},
		{name: "declared scope and sort", nav: inGroup(NavigationEntry{
			Entities: "project", QueryScope: "active",
			Sort: []SortSpec{{Property: "title", Direction: "desc"}, {Property: "id"}},
		})},
		{name: "implicit all scope", nav: inGroup(NavigationEntry{Entities: "note", QueryScope: "all"})},
		{name: "icon and permission allowed", nav: inGroup(NavigationEntry{
			Entities: "project", Icon: "folder", Permission: "p:nav",
		})},
		{
			name:    "unknown type",
			nav:     inGroup(NavigationEntry{Entities: "projct"}),
			wantErr: `navigation entities "projct": unknown entity type "projct"`,
		},
		{
			name:    "undeclared scope",
			nav:     inGroup(NavigationEntry{Entities: "project", QueryScope: "actve"}),
			wantErr: `navigation entities "project": query_scope "actve" is not declared on entity type "project"`,
		},
		{
			name:    "unknown sort property",
			nav:     inGroup(NavigationEntry{Entities: "project", Sort: []SortSpec{{Property: "nope"}}}),
			wantErr: `navigation entities "project": sort[0] references unknown property "nope"`,
		},
		{
			name: "bad sort direction",
			nav: inGroup(NavigationEntry{
				Entities: "project", Sort: []SortSpec{{Property: "title", Direction: "up"}},
			}),
			wantErr: `navigation entities "project": sort[0] has invalid direction "up"`,
		},
		{
			name:    "combined with another kind",
			nav:     inGroup(NavigationEntry{Entities: "project", List: "all"}),
			wantErr: `navigation entities "project": cannot be combined with list`,
		},
		{
			name:    "on a group",
			nav:     []NavigationEntry{{Group: "G", Entities: "project"}},
			wantErr: `navigation entities "project": cannot be combined with group`,
		},
		{
			name:    "at the top level",
			nav:     []NavigationEntry{{Entities: "project"}},
			wantErr: `navigation entities "project": must be inside a group`,
		},
		{
			name:    "with a label",
			nav:     inGroup(NavigationEntry{Entities: "project", Label: "Projects"}),
			wantErr: `navigation entities "project": label is not allowed`,
		},
		{
			name:    "query_scope without entities",
			nav:     inGroup(NavigationEntry{Label: "All", List: "all", QueryScope: "active"}),
			wantErr: `navigation "All": query_scope is only valid on an entities: entry`,
		},
		{
			name:    "sort without entities",
			nav:     inGroup(NavigationEntry{Label: "All", List: "all", Sort: []SortSpec{{Property: "title"}}}),
			wantErr: `navigation "All": sort is only valid on an entities: entry`,
		},
		{
			name:    "unknown icon names the type",
			nav:     inGroup(NavigationEntry{Entities: "project", Icon: "nosuchicon"}),
			wantErr: `navigation "project": unknown icon "nosuchicon"`,
		},
	}

	meta := navEntitiesMeta(t)
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &Config{
				Lists:      map[string]List{"all": {EntityType: "project"}},
				Navigation: tc.nav,
			}
			err := ValidateConfig(nil, cfg, meta)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("want valid, got: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error = %v\nwant one containing %q", err, tc.wantErr)
			}
		})
	}
}

func TestEffectiveNavSort(t *testing.T) {
	meta := navEntitiesMeta(t)
	own := []SortSpec{{Property: "status", Direction: "desc"}}
	tests := []struct {
		name string
		nav  NavigationEntry
		meta *metamodel.Metamodel
		want string
	}{
		{"entry sort wins", NavigationEntry{Entities: "project", Sort: own}, meta, "-status"},
		{"type default_sort", NavigationEntry{Entities: "project"}, meta, "title"},
		{"no default: id order", NavigationEntry{Entities: "note"}, meta, ""},
		{"unknown type", NavigationEntry{Entities: "x"}, meta, ""},
		{"nil meta", NavigationEntry{Entities: "project"}, nil, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := SortParam(EffectiveNavSort(tc.nav, tc.meta)); got != tc.want {
				t.Errorf("sort = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestSortParam(t *testing.T) {
	got := SortParam([]SortSpec{
		{Property: "priority", Direction: "desc"}, {Property: ""}, {Property: "title", Direction: "asc"},
	})
	if got != "-priority,title" {
		t.Errorf("SortParam = %q, want %q", got, "-priority,title")
	}
}
