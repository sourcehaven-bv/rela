package affordances_test

import (
	"context"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/affordances"
)

// TestAffordances_CurrentUserSugar proves the shared sugar
// (is_current_user / has_current_user) is usable in an affordance
// `when:` and agrees with the explicit current_user.id comparison it
// replaces. Same package, same semantics, one implementation.
//
// It uses a `fields:` block: a `visible:` condition must also lower to a
// store query, which refuses `or` and list membership (see
// TestNew_VisibleWhenMustLower). The sugar evaluates the same in every block.
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
    fields:
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
			fv := r.FieldVerdicts(ctxFor(t, r, tc.user), ticket("T-1", tc.props))
			// A field is hidden only when the map says so explicitly;
			// absence means no grant narrowed it.
			v, ok := fv.Writable["assignee"]
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

// TestAffordances_CurrentUserSugar_UnknownPlaceholderNeverMatches pins the
// fail-closed reading for an UNIDENTIFIED caller: an unstamped ctx, or the
// "unknown" attribution placeholder the data-entry server stamps when it has
// no identity source. Neither resolves into an identity, so no request scope
// is bound and the resolver denies. An entity whose property literally holds
// "unknown" is not owned by an anonymous caller, in any of the three
// spellings. The control case proves the same grant DOES evaluate for a real
// principal, so the refusal is not an artifact of the role never applying.
func TestAffordances_CurrentUserSugar_UnknownPlaceholderNeverMatches(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		when        string
		props       map[string]any
		user        string // "" means an unstamped, unbound context
		wantVisible bool
	}{
		{"control: a real principal matches via everyone",
			"is_current_user(entity.assignee)", map[string]any{"assignee": "alice"}, "alice", true},
		{"unknown placeholder, sugar",
			"is_current_user(entity.assignee)", map[string]any{"assignee": "unknown"}, "unknown", false},
		{"unknown placeholder, explicit comparison",
			"entity.assignee == current_user.id", map[string]any{"assignee": "unknown"}, "unknown", false},
		{"unknown placeholder, list membership",
			"has_current_user(entity.tags)", map[string]any{"tags": []any{"unknown"}}, "unknown", false},
		{"unstamped context, sugar",
			"is_current_user(entity.assignee)", map[string]any{"assignee": "unknown"}, "", false},
		{"unstamped context, explicit comparison against an empty owner",
			"entity.assignee == current_user.id", map[string]any{"assignee": ""}, "", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			p := policyFromYAML(t, `
roles:
  everyone:
    fields:
      ticket:
        - field: assignee
          when: `+quoteYAML(tc.when)+`
`)
			r, err := affordances.New(testMeta(t), newStubLookup(), declFor(t, p))
			if err != nil {
				t.Fatalf("New: %v", err)
			}
			// The "unknown" placeholder cannot be resolved into an identity,
			// so the edge binds nothing: stamp it without binding.
			ctx := context.Background()
			switch tc.user {
			case "":
			case "unknown":
				ctx = ctxAs(tc.user)
			default:
				ctx = ctxFor(t, r, tc.user)
			}
			fv := r.FieldVerdicts(ctx, ticket("T-1", tc.props))
			v, ok := fv.Writable["assignee"]
			hidden := ok && !v
			if hidden == tc.wantVisible {
				t.Fatalf("when %q: visible=%v, want visible=%v", tc.when, !hidden, tc.wantVisible)
			}
		})
	}
}
