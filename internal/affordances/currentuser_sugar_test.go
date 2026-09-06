package affordances_test

import (
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/affordances"
)

// TestAffordances_CurrentUserSugar proves the shared sugar
// (is_current_user / has_current_user) is usable in an affordance
// `when:` and agrees with the explicit current_user.id comparison it
// replaces. Same package, same semantics, one implementation.
func TestAffordances_CurrentUserSugar(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		when        string
		props       map[string]any
		user        string
		wantVisible bool
	}{
		{
			name:        "is_current_user matches the owner",
			when:        "is_current_user(entity.assignee)",
			props:       map[string]any{"assignee": "alice"},
			user:        "alice",
			wantVisible: true,
		},
		{
			name:        "is_current_user rejects another user",
			when:        "is_current_user(entity.assignee)",
			props:       map[string]any{"assignee": "alice"},
			user:        "bob",
			wantVisible: false,
		},
		{
			name:        "is_current_user on an unset property is a non-match",
			when:        "is_current_user(entity.assignee)",
			props:       nil,
			user:        "alice",
			wantVisible: false,
		},
		{
			name:        "has_current_user finds the user in a list property",
			when:        "has_current_user(entity.tags)",
			props:       map[string]any{"tags": []any{"bob", "alice"}},
			user:        "alice",
			wantVisible: true,
		},
		{
			name:        "has_current_user rejects a list without the user",
			when:        "has_current_user(entity.tags)",
			props:       map[string]any{"tags": []any{"bob"}},
			user:        "alice",
			wantVisible: false,
		},
		{
			name:        "the sugar agrees with the explicit comparison",
			when:        "entity.assignee == current_user.id",
			props:       map[string]any{"assignee": "alice"},
			user:        "alice",
			wantVisible: true,
		},
		{
			name:        "the sugar composes with the existing host funcs",
			when:        "is_current_user(entity.assignee) or has_global_role(current_user, 'viewer')",
			props:       map[string]any{"assignee": "nobody"},
			user:        "alice",
			wantVisible: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			p := policyFromYAML(t, `
roles:
  viewer:
    visible:
      ticket:
        - field: assignee
          when: `+quoteYAML(tc.when)+`
assignments:
  alice: viewer
  bob: viewer
`)
			r, err := affordances.New(testMeta(t), newStubLookup(), declFor(t, p))
			if err != nil {
				t.Fatalf("New: %v", err)
			}
			fv := r.FieldVerdicts(ctxAs(tc.user), ticket("T-1", tc.props))
			// A field is hidden only when the map says so explicitly;
			// absence means no grant narrowed it.
			v, ok := fv.Visible["assignee"]
			hidden := ok && !v
			if hidden == tc.wantVisible {
				t.Fatalf("when %q as %q: visible=%v, want visible=%v",
					tc.when, tc.user, !hidden, tc.wantVisible)
			}
		})
	}
}

// quoteYAML wraps an expression in double quotes for embedding in the
// test policy, escaping the single-quoted string literals predicate
// expressions use.
func quoteYAML(s string) string {
	return `"` + s + `"`
}
