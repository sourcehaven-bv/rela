package affordances_test

import (
	"errors"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/affordances"
)

// metaWritePolicy restricts `cites` meta writes through `clerk`, held by carl
// directly and by cora and ulla through the clerks team. cora also holds
// `creator`, which may not update tickets; ulla also holds `updater`, which
// may update them and declares no meta `fields:` for `cites`. pia also
// holds `clerk` through the policyclerks team and `reviewer` directly, which
// may update tickets but whose `cites` grant names
// no meta fields and applies only to done tickets.
const metaWritePolicy = `
roles:
  clerk:
    read: [ticket]
    update: [ticket]
    relations:
      ticket:
        - relation: cites
          fields:
            - field: note
  creator:
    read: [ticket]
    create: [ticket]
  updater:
    read: [ticket]
    update: [ticket]
  reviewer:
    read: [ticket]
    update: [ticket]
    relations:
      ticket:
        - relation: cites
          when: "entity.status == 'done'"
assignments:
  clerks: clerk
  carl: clerk
  cora: creator
  ulla: updater
  policyclerks: clerk
  pia: reviewer
`

// TestRelationMeta_FieldsGrantIsAnAllowlist pins BUG-O8QP7GX: once a role
// declares meta `fields:` for a relation type, a meta key no role grants is
// read-only, whether the metamodel declares it or not, as it is for entity
// `fields:` grants. A role that may update the source and declares no meta
// list lifts the restriction (role union); a grant whose when: fails does
// not. The order property stays writable: it is a position, not content.
func TestRelationMeta_FieldsGrantIsAnAllowlist(t *testing.T) {
	t.Parallel()
	w := newUnionWorld(t, metaWritePolicy)
	tests := []struct {
		user     string
		writable map[string]bool
	}{
		{"carl", map[string]bool{"note": true, "reason": false, "free-form": false, "_order_out": true}},
		{"cora", map[string]bool{"note": true, "reason": false, "free-form": false, "_order_out": true}},
		{"ulla", map[string]bool{"note": true, "reason": true, "free-form": true, "_order_out": true}},
		{"pia", map[string]bool{"note": true, "reason": false, "free-form": false, "_order_out": true}},
	}
	for _, tc := range tests {
		t.Run(tc.user, func(t *testing.T) {
			t.Parallel()
			ctx := userCtx(t, w.decl, tc.user)
			for name, verdicts := range map[string]affordances.RelationVerdicts{
				"conditional":   w.resolver.RelationVerdicts(ctx, unionTicket()),
				"unconditional": w.resolver.UnconditionalRelationVerdicts(ctx, unionTicket()),
			} {
				rv, restricted := verdicts.Types["cites"]
				for key, want := range tc.writable {
					got := !restricted || rv.MetaWritable(key)
					if got != want {
						t.Errorf("%s: meta %q writable = %v, want %v (verdict %+v)", name, key, got, want, rv)
					}
				}
			}
		})
	}
}

// TestWriteGate_RelationMetaFollowsTheAllowlist pins the gate the field-gated
// entity manager (MCP) asks: it refuses the meta keys the verdict above
// leaves read-only, with the same rule id dataentry serves.
func TestWriteGate_RelationMetaFollowsTheAllowlist(t *testing.T) {
	t.Parallel()
	w := newUnionWorld(t, metaWritePolicy)
	gate, err := affordances.NewWriteGate(w.resolver)
	if err != nil {
		t.Fatalf("NewWriteGate: %v", err)
	}
	carl := userCtx(t, w.decl, "carl")
	if err := gate.CheckRelationMetaWrite(carl, unionTicket(), "cites", map[string]any{"note": "x"}, nil); err != nil {
		t.Errorf("granted key refused: %v", err)
	}
	for name, tc := range map[string]struct {
		set   map[string]any
		unset []string
	}{
		"set":   {set: map[string]any{"free-form": "x"}},
		"unset": {unset: []string{"free-form"}},
	} {
		err := gate.CheckRelationMetaWrite(carl, unionTicket(), "cites", tc.set, tc.unset)
		var d *affordances.FieldWriteError
		if !errors.As(err, &d) || d.RuleID() != "relation-affordance:meta-read-only:cites.free-form" {
			t.Errorf("%s of an ungranted key = %v, want the meta-read-only rule", name, err)
		}
	}
	if err := gate.CheckRelationMetaWrite(userCtx(t, w.decl, "ulla"), unionTicket(), "cites",
		map[string]any{"free-form": "x"}, nil); err != nil {
		t.Errorf("role union: %v", err)
	}
}
