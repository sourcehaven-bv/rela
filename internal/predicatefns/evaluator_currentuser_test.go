package predicatefns_test

import (
	"context"
	"errors"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/predicate"
	"github.com/Sourcehaven-BV/rela/internal/predicatefns"
)

func evaluatorMeta() *metamodel.Metamodel {
	return &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"task": {Properties: map[string]metamodel.PropertyDef{
				"assignee": {Type: metamodel.PropertyTypeString},
				"status":   {Type: metamodel.PropertyTypeString},
				"watchers": {Type: metamodel.PropertyTypeString, List: true},
			}},
		},
	}
}

// TestRequiresCurrentUser pins the dependency test the request-scoped
// matcher relies on: every spelling that reads the identity is detected,
// and a condition that never mentions it is not.
func TestRequiresCurrentUser(t *testing.T) {
	ev := predicatefns.NewEvaluator(evaluatorMeta())
	tests := []struct {
		src  string
		want bool
	}{
		{"entity.assignee == current_user.id", true},
		{"current_user.tool == 'mcp'", true},
		{"is_current_user(entity.assignee)", true},
		{"has_current_user(entity.watchers)", true},
		{"entity.status == 'todo' and is_current_user(entity.assignee)", true},
		{"entity.status == 'todo'", false},
		{"contains(entity.watchers, 'x')", false},
	}
	for _, tc := range tests {
		t.Run(tc.src, func(t *testing.T) {
			prog, err := ev.CompileWithCurrentUser("task", tc.src)
			if err != nil {
				t.Fatalf("compile: %v", err)
			}
			if got := predicatefns.RequiresCurrentUser(prog); got != tc.want {
				t.Fatalf("RequiresCurrentUser = %v, want %v", got, tc.want)
			}
		})
	}
	if predicatefns.RequiresCurrentUser(nil) {
		t.Fatal("a nil program must require nothing")
	}
}

// TestMatchesAs_IdentityBoundOnlyWhenNeeded is the contract that lets one
// request-scoped profile serve every next-action condition: a condition
// without an identity clause evaluates on an unidentified ctx, one with
// an identity clause fails closed on it, and both work once an identity
// is stamped.
func TestMatchesAs_IdentityBoundOnlyWhenNeeded(t *testing.T) {
	ev := predicatefns.NewEvaluator(evaluatorMeta())
	props := map[string]any{"status": "todo", "assignee": "PERS-JV", "watchers": []any{"PERS-AB", "PERS-JV"}}
	noIdentity := context.Background()
	withIdentity := predicatefns.WithQueryIdentity(noIdentity, predicatefns.QueryIdentity{EntityID: "PERS-JV"})
	otherIdentity := predicatefns.WithQueryIdentity(noIdentity, predicatefns.QueryIdentity{EntityID: "PERS-ZZ"})

	tests := []struct {
		name    string
		src     string
		ctx     context.Context //nolint:containedctx // table fixture selects among three prebuilt contexts
		want    bool
		wantErr error
	}{
		{"identity-free condition evaluates without an identity",
			"entity.status == 'todo'", noIdentity, true, nil},
		{"identity-free condition evaluates with one too",
			"entity.status == 'todo'", withIdentity, true, nil},
		{"equality clause fails closed without an identity",
			"entity.assignee == current_user.id", noIdentity, false, predicatefns.ErrNoCurrentUser},
		{"is_current_user fails closed without an identity",
			"is_current_user(entity.assignee)", noIdentity, false, predicatefns.ErrNoCurrentUser},
		{"has_current_user fails closed without an identity",
			"has_current_user(entity.watchers)", noIdentity, false, predicatefns.ErrNoCurrentUser},
		{"equality clause matches the stamped identity",
			"entity.assignee == current_user.id", withIdentity, true, nil},
		{"is_current_user matches the stamped identity",
			"is_current_user(entity.assignee)", withIdentity, true, nil},
		{"has_current_user finds the stamped identity in the list",
			"has_current_user(entity.watchers)", withIdentity, true, nil},
		{"a different identity does not match",
			"is_current_user(entity.assignee) or has_current_user(entity.watchers)", otherIdentity, false, nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			prog, err := ev.CompileWithCurrentUser("task", tc.src)
			if err != nil {
				t.Fatalf("compile: %v", err)
			}
			got, err := ev.MatchesAs(tc.ctx, prog, "task", "T-1", props)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("got err %v, want %v", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("MatchesAs: %v", err)
			}
			if got != tc.want {
				t.Fatalf("MatchesAs = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestCurrentUserPrefilterSpec_MatchesTheDeclaredFuncs is the drift guard
// between the sugar functions and the pushdown's reading of them: every
// declared sugar function is listed, and it resolves to the identity
// field.
func TestCurrentUserPrefilterSpec_MatchesTheDeclaredFuncs(t *testing.T) {
	spec := predicatefns.CurrentUserPrefilterSpec()
	if spec.RecordVar != predicatefns.VarEntity || spec.ConstVar != predicatefns.VarCurrentUser {
		t.Fatalf("spec names %q/%q, want entity/current_user", spec.RecordVar, spec.ConstVar)
	}
	for name := range predicatefns.CurrentUserFuncs() {
		field, ok := spec.ConstFuncs[name]
		if !ok {
			t.Fatalf("sugar function %q is declared but not in the prefilter spec", name)
		}
		if field != predicatefns.FieldCurrentUserID {
			t.Fatalf("%q maps to %q, want %q", name, field, predicatefns.FieldCurrentUserID)
		}
	}
	if len(spec.ConstFuncs) != len(predicatefns.CurrentUserFuncs()) {
		t.Fatalf("spec lists %d functions, %d are declared", len(spec.ConstFuncs), len(predicatefns.CurrentUserFuncs()))
	}

	// And the spec drives ConstEqualities as the pushdown expects.
	ev := predicatefns.NewEvaluator(evaluatorMeta())
	prog, err := ev.CompileWithCurrentUser("task",
		"is_current_user(entity.assignee) and has_current_user(entity.watchers)")
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	want := []predicate.ConstEquality{
		{Attribute: "assignee", FromVar: "id"},
		{Attribute: "watchers", FromVar: "id", List: true},
	}
	got := prog.ConstEqualities(spec)
	if len(got) != len(want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("equality %d: got %+v, want %+v", i, got[i], want[i])
		}
	}
}
