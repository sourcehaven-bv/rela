package affordances_test

import (
	"context"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/acl/acltest"
	"github.com/Sourcehaven-BV/rela/internal/aclstore"
	"github.com/Sourcehaven-BV/rela/internal/affordances"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// Role union (BUG-Y5UF3K). Each role a principal holds on a row contributes a
// set per dimension, and the verdict is the union of those sets. A role with
// no block for the type contributes every declared field (or every option),
// provided it holds the matching verb on the type: read for `visible:`,
// update for `fields:` and `options:`. Gaining a role therefore never narrows
// what the principal sees or may write.

// roleUnionMeta declares a ticket with three fields and an enum status.
func roleUnionMeta() *metamodel.Metamodel {
	return &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"ticket": {
				Properties: map[string]metamodel.PropertyDef{
					"title":  {Type: metamodel.PropertyTypeString},
					"status": {Type: metamodel.PropertyTypeEnum, Values: []string{"open", "review", "done"}},
					"secret": {Type: metamodel.PropertyTypeString},
				},
			},
			"person": {Properties: map[string]metamodel.PropertyDef{}},
		},
		Relations: map[string]metamodel.RelationDef{
			"watches": {From: []string{"person"}, To: []string{"ticket"}},
		},
	}
}

// roleUnionPolicy gives every user a global role and confers `watcher` on
// TKT-1 through a `watches` edge. watcher restricts all three dimensions.
const roleUnionPolicy = `
role_relations:
  watches:
    confers: watcher
roles:
  admin:
    read: ["*"]
    update: ["*"]
  reader:
    read: [ticket]
  viewer:
    read: [person]
  watcher:
    read: [ticket]
    update: [ticket]
    visible:
      ticket:
        - field: title
        - field: status
    fields:
      ticket:
        - field: title
    options:
      ticket:
        - field: status
          option: open
assignments:
  alice: admin
  rita: reader
  vic: viewer
`

type roleUnionWorld struct {
	resolver *affordances.PolicyResolver
	decl     *acl.Declarative
	watched  *entity.Entity
	other    *entity.Entity
}

func newRoleUnionWorld(t *testing.T, policyYAML string) roleUnionWorld {
	t.Helper()
	policy, err := acl.LoadPolicyBytes([]byte(policyYAML))
	if err != nil {
		t.Fatalf("LoadPolicyBytes: %v", err)
	}
	ms := memstore.New()
	for _, user := range []string{"alice", "rita", "vic"} {
		mustCreateEntity(t, ms, user, "person")
	}
	watched := entity.New("TKT-1", "ticket")
	watched.SetString("title", "t")
	watched.SetString("status", "open")
	watched.SetString("secret", "s")
	mustCreate(t, ms, watched)
	other := entity.New("TKT-2", "ticket")
	mustCreate(t, ms, other)
	for _, user := range []string{"alice", "rita", "vic"} {
		mustCreateRelation(t, ms, user, "watches", "TKT-1")
	}
	decl, err := acl.NewDeclarative(policy, aclstore.MustNewGraph(ms), ms)
	if err != nil {
		t.Fatalf("NewDeclarative: %v", err)
	}
	resolver, err := affordances.New(roleUnionMeta(), storeRelationLookup{ms}, decl)
	if err != nil {
		t.Fatalf("affordances.New: %v", err)
	}
	return roleUnionWorld{resolver: resolver, decl: decl, watched: watched, other: other}
}

// userCtx stamps user on a context and binds its Request on decl, as the
// request edge does.
func userCtx(tb testing.TB, decl *acl.Declarative, user string) context.Context {
	tb.Helper()
	ctx := principal.With(context.Background(),
		principal.Principal{User: user, Tool: principal.ToolDataEntry})
	return acltest.Bind(ctx, tb, decl)
}

// deniedFields returns the fields a sparse verdict map marks false.
func deniedFields(m map[string]bool) []string {
	var out []string
	for _, f := range []string{"title", "status", "secret"} {
		if v, ok := m[f]; ok && !v {
			out = append(out, f)
		}
	}
	return out
}

// deniedOptions returns the status options a verdict marks false.
func deniedOptions(m map[string]map[string]bool) []string {
	var out []string
	for _, o := range []string{"open", "review", "done"} {
		if v, ok := m["status"][o]; ok && !v {
			out = append(out, o)
		}
	}
	return out
}

func sameStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// TestRoleUnion_ConferredRoleNeverNarrows is the reproduction for BUG-Y5UF3K:
// alice is admin and also watches TKT-1. Before the fix the watcher's blocks
// closed every dimension and alice lost secret on the row she watches.
func TestRoleUnion_ConferredRoleNeverNarrows(t *testing.T) {
	t.Parallel()
	w := newRoleUnionWorld(t, roleUnionPolicy)

	tests := []struct {
		name          string
		user          string
		row           func(roleUnionWorld) *entity.Entity
		hidden        []string
		readOnly      []string
		deniedOptions []string
	}{
		{
			name: "admin watching keeps every field",
			user: "alice", row: func(w roleUnionWorld) *entity.Entity { return w.watched },
		},
		{
			name: "admin on an unwatched row is unrestricted",
			user: "alice", row: func(w roleUnionWorld) *entity.Entity { return w.other },
		},
		{
			// reader holds read but not update: it widens visible only.
			name: "reader watching sees every field but writes only the watcher's",
			user: "rita", row: func(w roleUnionWorld) *entity.Entity { return w.watched },
			readOnly: []string{"status", "secret"}, deniedOptions: []string{"review", "done"},
		},
		{
			// viewer holds no verb on ticket, so it contributes nothing and
			// the watcher's grants are the whole answer.
			name: "viewer watching stays restricted to the watcher's grants",
			user: "vic", row: func(w roleUnionWorld) *entity.Entity { return w.watched },
			hidden: []string{"secret"}, readOnly: []string{"status", "secret"},
			deniedOptions: []string{"review", "done"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			fv := w.resolver.FieldVerdicts(userCtx(t, w.decl, tc.user), tc.row(w))
			if got := deniedFields(fv.Visible); !sameStrings(got, tc.hidden) {
				t.Errorf("hidden = %v, want %v", got, tc.hidden)
			}
			if got := deniedFields(fv.Writable); !sameStrings(got, tc.readOnly) {
				t.Errorf("read-only = %v, want %v", got, tc.readOnly)
			}
			if got := deniedOptions(fv.Options); !sameStrings(got, tc.deniedOptions) {
				t.Errorf("denied options = %v, want %v", got, tc.deniedOptions)
			}
		})
	}
}
