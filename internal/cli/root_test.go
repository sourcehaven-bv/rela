package cli

import (
	"reflect"
	"testing"
)

// root_test.go covers root-level kong wiring.
//
// Tests dropped during the kong migration:
//   - TestNoShorthandConflicts: cobra-specific flag/shorthand
//     introspection. Kong reports a parser error at parse time if two
//     short flags collide, and there is no equivalent runtime walk of
//     the command tree to assert against.
//
// TestWrapDiscoverError moved to internal/errors alongside the shared
// WrapDiscoverError function (used by both rela and rela-docs).

// TestRootCmdProjectFlag verifies the kong CLI struct exposes a
// Project field with no short alias (removed to avoid conflict with
// --priority on create/update).
func TestRootCmdProjectFlag(t *testing.T) {
	rt := reflect.TypeFor[CLI]()
	f, ok := rt.FieldByName("Project")
	if !ok {
		t.Fatal("expected Project field on CLI struct")
	}
	if got := f.Tag.Get("short"); got != "" {
		t.Errorf("expected no short tag, got %q", got)
	}
	if got := f.Tag.Get("env"); got != "RELA_PROJECT" {
		t.Errorf("expected env=RELA_PROJECT, got %q", got)
	}
}
