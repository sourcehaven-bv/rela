package affordances_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/affordances"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// BenchmarkVerdicts pins the per-entity affordance resolution cost
// (TKT-9Y4ZWS): every data-entry response carries `_fields` and
// relation verdicts, so this path runs once per entity per request.
// The fixture mirrors the UC10 feature-test shape — a real Declarative
// with a group-conferred role (member-of walk through the store graph)
// and predicate-gated grants, so the number includes the role walk and
// predicate evaluation, not just map lookups.
//
// Note: this is the per-call path without a ctx-cached acl.Request —
// the production router attaches one per HTTP request to amortize the
// member-of walk (see TestResolver_ReusesRequestFromContext). This
// benchmark is the uncached upper bound.
func BenchmarkVerdicts(b *testing.B) {
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"ticket": {
				Properties: map[string]metamodel.PropertyDef{
					"title":          {Type: metamodel.PropertyTypeString},
					"status":         {Type: metamodel.PropertyTypeString},
					"internal_notes": {Type: metamodel.PropertyTypeString},
				},
			},
			"person": {},
			"team":   {},
		},
		Relations: map[string]metamodel.RelationDef{
			"depends-on": {From: []string{"ticket"}, To: []string{"ticket"}},
		},
	}

	policy, err := acl.LoadPolicyBytes([]byte(`
roles:
  editor:
    write: [ticket]
    fields:
      ticket:
        - field: status
          when: "entity.status == 'open'"
        - field: title
    visible:
      ticket:
        - field: title
        - field: status
    relations:
      ticket:
        - relation: depends-on
          when: "entity.status == 'open'"
assignments:
  editors: editor
`))
	if err != nil {
		b.Fatal(err)
	}

	ms := memstore.New()
	ctx := context.Background()
	seed := func(id, typ string) {
		if seedErr := ms.CreateEntity(ctx, entity.New(id, typ)); seedErr != nil {
			b.Fatalf("seed %s: %v", id, seedErr)
		}
	}
	seed("alice", "person")
	seed("editors", "team")
	if _, relErr := ms.CreateRelation(ctx, "alice", "member-of", "editors", nil); relErr != nil {
		b.Fatal(relErr)
	}
	for i := range 200 {
		seed(fmt.Sprintf("TKT-%03d", i), "ticket")
	}
	// Some outgoing edges so OutgoingCounts has work to do.
	for i := range 50 {
		from := fmt.Sprintf("TKT-%03d", i)
		to := fmt.Sprintf("TKT-%03d", i+100)
		if _, relErr := ms.CreateRelation(ctx, from, "depends-on", to, nil); relErr != nil {
			b.Fatal(relErr)
		}
	}

	declarative, err := acl.NewDeclarative(policy, acl.NewStoreGraph(ms))
	if err != nil {
		b.Fatal(err)
	}
	resolver, err := affordances.New(meta, storeRelationLookup{ms}, declarative)
	if err != nil {
		b.Fatal(err)
	}

	tkt := entity.New("TKT-007", "ticket")
	tkt.SetString("title", "broken login")
	tkt.SetString("status", "open")
	tkt.SetString("internal_notes", "do not leak")

	pctx := principal.With(context.Background(),
		principal.Principal{User: "alice", Tool: principal.ToolDataEntry})

	b.ReportAllocs()
	for b.Loop() {
		_ = resolver.FieldVerdicts(pctx, tkt)
		_ = resolver.RelationVerdicts(pctx, tkt)
	}
}
