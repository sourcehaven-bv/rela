package predicatefns_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/predicate"
	"github.com/Sourcehaven-BV/rela/internal/predicatefns"
)

// compileWithUser builds the request-scoped profile: entity + stdlib +
// current_user. Mirrors what a view condition would use.
func compileWithUser(t *testing.T, src string, entityFields predicate.RecordType) *predicate.Program {
	t.Helper()
	env := predicate.NewEnv()
	if err := env.DeclareVar("entity", entityFields); err != nil {
		t.Fatalf("declare entity: %v", err)
	}
	if err := predicatefns.Declare(env); err != nil {
		t.Fatalf("declare stdlib: %v", err)
	}
	if err := predicatefns.DeclareCurrentUser(env); err != nil {
		t.Fatalf("declare current_user: %v", err)
	}
	prog, err := predicate.Compile(env, src)
	if err != nil {
		t.Fatalf("compile %q: %v", src, err)
	}
	return prog
}

func evalWithUser(
	ctx context.Context, t *testing.T, prog *predicate.Program, entity predicate.Value,
) (bool, error) {
	t.Helper()
	b := predicate.NewBindings()
	if err := b.SetVar("entity", entity); err != nil {
		t.Fatalf("bind entity: %v", err)
	}
	if err := predicatefns.Bind(b, testNow()); err != nil {
		t.Fatalf("bind stdlib: %v", err)
	}
	if err := predicatefns.BindCurrentUser(ctx, b); err != nil {
		return false, err
	}
	v, err := prog.Eval(ctx, b)
	if err != nil {
		return false, err
	}
	bv, ok := v.(predicate.Bool)
	if !ok {
		t.Fatalf("program did not return bool, got %T", v)
	}
	return bv.Bool(), nil
}

func meCtx(entityID, raw, tool string) context.Context {
	return predicatefns.WithQueryIdentity(context.Background(), predicatefns.QueryIdentity{
		EntityID: entityID, Raw: raw, Tool: tool,
	})
}

// TestCurrentUser_EqualityAndSugar pins the three spellings an operator
// may write for "assigned to me" and proves they agree.
func TestCurrentUser_EqualityAndSugar(t *testing.T) {
	fields := predicate.RecordType{
		"assignee": predicate.StringType,
		"watchers": predicate.ListType{Elem: predicate.StringType},
	}
	ctx := meCtx("PERS-JV", "jeroen@example.com", "data-entry")

	tests := []struct {
		name     string
		src      string
		assignee predicate.Value
		watchers predicate.Value
		want     bool
	}{
		{
			name:     "explicit id comparison matches",
			src:      "entity.assignee == current_user.id",
			assignee: predicate.NewString("PERS-JV"),
			want:     true,
		},
		{
			name:     "explicit id comparison rejects another user",
			src:      "entity.assignee == current_user.id",
			assignee: predicate.NewString("PERS-AB"),
			want:     false,
		},
		{
			name:     "is_me sugar matches",
			src:      "is_me(entity.assignee)",
			assignee: predicate.NewString("PERS-JV"),
			want:     true,
		},
		{
			name:     "is_me on an unset property is a non-match, not an error",
			src:      "is_me(entity.assignee)",
			assignee: predicate.NewNil(),
			want:     false,
		},
		{
			name:     "me_in finds the user in a list",
			src:      "me_in(entity.watchers)",
			assignee: predicate.NewNil(),
			watchers: predicate.NewList([]predicate.Value{
				predicate.NewString("PERS-AB"), predicate.NewString("PERS-JV"),
			}),
			want: true,
		},
		{
			name:     "me_in rejects a list without the user",
			src:      "me_in(entity.watchers)",
			assignee: predicate.NewNil(),
			watchers: predicate.NewList([]predicate.Value{predicate.NewString("PERS-AB")}),
			want:     false,
		},
		{
			name:     "me_in on an unset list is a non-match",
			src:      "me_in(entity.watchers)",
			assignee: predicate.NewNil(),
			watchers: predicate.NewNil(),
			want:     false,
		},
		{
			name:     "the inbox shape: mine or watched, composed with or",
			src:      "is_me(entity.assignee) or me_in(entity.watchers)",
			assignee: predicate.NewString("PERS-AB"),
			watchers: predicate.NewList([]predicate.Value{predicate.NewString("PERS-JV")}),
			want:     true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			watchers := tc.watchers
			if watchers == nil {
				watchers = predicate.NewList(nil)
			}
			prog := compileWithUser(t, tc.src, fields)
			got, err := evalWithUser(ctx, t, prog, predicate.NewRecord(map[string]predicate.Value{
				"assignee": tc.assignee,
				"watchers": watchers,
			}))
			if err != nil {
				t.Fatalf("eval: %v", err)
			}
			if got != tc.want {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}

// TestCurrentUser_RecordComparisonIsACompileError documents WHY the
// ergonomic `entity.assignee == current_user` is not the spelling: the
// variable must stay a record for affordance compatibility, and the type
// checker rejects record-vs-string. is_me() is the sugar that replaces it.
func TestCurrentUser_RecordComparisonIsACompileError(t *testing.T) {
	env := predicate.NewEnv()
	if err := env.DeclareVar("entity", predicate.RecordType{"assignee": predicate.StringType}); err != nil {
		t.Fatalf("declare entity: %v", err)
	}
	if err := predicatefns.Declare(env); err != nil {
		t.Fatalf("declare stdlib: %v", err)
	}
	if err := predicatefns.DeclareCurrentUser(env); err != nil {
		t.Fatalf("declare current_user: %v", err)
	}
	if _, err := predicate.Compile(env, "entity.assignee == current_user"); err == nil {
		t.Fatal("expected a compile error comparing a string to a record")
	}
}

// TestCurrentUser_UndeclaredInStdlibProfile is the load-time guard: a
// profile that did not opt in must REJECT a current_user reference
// rather than evaluate it against a guessed identity.
func TestCurrentUser_UndeclaredInStdlibProfile(t *testing.T) {
	for _, src := range []string{"entity.assignee == current_user.id", "is_me(entity.assignee)"} {
		t.Run(src, func(t *testing.T) {
			env := predicate.NewEnv()
			if err := env.DeclareVar("entity", predicate.RecordType{"assignee": predicate.StringType}); err != nil {
				t.Fatalf("declare entity: %v", err)
			}
			if err := predicatefns.Declare(env); err != nil {
				t.Fatalf("declare stdlib: %v", err)
			}
			// Deliberately no DeclareCurrentUser.
			if _, err := predicate.Compile(env, src); err == nil {
				t.Fatalf("expected a compile error for %q in a profile without current_user", src)
			}
		})
	}
}

// TestCurrentUser_BindFailsClosed proves an unidentified context cannot
// silently widen a filter: binding errors instead of producing an empty
// identity that would match unset properties.
func TestCurrentUser_BindFailsClosed(t *testing.T) {
	tests := []struct {
		name string
		// stamp reports whether an identity is put on the context at
		// all, distinguishing "never stamped" from "stamped but empty".
		stamp bool
		tool  string
	}{
		{name: "no identity stamped", stamp: false},
		{name: "zero identity", stamp: true},
		{name: "tool without identity", stamp: true, tool: "scheduler"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			if tc.stamp {
				ctx = meCtx("", "", tc.tool)
			}
			b := predicate.NewBindings()
			err := predicatefns.BindCurrentUser(ctx, b)
			if !errors.Is(err, predicatefns.ErrNoCurrentUser) {
				t.Fatalf("got %v, want ErrNoCurrentUser", err)
			}
		})
	}
}

// TestQueryIdentity_PrefersEntityIDFallsBackToRaw pins the resolution
// rule: an entity id when the deployment has one (so a comparison
// against entity.assignee works), the raw principal when it does not.
func TestQueryIdentity_PrefersEntityIDFallsBackToRaw(t *testing.T) {
	tests := []struct {
		name  string
		q     predicatefns.QueryIdentity
		want  string
		valid bool
	}{
		{
			name:  "entity id wins when resolved",
			q:     predicatefns.QueryIdentity{EntityID: "PERS-JV", Raw: "jeroen@example.com"},
			want:  "PERS-JV",
			valid: true,
		},
		{
			name:  "raw principal when no user entity type",
			q:     predicatefns.QueryIdentity{Raw: "jeroen@example.com"},
			want:  "jeroen@example.com",
			valid: true,
		},
		{
			name:  "zero identity is invalid",
			q:     predicatefns.QueryIdentity{},
			want:  "",
			valid: false,
		},
		{
			name:  "a tool alone does not make an identity",
			q:     predicatefns.QueryIdentity{Tool: "cli"},
			want:  "",
			valid: false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.q.ID(); got != tc.want {
				t.Fatalf("ID() = %q, want %q", got, tc.want)
			}
			if got := tc.q.Valid(); got != tc.valid {
				t.Fatalf("Valid() = %v, want %v", got, tc.valid)
			}
		})
	}
}

// TestCurrentUser_RawFallbackComparesAgainstRawPrincipal covers the
// no-user_entity_type deployment (the data-entry prototype): the raw
// identifier is what the graph can reference, so that is what binds.
func TestCurrentUser_RawFallbackComparesAgainstRawPrincipal(t *testing.T) {
	prog := compileWithUser(t, "is_me(entity.reporter)", predicate.RecordType{
		"reporter": predicate.StringType,
	})
	ctx := meCtx("", "jeroen@example.com", "data-entry")
	got, err := evalWithUser(ctx, t, prog, predicate.NewRecord(map[string]predicate.Value{
		"reporter": predicate.NewString("jeroen@example.com"),
	}))
	if err != nil {
		t.Fatalf("eval: %v", err)
	}
	if !got {
		t.Fatal("expected the raw principal to match reporter")
	}
}

// TestCurrentUser_SQLPortableClassification guards the pushdown
// precondition: a current-user condition must classify as portable, or a
// future predicate->SQL compiler could never push it down.
func TestCurrentUser_SQLPortableClassification(t *testing.T) {
	fields := predicate.RecordType{
		"assignee": predicate.StringType,
		"watchers": predicate.ListType{Elem: predicate.StringType},
	}
	tests := []struct {
		src      string
		portable bool
	}{
		{src: "entity.assignee == current_user.id", portable: true},
		{src: "is_me(entity.assignee)", portable: true},
		{src: "me_in(entity.watchers)", portable: true},
		{src: "is_me(entity.assignee) or me_in(entity.watchers)", portable: true},
		// A non-portable stdlib func still poisons the program, proving the
		// classification is genuinely computed rather than always true.
		{src: "is_me(sha256(entity.assignee))", portable: false},
	}
	for _, tc := range tests {
		t.Run(tc.src, func(t *testing.T) {
			prog := compileWithUser(t, tc.src, fields)
			if got := prog.SQLPortable(); got != tc.portable {
				t.Fatalf("SQLPortable() = %v, want %v", got, tc.portable)
			}
		})
	}
}

// TestCurrentUser_TypeMatchesAffordances is the drift guard. The two
// dialects are one language to an operator, so the record shape must not
// diverge; if affordances gains a field, this fails until it is mirrored.
func TestCurrentUser_TypeMatchesAffordances(t *testing.T) {
	want := []string{"id", "tool"}
	for _, f := range want {
		if _, ok := predicatefns.CurrentUserType[f]; !ok {
			t.Fatalf("current_user is missing field %q", f)
		}
	}
	if len(predicatefns.CurrentUserType) != len(want) {
		t.Fatalf("current_user has %d fields, want %d (%s) — mirror internal/affordances/env.go",
			len(predicatefns.CurrentUserType), len(want), strings.Join(want, ", "))
	}
}
