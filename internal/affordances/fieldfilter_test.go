package affordances_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/affordances"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// TestFieldVisibleOnEveryRow_ResolvesRequestScope pins that the rule reads the
// same global roles and client ceiling that FieldVerdicts uses (BUG-WJSYQT),
// whether or not the acl.Request is already on ctx.
func TestFieldVisibleOnEveryRow_ResolvesRequestScope(t *testing.T) {
	t.Parallel()
	w := newCeilingWorld(t, hrCanSeeEverything+`
client_baselines:
  apps:
    applies_to: [app]
    redact:
      person: [salary]
`)
	appPrincipal := principal.VerifiedFrom("alice", principal.ToolDataEntry, principal.Claims{PrincipalType: "app"})
	// caller is "user" or "app" for a request scope on ctx, or "bare app"
	// for an app principal with no request scope attached.
	cases := []struct {
		name, caller, property string
		want                   bool
	}{
		{"user sees every field", "user", "salary", true},
		{"ceiling hides a field", "app", "salary", false},
		{"ceiling leaves other fields", "app", "email", true},
		{"ceiling applies without a request on ctx", "bare app", "salary", false},
		{"undeclared property is never hidden", "app", "nickname", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx := principal.With(context.Background(), appPrincipal)
			if tc.caller != "bare app" {
				ctx = w.ctxFor(t, tc.caller, nil)
			}
			if got := w.resolver.FieldVisibleOnEveryRow(ctx, "person", tc.property); got != tc.want {
				t.Errorf("FieldVisibleOnEveryRow(person, %q) = %v, want %v", tc.property, got, tc.want)
			}
		})
	}
}

// TestFieldVisibleOnEveryRow_UnstampedCaller pins the fallback for a caller
// with no verified principal: only the `everyone` role applies.
func TestFieldVisibleOnEveryRow_UnstampedCaller(t *testing.T) {
	t.Parallel()
	w := newCeilingWorld(t, `
roles:
  everyone:
    read: [person]
    visible:
      person:
        - field: name
`)
	ctx := context.Background()
	if w.resolver.FieldVisibleOnEveryRow(ctx, "person", "salary") {
		t.Error("salary: want hidden, the everyone role does not grant it")
	}
	if !w.resolver.FieldVisibleOnEveryRow(ctx, "person", "name") {
		t.Error("name: want visible, the everyone role grants it")
	}
}

// fieldFilterDiffPolicy mixes every shape the rule must judge: a global role
// without a block that reads the type (admin), a closed global role (viewer),
// a `when:` grant (auditor), an unconditional grant (hr), and two local roles,
// both reading tickets, one of which shows status on some rows only (watcher)
// and one that shows it everywhere (owner).
const fieldFilterDiffPolicy = `
roles:
  admin:
    read: [ticket]
  viewer:
    visible:
      ticket:
        - field: title
  auditor:
    visible:
      ticket:
        - field: title
        - field: status
          when: "entity.title == 'Alpha'"
  hr:
    visible:
      ticket:
        - field: title
        - field: status
  watcher:
    read: [ticket]
    visible:
      ticket:
        - field: title
        - field: status
          when: "entity.title == 'Alpha'"
  owner:
    read: [ticket]
    visible:
      ticket:
        - field: title
        - field: status
        - field: assignee
assignments:
  alice: viewer
  audrey: auditor
  harry: hr
  root: admin
role_relations:
  watches:
    confers: watcher
  owner-of:
    confers: owner
`

// TestFieldVisibleOnEveryRow_NeverContradictsFieldVerdicts is the
// differential pin (RR-5X1GV2): whenever the list gate lets a principal
// filter on a property, FieldVerdicts shows that property on every row,
// whichever local roles the principal holds there. The rows cover each
// combination of local role (none, watcher, owner) and `when:` outcome.
func TestFieldVisibleOnEveryRow_NeverContradictsFieldVerdicts(t *testing.T) {
	t.Parallel()
	users := []string{"alice", "audrey", "harry", "root", "nobody"}
	var edges [][3]string
	for _, u := range users {
		edges = append(edges, [3]string{u, "watches", "T-1"}, [3]string{u, "watches", "T-2"},
			[3]string{u, "owner-of", "T-3"}, [3]string{u, "owner-of", "T-4"})
	}
	r, err := affordances.New(testMeta(t), newStubLookup(edges...), declFor(t, policyFromYAML(t, fieldFilterDiffPolicy)))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	var rows []*entity.Entity
	for i, title := range []string{"Alpha", "Beta", "Alpha", "Beta", "Alpha", "Beta"} {
		rows = append(rows, ticket(fmt.Sprintf("T-%d", i+1), map[string]any{"title": title}))
	}

	for _, user := range users {
		for property := range testMeta(t).Entities["ticket"].Properties {
			if !r.FieldVisibleOnEveryRow(ctxAs(user), "ticket", property) {
				continue
			}
			for _, row := range rows {
				if _, hidden := r.FieldVerdicts(ctxAs(user), row).Visible[property]; hidden {
					t.Errorf("%s: gate allows %q, but FieldVerdicts hides it on %s", user, property, row.ID)
				}
			}
		}
	}

	// The expected answers. root's role declares no block and reads the type,
	// so it shows every field on every row whatever local roles root also
	// holds (BUG-Y5UF3K). The others read tickets only through the local
	// roles, so a field passes when both grant it without `when:` (title)
	// or a global block does (harry's status). A `when:` grant refuses even
	// where the condition holds on the row in front of it.
	for _, tc := range []struct {
		user, property string
		want           bool
	}{
		{"harry", "status", true},
		{"harry", "assignee", false},
		{"alice", "status", false},
		{"audrey", "status", false},
		{"root", "status", true},
		{"root", "assignee", true},
		{"nobody", "title", true},
		{"alice", "title", true},
		{"nobody", "status", false},
	} {
		if got := r.FieldVisibleOnEveryRow(ctxAs(tc.user), "ticket", tc.property); got != tc.want {
			t.Errorf("%s on %q = %v, want %v", tc.user, tc.property, got, tc.want)
		}
	}
}

// TestFieldVisibleOnEveryRow_NoReaderPasses pins that a principal for whom no
// role reads the type may filter and sort on any field. The read gate admits
// no row, so the list is an empty 200 either way, and a 400 would tell them
// nothing more.
func TestFieldVisibleOnEveryRow_NoReaderPasses(t *testing.T) {
	t.Parallel()
	policy := policyFromYAML(t, `
roles:
  viewer:
    read: [ticket]
    visible:
      ticket:
        - field: title
  outsider: {}
assignments:
  olga: outsider
`)
	r, err := affordances.New(testMeta(t), newStubLookup(), declFor(t, policy))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if !r.FieldVisibleOnEveryRow(ctxAs("olga"), "ticket", "status") {
		t.Error("olga reads no tickets: want the gate to pass, so the list is an empty 200")
	}
}

// TestFieldVisibleOnEveryRow_PerFace pins the face rule on a type with a draft
// and a published face. pia holds pubreader (no block, reads published only)
// and, through her team, policyclerk (reads every face, shows body only). On
// a published row pubreader shows every field; on a draft row only
// policyclerk applies, so owner is hidden there and the list must refuse it.
func TestFieldVisibleOnEveryRow_PerFace(t *testing.T) {
	t.Parallel()
	w := newUnionWorld(t, unionPolicy)
	for _, tc := range []struct {
		user, entityType, property string
		want                       bool
	}{
		{"pia", "policy", "body", true},
		{"pia", "policy", "owner", false},
		{"ulla", "policy", "owner", true},
		{"ulla", "ticket", "secret", true},
		{"carl", "ticket", "secret", false},
		{"carl", "ticket", "title", true},
	} {
		ctx := w.clientCtx(t, tc.user, "")
		if got := w.resolver.FieldVisibleOnEveryRow(ctx, tc.entityType, tc.property); got != tc.want {
			t.Errorf("%s on %s.%s = %v, want %v", tc.user, tc.entityType, tc.property, got, tc.want)
		}
	}

	// The verdicts the rule mirrors, per face pia reads.
	ctx := w.clientCtx(t, "pia", "")
	if _, hidden := w.resolver.FieldVerdicts(ctx, unionPolicyRow("published")).Visible["owner"]; hidden {
		t.Error("pia: owner hidden on a published row, want shown by pubreader")
	}
	if visible, judged := w.resolver.FieldVerdicts(ctx, unionPolicyRow("draft")).Visible["owner"]; !judged || visible {
		t.Error("pia: owner shown on a draft row, want hidden by policyclerk")
	}
}
