package frontendroutes

import (
	"sort"
	"strings"
	"testing"
)

func TestAll_stableSortedCopy(t *testing.T) {
	t.Parallel()
	got := All()
	if len(got) == 0 {
		t.Fatal("All() returned no routes")
	}
	names := make([]string, len(got))
	for i, r := range got {
		names[i] = r.Name
	}
	if !sort.StringsAreSorted(names) {
		t.Errorf("All() not sorted by name: %v", names)
	}
	// Mutating the result must not affect internal state.
	got[0].Name = "MUTATED"
	again := All()
	if again[0].Name == "MUTATED" {
		t.Error("All() returned shared slice; mutation leaked into catalog")
	}
}

func TestHas(t *testing.T) {
	t.Parallel()
	cases := []struct {
		path string
		want bool
	}{
		{"/dashboard", true},
		{"/form/full_ticket", true},
		{"/form/full_ticket/TKT-001", true},
		{"/entity/ticket/TKT-001", true},
		{"/list/all_tasks", true},
		{"/search", true},
		{"/", false},           // redirect, not a real route
		{"/nope", false},       // unknown top-level
		{"/form", false},       // missing required segment
		{"/form/a/b/c", false}, // too many segments
		{"", false},
		{"not-absolute", false},
	}
	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			t.Parallel()
			if got := Has(tc.path); got != tc.want {
				t.Errorf("Has(%q) = %v, want %v", tc.path, got, tc.want)
			}
		})
	}
}

func TestMatch_routeByPattern(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		path     string
		wantName string
	}{
		{name: "form-edit", path: "/form/full_ticket/TKT-001", wantName: "form-edit"},
		{name: "form-create", path: "/form/quick_task", wantName: "form-create"},
		{name: "entity", path: "/entity/ticket/TKT-001", wantName: "entity"},
		{name: "dashboard-no-params", path: "/dashboard", wantName: "dashboard"},
		// Encoded path segments are compared byte-for-byte (documented
		// on Match) — percent-encoding does not prevent a match.
		{name: "encoded-slash-in-segment", path: "/form/edit_ticket/TKT%2F001", wantName: "form-edit"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			m, ok := Match(tc.path)
			if !ok {
				t.Fatalf("Match(%q) returned !ok", tc.path)
			}
			if m.Route.Name != tc.wantName {
				t.Errorf("route name = %q, want %q", m.Route.Name, tc.wantName)
			}
		})
	}
}

func TestMatch_noMatch(t *testing.T) {
	t.Parallel()
	cases := []string{
		"/nope",
		"/form",
		"/form/a/b/c",
		"",
		"not-absolute",
	}
	for _, path := range cases {
		t.Run(path, func(t *testing.T) {
			t.Parallel()
			if m, ok := Match(path); ok {
				t.Errorf("Match(%q) unexpectedly matched %q", path, m.Route.Name)
			}
		})
	}
}

func TestRoutes_noPatternOverlap(t *testing.T) {
	t.Parallel()
	// A literal "probe" path constructed from each route's pattern must match
	// that route and no other. Otherwise Match's first-wins behavior hides
	// ambiguity.
	for i, r := range routes {
		probe := probePath(r)
		m, ok := Match(probe)
		if !ok {
			t.Errorf("route %q (path %q): probe %q failed to match", r.Name, r.Path, probe)
			continue
		}
		if m.Route.Name != r.Name {
			t.Errorf("route %q (path %q): probe %q matched %q instead", r.Name, r.Path, probe, m.Route.Name)
		}
		// Also ensure no later route also matches the same probe.
		for j := i + 1; j < len(routes); j++ {
			if patternMatches(routes[j].Path, probe) {
				t.Errorf("routes %q and %q both match probe %q — patterns overlap", r.Name, routes[j].Name, probe)
			}
		}
	}
}

func probePath(r Route) string {
	segs := splitPath(r.Path)
	out := make([]string, len(segs))
	for i, s := range segs {
		if s != "" && s[0] == ':' {
			out[i] = "X" + s[1:] // unique synthetic value
		} else {
			out[i] = s
		}
	}
	return "/" + joinSegs(out)
}

func joinSegs(segs []string) string {
	return strings.Join(segs, "/")
}
