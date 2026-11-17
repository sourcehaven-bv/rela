package affordances_test

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/acl/acltest"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/readtime"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// TestSearchFieldScope_ConditionalRoleRelation (TKT-TKSCSBT): the owner role
// is held through owner-of (unconditional) and through reviews (only while
// edge.valid_until >= today()). The search scope and FieldVerdicts must agree
// on every row at every read time, and the condition must change the answer.
//
//   - erin --reviews{2026-12-31}--> T-3
//   - erin --reviews{2026-01-01}--> T-4
//   - erin --owner-of--> T-2
func TestSearchFieldScope_ConditionalRoleRelation(t *testing.T) {
	t.Parallel()
	w := newFieldWorld(t, fieldWorldPolicy,
		func(meta *metamodel.Metamodel, p *acl.Policy, ms *memstore.MemStore) {
			meta.Relations["reviews"] = metamodel.RelationDef{
				From: []string{"person"}, To: []string{"ticket"},
				Properties: map[string]metamodel.PropertyDef{"valid_until": {Type: metamodel.PropertyTypeDate}},
			}
			p.RoleRelations["reviews"] = acl.RoleRelationDef{
				Confers: "owner", RequiresPermission: "delegate", When: "edge.valid_until >= today()",
				Condition: &acl.GrantCondition{Terms: []acl.GrantTerm{{
					Property: "valid_until", Op: acl.GrantGE, Operand: acl.OperandToday, Date: true,
				}}},
			}
			mustCreateEntity(t, ms, "erin", "person")
			mustCreateRelation(t, ms, "erin", "owner-of", "T-2")
			for to, until := range map[string]string{"T-3": "2026-12-31", "T-4": "2026-01-01"} {
				_, err := ms.CreateRelation(context.Background(),
					entity.RelationKey{From: "erin", Type: "reviews", To: to},
					&store.RelationData{Properties: map[string]any{"valid_until": until}})
				if err != nil {
					t.Fatalf("create reviews: %v", err)
				}
			}
		})
	hidden := map[string]map[string][]string{}
	for name, at := range map[string]time.Time{
		"live":    time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC),
		"expired": time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC),
		"no time": {},
	} {
		ctx := principal.With(context.Background(), principal.Principal{User: "erin", Tool: principal.ToolDataEntry})
		if !at.IsZero() {
			ctx = readtime.With(ctx, at)
		}
		ctx = acltest.Bind(ctx, t, w.decl)
		scope, err := w.resolver.SearchFieldScope(ctx, "ticket")
		if err != nil {
			t.Fatalf("%s: SearchFieldScope: %v", name, err)
		}
		hidden[name] = map[string][]string{}
		for _, id := range []string{"T-1", "T-2", "T-3", "T-4"} {
			row := w.tickets[id]
			want := hiddenFields(w.resolver.FieldVerdicts(ctx, row))
			if got := scopeHidden(t, w, scope, row); !slices.Equal(got, want) {
				t.Errorf("%s %s: scope hides %v, FieldVerdicts hides %v", name, id, got, want)
			}
			hidden[name][id] = want
		}
	}
	if slices.Equal(hidden["live"]["T-3"], hidden["live"]["T-4"]) {
		t.Errorf("live: the live reviews edge (T-3) and the expired one (T-4) hide the same %v",
			hidden["live"]["T-3"])
	}
	if !slices.Equal(hidden["expired"]["T-3"], hidden["expired"]["T-4"]) ||
		!slices.Equal(hidden["no time"]["T-3"], hidden["no time"]["T-4"]) {

		t.Errorf("after expiry and without a read time, T-3 and T-4 must look alike: %v", hidden)
	}
	if !slices.Equal(hidden["no time"]["T-2"], hidden["live"]["T-2"]) {
		t.Errorf("owner-of is unconditional: T-2 must not depend on the read time: %v", hidden)
	}
}
