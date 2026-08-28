package appbuild_test

import (
	"context"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/appbuild/appbuildtest"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// TKT-K2VN9D: end-to-end proof through the SCHEDULER + Lua path — the exact
// path the motivating outage ran on. The internal/acl tests pin the decision;
// this pins that a real `rela.create_relation` call reaches it and is allowed
// on the strength of relation_grants alone.
//
// Without this, the feature could be correct in the resolver and still never
// reach it from the path that mattered.

func spawntMeta() *metamodel.Metamodel {
	str := metamodel.PropertyDef{Type: metamodel.PropertyTypeString}
	return &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"terugkerend": {
				IDPrefix:   "TERUG",
				Properties: map[string]metamodel.PropertyDef{"title": str},
			},
			"taak": {
				IDPrefix:   "TAAK",
				Properties: map[string]metamodel.PropertyDef{"title": str},
			},
		},
		Relations: map[string]metamodel.RelationDef{
			"spawnt": {From: []string{"terugkerend"}, To: []string{"taak"}},
		},
	}
}

// The outage policy, plus the grant that makes it expressible. Note what is
// NOT here: `create: [terugkerend]`. Before relation_grants: that was the only
// way to let the scheduler write the edge.
const spawntPolicy = `
roles:
  scheduler-system:
    read: ["*"]
    create: [taak]
    update: [taak, terugkerend]
    permissions: [create-spawnt]
assignments:
  system:scheduler: scheduler-system
relation_grants:
  spawnt:
    create: create-spawnt
`

// spawntHarness is the scheduler wiring under test: a seeded store plus a
// runner that executes Lua as `system:scheduler`, exactly as the scheduler does.
type spawntHarness struct {
	store *memstore.MemStore
	run   func(script string) (string, error)
	close func()
}

func newSpawntHarness(t *testing.T, policyBody string) spawntHarness {
	t.Helper()
	ctx := context.Background()
	st := memstore.New()
	for _, e := range []*entity.Entity{
		{ID: "TERUG-1", Type: "terugkerend", Properties: map[string]any{"title": "Weekly"}},
		{ID: "TAAK-1", Type: "taak", Properties: map[string]any{"title": "Existing"}},
	} {
		if err := st.CreateEntity(ctx, e); err != nil {
			t.Fatalf("seed %s: %v", e.ID, err)
		}
	}
	p, err := acl.LoadPolicyBytes([]byte(policyBody))
	if err != nil {
		t.Fatalf("load policy: %v", err)
	}
	d, err := acl.NewDeclarative(p, acl.NewStoreGraph(st), st)
	if err != nil {
		t.Fatalf("NewDeclarative: %v", err)
	}
	svc := appbuildtest.New(spawntMeta(), appbuildtest.WithStore(st), appbuildtest.WithDeclarative(d))

	run := func(script string) (string, error) {
		taskCtx := principal.With(ctx, principal.Principal{
			User: "system:scheduler", Tool: principal.ToolScheduler,
		})
		var out strings.Builder
		rt := lua.NewWriter(svc.ScheduledLuaWriteDeps(), &out,
			lua.WithContext(taskCtx), lua.WithPrincipal(principal.From(taskCtx)))
		defer rt.Close()
		//nolint:contextcheck // the ctx is supplied via lua.WithContext above;
		// RunString takes no ctx parameter.
		err := rt.RunString(script)
		return out.String(), err
	}
	return spawntHarness{store: st, run: run, close: func() { _ = svc.Close() }}
}

// TestRelationGrants_SchedulerCanWriteEdgeWithoutEntityCreate replays the
// outage: create a taak, then relate it back to its source. Before this
// feature the second call failed with "no role grants create on relations from
// type \"terugkerend\"" while the first had already committed — which is how
// one permissions typo became unbounded duplicate growth.
func TestRelationGrants_SchedulerCanWriteEdgeWithoutEntityCreate(t *testing.T) {
	h := newSpawntHarness(t, spawntPolicy)
	defer h.close()

	out, err := h.run(`
local taak = rela.create_entity("taak", {title = "Spawned"})
rela.create_relation("TERUG-1", "spawnt", taak.id)
rela.output("created=" .. taak.id)
`)
	if err != nil {
		t.Fatalf("scheduler script failed: %v (output %q)", err, out)
	}

	var found int
	for r, iterErr := range h.store.ListRelations(context.Background(),
		store.RelationQuery{From: "TERUG-1", Type: "spawnt"}) {
		if iterErr != nil {
			t.Fatalf("ListRelations: %v", iterErr)
		}
		_ = r
		found++
	}
	if found != 1 {
		t.Fatalf("spawnt edges = %d, want 1 — the edge the dedup guard keys on", found)
	}
}

// TestRelationGrants_SchedulerStillCannotCreateSourceEntities is the
// least-privilege half. If this fails the feature bought nothing: the operator
// could have written `create: [terugkerend]` and had the same authority.
func TestRelationGrants_SchedulerStillCannotCreateSourceEntities(t *testing.T) {
	h := newSpawntHarness(t, spawntPolicy)
	defer h.close()

	out, err := h.run(`rela.create_entity("terugkerend", {title = "Sneaky"})`)
	if err == nil {
		t.Fatalf("scheduler created a terugkerend entity; relation_grants must not "+
			"confer entity-create on the source type (output %q)", out)
	}
	if !strings.Contains(err.Error(), "forbidden") {
		t.Errorf("error %q does not look like an ACL denial", err)
	}
}

// TestRelationGrants_WithoutTheGrantTheWriteIsStillDenied is the negative
// control: same policy minus relation_grants must reproduce the ORIGINAL
// failure. Without it, the two tests above would pass equally against a build
// that authorized every relation write.
func TestRelationGrants_WithoutTheGrantTheWriteIsStillDenied(t *testing.T) {
	const noGrant = `
roles:
  scheduler-system:
    read: ["*"]
    create: [taak]
    update: [taak, terugkerend]
    permissions: [create-spawnt]
assignments:
  system:scheduler: scheduler-system
`
	h := newSpawntHarness(t, noGrant)
	defer h.close()

	out, err := h.run(`rela.create_relation("TERUG-1", "spawnt", "TAAK-1")`)
	if err == nil {
		t.Fatalf("relation write allowed with no relation_grants entry and no "+
			"create grant on the source type (output %q)", out)
	}
	if !strings.Contains(err.Error(), "relations from type") {
		t.Errorf("error %q is not the source-type denial", err)
	}
}
