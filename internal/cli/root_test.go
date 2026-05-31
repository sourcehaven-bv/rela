package cli

import (
	stderrors "errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/errors"
)

// root_test.go covers root-level kong wiring.
//
// Tests dropped during the kong migration:
//   - TestNoShorthandConflicts: cobra-specific flag/shorthand
//     introspection. Kong reports a parser error at parse time if two
//     short flags collide, and there is no equivalent runtime walk of
//     the command tree to assert against.

func TestWrapDiscoverError(t *testing.T) {
	t.Run("no project returns init hint", func(t *testing.T) {
		wrapped := fmt.Errorf("discover: %w", errors.ErrNoProject)
		got := wrapDiscoverError(wrapped)
		if got == nil {
			t.Fatal("expected error, got nil")
		}
		msg := got.Error()
		if !strings.Contains(msg, "run 'rela init'") {
			t.Errorf("expected init hint, got: %q", msg)
		}
	})

	t.Run("other errors are surfaced verbatim", func(t *testing.T) {
		underlying := stderrors.New("load metamodel: yaml: line 3: mapping values are not allowed in this context")
		got := wrapDiscoverError(underlying)
		if got == nil {
			t.Fatal("expected error, got nil")
		}
		msg := got.Error()
		if strings.Contains(msg, "run 'rela init'") {
			t.Errorf("should not suggest init for load failures, got: %q", msg)
		}
		if !strings.Contains(msg, "yaml: line 3") {
			t.Errorf("expected underlying error to be surfaced, got: %q", msg)
		}
		if !stderrors.Is(got, underlying) && got.Error() != underlying.Error() {
			t.Errorf("expected underlying error preserved, got: %v", got)
		}
	})
}

// TestRootCmdProjectFlag verifies the kong CLI struct exposes a
// Project field with no short alias (removed to avoid conflict with
// --priority on create/update).
func TestRootCmdProjectFlag(t *testing.T) {
	rt := reflect.TypeOf(CLI{})
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
