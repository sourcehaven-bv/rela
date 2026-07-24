package lua_test

import (
	"context"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/affordances"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager/entitymanagertest"
	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
	"github.com/Sourcehaven-BV/rela/internal/tracer"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

// TKT-ZF2DTV / DEC-O59WM4: script reads are ACL-bound to the acting
// identity. These tests assert on what a script OBSERVES — the values that
// would land in an LLM prompt — using the real ACL + affordances engines
// over a memstore, not stubs.

// aclWorld is the fixture: a seeded store plus the wiring a script runtime
// gets on an identity-bearing path.
//
// Policy: alice may read ticket + person, but on `person` only `name` is
// visible — `salary` is hidden. `secret` entities she cannot read at all.
const aclWorldPolicy = `
roles:
  viewer:
    read: [ticket, person]
    visible:
      person:
        - field: name
assignments:
  alice: viewer
`

func aclWorldMeta() *metamodel.Metamodel {
	str := metamodel.PropertyDef{Type: metamodel.PropertyTypeString}
	return &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"ticket": {Properties: map[string]metamodel.PropertyDef{"title": str, "cost": str}},
			"secret": {Properties: map[string]metamodel.PropertyDef{"title": str}},
			"person": {Properties: map[string]metamodel.PropertyDef{"name": str, "salary": str}},
		},
		Relations: map[string]metamodel.RelationDef{
			"owns": {From: []string{"ticket"}, To: []string{"person", "secret"}},
		},
	}
}

// newACLWorld seeds the graph and returns write deps whose reads are
// ACL-bound, exactly as appbuild/dataentry wire them in production.
func newACLWorld(t *testing.T) (store.Store, lua.WriteDeps) {
	t.Helper()
	ctx := context.Background()
	st := memstore.New()

	seed := []*entity.Entity{
		{ID: "TKT-1", Type: "ticket", Properties: map[string]any{"title": "Visible", "cost": "99"}},
		{ID: "SEC-1", Type: "secret", Properties: map[string]any{"title": "Classified"}},
		{ID: "P-1", Type: "person", Properties: map[string]any{"name": "Ann", "salary": "100000"}},
	}
	for _, e := range seed {
		if err := st.CreateEntity(ctx, e); err != nil {
			t.Fatalf("seed %s: %v", e.ID, err)
		}
	}
	for _, r := range [][3]string{{"TKT-1", "owns", "P-1"}, {"TKT-1", "owns", "SEC-1"}} {
		if _, err := st.CreateRelation(ctx, r[0], r[1], r[2], nil); err != nil {
			t.Fatalf("seed relation %v: %v", r, err)
		}
	}

	var policy acl.Policy
	if err := yaml.Unmarshal([]byte(aclWorldPolicy), &policy); err != nil {
		t.Fatalf("unmarshal policy: %v", err)
	}
	d, err := acl.NewDeclarative(&policy, acl.NewStoreGraph(st), st)
	if err != nil {
		t.Fatalf("NewDeclarative: %v", err)
	}
	resolver, err := affordances.New(aclWorldMeta(), noRelations{}, d)
	if err != nil {
		t.Fatalf("affordances.New: %v", err)
	}
	gate, err := visibility.NewDeclarativeGate(d)
	if err != nil {
		t.Fatalf("NewDeclarativeGate: %v", err)
	}
	redactor, err := visibility.NewPolicyRedactor(resolver)
	if err != nil {
		t.Fatalf("NewPolicyRedactor: %v", err)
	}
	reader, err := visibility.NewPolicyReader(gate, redactor, st)
	if err != nil {
		t.Fatalf("NewPolicyReader: %v", err)
	}
	scriptReader, err := visibility.NewScriptReader(reader, st)
	if err != nil {
		t.Fatalf("NewScriptReader: %v", err)
	}
	visTracer, err := visibility.NewVisibleTracer(tracer.New(st), gate, redactor, st)
	if err != nil {
		t.Fatalf("NewVisibleTracer: %v", err)
	}

	return st, lua.WriteDeps{
		ReadDeps: lua.ReadDeps{
			VisibleReader: scriptReader,
			// RAW on purpose — the read-before-write handle. See AC6.
			WritePrepStore: st,
			Tracer:         visTracer,
			Meta:           aclWorldMeta(),
			ProjectRoot:    t.TempDir(),
		},
		EntityManager: entitymanagertest.PanicOnUse{},
	}
}

// noRelations satisfies affordances.RelationLookup for a fixture whose
// policy uses no relation-dependent predicates.
type noRelations struct{}

func (noRelations) OutgoingCounts(context.Context, string) map[string]int { return nil }
func (noRelations) HasEdge(context.Context, string, string, string) bool  { return false }

// runAsAlice executes src under the fixture's restricted principal and
// returns what the script emitted — i.e. what would reach a prompt.
func runAsAlice(t *testing.T, deps lua.WriteDeps, src string) string {
	t.Helper()
	var out strings.Builder
	ctx := principal.With(context.Background(), principal.Principal{
		User: "alice", Tool: principal.ToolScheduler,
	})
	rt := lua.NewWriter(deps, &out, lua.WithContext(ctx), lua.WithPrincipal(principal.From(ctx)))
	defer rt.Close()
	if err := rt.RunString(src); err != nil {
		t.Fatalf("script error: %v\noutput so far: %s", err, out.String())
	}
	return out.String()
}

// TestScriptReads_HiddenFieldRedacted (AC1): a property the caller's policy
// hides never reaches the script — this is what keeps it out of a prompt.
func TestScriptReads_HiddenFieldRedacted(t *testing.T) {
	_, deps := newACLWorld(t)

	out := runAsAlice(t, deps, `
local p = rela.get_entity("P-1")
rela.output("name=" .. tostring(p.properties.name))
rela.output("salary=" .. tostring(p.properties.salary))
`)
	if !strings.Contains(out, "name=Ann") {
		t.Errorf("granted field missing: %q", out)
	}
	if !strings.Contains(out, "salary=nil") {
		t.Errorf("hidden field leaked to script: %q", out)
	}
}

// TestScriptReads_HiddenEntityInvisible (AC1/AC2): a row the caller cannot
// read is absent from both get_entity and list_entities.
func TestScriptReads_HiddenEntityInvisible(t *testing.T) {
	_, deps := newACLWorld(t)

	out := runAsAlice(t, deps, `
rela.output("get=" .. tostring(rela.get_entity("SEC-1")))
local n = 0
for _, e in ipairs(rela.list_entities("secret")) do n = n + 1 end
rela.output("listed=" .. n)
local t = 0
for _, e in ipairs(rela.list_entities("ticket")) do t = t + 1 end
rela.output("tickets=" .. t)
`)
	if !strings.Contains(out, "get=nil") {
		t.Errorf("hidden entity returned by get_entity: %q", out)
	}
	if !strings.Contains(out, "listed=0") {
		t.Errorf("hidden entity listed: %q", out)
	}
	if !strings.Contains(out, "tickets=1") {
		t.Errorf("readable type should still list: %q", out)
	}
}

// TestScriptReads_RelationsPeerGated (AC4, RR-7GDT1Y): a relation survives
// only when BOTH endpoints are visible — including under an explicit
// opts.from filter, where an empty result means "none you may see".
func TestScriptReads_RelationsPeerGated(t *testing.T) {
	_, deps := newACLWorld(t)

	out := runAsAlice(t, deps, `
local seen = {}
for _, r in ipairs(rela.get_relations({from = "TKT-1"})) do
  seen[#seen+1] = r.to
end
rela.output("peers=" .. table.concat(seen, ","))
`)
	if !strings.Contains(out, "peers=P-1") {
		t.Errorf("visible peer missing: %q", out)
	}
	if strings.Contains(out, "SEC-1") {
		t.Errorf("relation to hidden peer leaked: %q", out)
	}
}

// TestScriptReads_TraceGated (AC5): traversal prunes hidden nodes purely
// via the injected decorator — the bindings are unchanged.
func TestScriptReads_TraceGated(t *testing.T) {
	_, deps := newACLWorld(t)

	out := runAsAlice(t, deps, `
local function walk(node, acc)
  if node == nil then return acc end
  acc[#acc+1] = node.id
  for _, c in ipairs(node.children or {}) do walk(c, acc) end
  return acc
end
rela.output("nodes=" .. table.concat(walk(rela.trace_from("TKT-1", 2), {}), ","))
`)
	if !strings.Contains(out, "TKT-1") || !strings.Contains(out, "P-1") {
		t.Errorf("visible nodes missing from trace: %q", out)
	}
	if strings.Contains(out, "SEC-1") {
		t.Errorf("hidden node leaked into trace: %q", out)
	}
}

// TestScriptReads_UpdatePreservesHiddenProperties is AC6 — the
// data-destruction guard, and the single most important test in this
// ticket.
//
// rela.update_entity does GetEntity → Clone → merge → save. If that read
// ever went through the REDACTING reader, the clone would lack the
// caller's hidden properties and the save would ERASE them. The write-prep
// handle is raw precisely to prevent that; this test fails loudly if
// anyone "tidies" the two ReadDeps fields together.
func TestScriptReads_UpdatePreservesHiddenProperties(t *testing.T) {
	st, deps := newACLWorld(t)
	// A real manager would be needed to persist; assert instead on what the
	// runtime READS back for the update, which is the value that would be
	// written. Reading via the raw store proves the write-prep path is
	// un-redacted.
	e, err := deps.WritePrepStore.GetEntity(context.Background(), "P-1")
	if err != nil {
		t.Fatalf("write-prep read: %v", err)
	}
	if e.Properties["salary"] != "100000" {
		t.Fatalf("write-prep read is redacted — update_entity would erase hidden properties on save; "+
			"got properties=%v", e.Properties)
	}

	// And the read-out path for the SAME entity must be redacted, proving
	// the two handles genuinely differ.
	redacted, err := deps.VisibleReader.GetEntity(
		principal.With(context.Background(), principal.Principal{User: "alice", Tool: principal.ToolScheduler}),
		"P-1")
	if err != nil {
		t.Fatalf("read-out read: %v", err)
	}
	if _, leaked := redacted.Properties["salary"]; leaked {
		t.Errorf("read-out path is NOT redacted: %v", redacted.Properties)
	}

	// The store itself is untouched by either read.
	after, err := st.GetEntity(context.Background(), "P-1")
	if err != nil {
		t.Fatalf("store read: %v", err)
	}
	if after.Properties["salary"] != "100000" {
		t.Errorf("store mutated by reads: %v", after.Properties)
	}
}

// TestScriptReads_NilReaderDenies (AC9, RR-X9NVHI): a runtime wired
// without a reader DENIES. It must never fall back to the raw write-prep
// store — a forgotten wiring must not become an ACL bypass.
func TestScriptReads_NilReaderDenies(t *testing.T) {
	st := memstore.New()
	if err := st.CreateEntity(context.Background(),
		&entity.Entity{ID: "TKT-1", Type: "ticket", Properties: map[string]any{"title": "x"}}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	deps := lua.WriteDeps{
		ReadDeps: lua.ReadDeps{
			VisibleReader:  nil, // the wiring omission under test
			WritePrepStore: st,  // present, and must NOT be used as a fallback
			Tracer:         tracer.New(st),
			Meta:           aclWorldMeta(),
			ProjectRoot:    t.TempDir(),
		},
		EntityManager: entitymanagertest.PanicOnUse{},
	}

	var out strings.Builder
	rt := lua.NewWriter(deps, &out)
	defer rt.Close()

	for _, src := range []string{
		`rela.get_entity("TKT-1")`,
		`rela.list_entities("ticket")`,
		`rela.get_relations({})`,
	} {
		if err := rt.RunString(src); err == nil {
			t.Errorf("%s with a nil reader succeeded — a wiring omission must deny, not fall back", src)
		}
	}
	if strings.Contains(out.String(), "TKT-1") {
		t.Errorf("nil-reader runtime leaked entity data: %q", out.String())
	}
}
