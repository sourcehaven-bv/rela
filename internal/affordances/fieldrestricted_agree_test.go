package affordances_test

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// TestFieldRestricted_AgreesWithFieldVerdicts pins the load-time answer of
// [acl.Policy.FieldRestricted] to the read path: a property it does not
// restrict is shown by FieldVerdicts to every principal on every row that
// principal may read. Flow guards and templates and realtime rules rely on
// this, because their output reaches every reader of the subject.
//
// Live reads only: a historical read drops relation-conferred roles and so
// may hide more, which fails closed rather than disclosing anything.
func TestFieldRestricted_AgreesWithFieldVerdicts(t *testing.T) {
	t.Parallel()
	// The auditor's block lists title too, so every ticket and page block
	// lists title without a when: and title is the unrestricted property.
	auditorSecret := "        - field: secret\n          when: \"entity.status == 'done'\"\n"
	if !strings.Contains(fieldWorldPolicy, auditorSecret) {
		t.Fatal("the fixture policy has no auditor secret grant")
	}
	everyBlockShowsTitle := strings.Replace(fieldWorldPolicy, auditorSecret,
		"        - field: title\n"+auditorSecret, 1)
	policies := []struct {
		name, yaml   string
		unrestricted []string // type.property FieldRestricted must not restrict
	}{
		{"every block lists title", everyBlockShowsTitle, []string{"ticket.title", "page.title"}},
		{"one block omits title", fieldWorldPolicy, []string{"page.title"}},
		{"a baseline redacts title", everyBlockShowsTitle + `
client_baselines:
  apps:
    applies_to: [app]
    redact:
      ticket: [title]
`, []string{"page.title"}},
	}
	for _, pc := range policies {
		t.Run(pc.name, func(t *testing.T) {
			t.Parallel()
			var policy *acl.Policy
			w := newFieldWorld(t, pc.yaml, func(_ *metamodel.Metamodel, p *acl.Policy, _ *memstore.MemStore) {
				policy = p
			})
			var got []string
			for _, typ := range []string{"ticket", "page"} {
				for prop := range w.meta.Entities[typ].Properties {
					if !policy.FieldRestricted(typ, prop) {
						got = append(got, typ+"."+prop)
					}
				}
			}
			slices.Sort(got)
			want := slices.Sorted(slices.Values(pc.unrestricted))
			if !slices.Equal(got, want) {
				t.Fatalf("unrestricted = %v; want %v", got, want)
			}
			rows := []*entity.Entity{w.tickets["T-1"], w.tickets["T-2"], w.tickets["T-3"], w.tickets["T-4"]}
			rows = append(rows, w.pages...)
			checked := 0
			for _, user := range []string{"alice", "bob", "carol", "dave", "rita", "pia"} {
				for _, via := range []string{"", "app", "bot"} {
					ctx := w.ctxFor(t, user)
					if via != "" {
						ctx = w.ctxVia(t, user, via)
					}
					checked += assertUnrestrictedShown(ctx, t, w, policy, rows, user+"@"+via)
				}
			}
			if checked == 0 {
				t.Fatal("no principal read a row with an unrestricted property; the test checks nothing")
			}
		})
	}
}

// assertUnrestrictedShown checks every unrestricted property on every row
// ctx may read, and returns how many it checked.
func assertUnrestrictedShown(
	ctx context.Context, t *testing.T, w fieldWorld, policy *acl.Policy, rows []*entity.Entity, who string,
) int {
	t.Helper()
	req, err := w.decl.RequestFrom(ctx)
	if err != nil {
		t.Fatalf("%s: RequestFrom: %v", who, err)
	}
	checked := 0
	for _, row := range rows {
		ok, err := req.PermitsReadFace(ctx, row.Type, row.ID, row.Face)
		if err != nil {
			t.Fatalf("%s: PermitsReadFace(%s): %v", who, row.ID, err)
		}
		if !ok {
			continue
		}
		verdicts := w.resolver.FieldVerdicts(ctx, row)
		for prop := range w.meta.Entities[row.Type].Properties {
			if policy.FieldRestricted(row.Type, prop) {
				continue
			}
			checked++
			if shown, gated := verdicts.Visible[prop]; gated && !shown {
				t.Errorf("%s: %s@%s hides %s, which FieldRestricted does not restrict", who, row.ID, row.Face, prop)
			}
		}
	}
	return checked
}
