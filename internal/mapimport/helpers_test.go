package mapimport

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/aclstore"
	"github.com/Sourcehaven-BV/rela/internal/affordances"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/flowcfg/transitions"
	"github.com/Sourcehaven-BV/rela/internal/lock"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
	"github.com/Sourcehaven-BV/rela/internal/templating"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

const testSchema = `
version: "1"
entities:
  control-group:
    label: Control group
    id_prefix: GRP
    properties:
      oscal_id: {type: string, unique: true}
      title: {type: string}
      catalog: {type: string}
  control:
    label: Control
    id_prefix: CTL
    properties:
      oscal_id: {type: string, unique: true}
      title: {type: string}
      label: {type: string}
      catalog: {type: string}
      weight: {type: integer}
      tags: {type: string, list: true}
      note: {type: string}
  risk:
    label: Risk
    id_prefix: RSK
    id_type: manual
    properties:
      ref: {type: external_ref, system: register}
      title: {type: string}
  doc:
    label: Document
    id_type: manual
    properties:
      title: {type: string}
relations:
  part-of:
    label: Part of
    from: [control]
    to: [control-group]
  refines:
    label: Refines
    from: [control]
    to: [control]
  mitigates:
    label: Mitigates
    from: [control]
    to: [risk]
`

func testMeta(t *testing.T) *metamodel.Metamodel {
	t.Helper()
	meta, err := metamodel.Parse([]byte(testSchema))
	if err != nil {
		t.Fatalf("parse schema: %v", err)
	}
	return meta
}

type nopTemplater struct{}

func (nopTemplater) EntityTemplate(context.Context, string, string) (*templating.Template, error) {
	return nil, nil //nolint:nilnil // no template
}

func (nopTemplater) RelationTemplate(context.Context, string) (*templating.Template, error) {
	return nil, nil //nolint:nilnil // no template
}

// mapFiles serves project files from memory.
type mapFiles map[string]string

func (f mapFiles) Load(_ context.Context, name string) ([]byte, error) {
	s, ok := f[name]
	if !ok {
		return nil, fs.ErrNotExist
	}
	return []byte(s), nil
}

// world is a memory store with a manager and a gate that reads every row.
// ctx is the context every call in the test runs under, bound to the
// principal for the ACL variant.
type world struct {
	t     *testing.T
	ctx   context.Context //nolint:containedctx // one test's fixture, not an API
	meta  *metamodel.Metamodel
	st    store.Store
	mgr   *entitymanager.Manager
	gate  ReadGate
	files mapFiles
}

func newWorld(t *testing.T) *world {
	t.Helper()
	meta := testMeta(t)
	st := memstore.New()
	mgr, err := entitymanager.New(entitymanager.Deps{
		Store: st, Meta: meta, Templater: nopTemplater{}, Audit: audit.Nop{}, ACL: acl.NopACL{},
		Transitions: transitions.NoTransitions(), FieldGate: entitymanager.AllowAllFieldGate{},
		AttachmentLocker: lock.NewMemoryLocker(), AnchorGuard: entitymanager.NopAnchorGuard{},
	})
	if err != nil {
		t.Fatalf("entitymanager.New: %v", err)
	}
	gate := visibility.Unrestricted(st).WithWorld(visibility.WorldOf(store.TrivialScope()))
	return &world{
		t: t, ctx: context.Background(), meta: meta, st: st, mgr: mgr, gate: gate,
		files: mapFiles{"scripts/oscal.lua": readTestdata(t, "oscal/oscal.lua")},
	}
}

// aclWorld is newWorld with the manager on an acl.yaml policy, the gate
// reading as the user, and ctx bound to the user as the CLI binds it.
func aclWorld(t *testing.T, policy, user string) *world {
	t.Helper()
	w := newWorld(t)
	p, err := acl.LoadPolicyBytes([]byte(policy))
	if err != nil {
		t.Fatalf("load policy: %v", err)
	}
	d, err := acl.NewDeclarative(p, aclstore.MustNewGraph(w.st), w.st)
	if err != nil {
		t.Fatalf("NewDeclarative: %v", err)
	}
	copyVis, err := entitymanager.NewAllowAllCopyVisibility(w.st)
	if err != nil {
		t.Fatalf("NewAllowAllCopyVisibility: %v", err)
	}
	w.mgr, err = entitymanager.New(entitymanager.Deps{
		Store: w.st, Meta: w.meta, Templater: nopTemplater{}, Audit: audit.Nop{}, ACL: d,
		Transitions: transitions.NoTransitions(), FieldGate: entitymanager.AllowAllFieldGate{},
		CopyReadGate: entitymanager.AllowAllCopyReadGate{}, CopyVisibility: copyVis,
		AttachmentLocker: lock.NewMemoryLocker(), AnchorGuard: entitymanager.NopAnchorGuard{},
	})
	if err != nil {
		t.Fatalf("entitymanager.New: %v", err)
	}
	gate, err := visibility.NewDeclarativeGate(d, store.TrivialScope())
	if err != nil {
		t.Fatalf("NewDeclarativeGate: %v", err)
	}
	resolver, err := affordances.New(w.meta, relLookup{w.st}, d)
	if err != nil {
		t.Fatalf("affordances.New: %v", err)
	}
	redactor, err := visibility.NewPolicyRedactor(resolver)
	if err != nil {
		t.Fatalf("NewPolicyRedactor: %v", err)
	}
	policyReader, err := visibility.NewPolicyReader(gate, redactor, w.st)
	if err != nil {
		t.Fatalf("NewPolicyReader: %v", err)
	}
	sr, err := visibility.NewScriptReader(policyReader, w.st, gate)
	if err != nil {
		t.Fatalf("NewScriptReader: %v", err)
	}
	w.gate = sr.WithWorld(visibility.WorldOf(store.TrivialScope()))
	ctx := principal.With(context.Background(), principal.Principal{User: user, Tool: principal.ToolCLI})
	if w.ctx, err = acl.BindRequest(ctx, d); err != nil {
		t.Fatalf("BindRequest: %v", err)
	}
	return w
}

// relLookup answers the resolver's relation questions from the store.
type relLookup struct{ st store.Store }

func (l relLookup) OutgoingCounts(ctx context.Context, fromID string) map[string]int {
	counts := map[string]int{}
	for r, err := range l.st.ListRelations(ctx, store.RelationQuery{From: fromID, Direction: store.DirectionOutgoing}) {
		if err == nil {
			counts[r.Type]++
		}
	}
	return counts
}

func (l relLookup) HasEdge(ctx context.Context, fromID, relType, toID string) bool {
	for r, err := range l.st.ListRelations(ctx, store.RelationQuery{From: fromID, Direction: store.DirectionOutgoing}) {
		if err == nil && r.Type == relType && r.To == toID {
			return true
		}
	}
	return false
}

func readTestdata(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func (w *world) mapping(src string) *Mapping {
	w.t.Helper()
	m, err := Parse("test", []byte(src), w.meta)
	if err != nil {
		w.t.Fatalf("Parse: %v", err)
	}
	return m
}

func (w *world) plan(m *Mapping, format Format, data string) *Plan {
	w.t.Helper()
	root, err := Decode([]byte(data), format, m.Limits())
	if err != nil {
		w.t.Fatalf("Decode: %v", err)
	}
	p, err := Build(w.ctx, Deps{Meta: w.meta, Store: w.st, Gate: w.gate, Files: w.files}, m, root)
	if err != nil {
		w.t.Fatalf("Build: %v", err)
	}
	return p
}

func (w *world) apply(p *Plan) *Result {
	w.t.Helper()
	res, err := Apply(w.ctx, w.mgr, p, ApplyOptions{})
	if err != nil {
		w.t.Fatalf("Apply: %v", err)
	}
	if len(res.Failures) > 0 {
		w.t.Fatalf("Apply failures: %+v", res.Failures)
	}
	return res
}

// run plans and applies, and fails on record errors.
func (w *world) run(m *Mapping, format Format, data string) *Plan {
	w.t.Helper()
	p := w.plan(m, format, data)
	if len(p.Errors) > 0 {
		w.t.Fatalf("plan errors: %+v", p.Errors)
	}
	w.apply(p)
	return p
}

// create stores an entity and returns its id. id may be "" for a type
// that generates ids.
func (w *world) create(typ, id string, props map[string]any) string {
	w.t.Helper()
	res, err := w.mgr.CreateEntity(w.ctx, &entity.Entity{Type: typ, Properties: props},
		entity.CreateOptions{ID: id, WriteExternalRefs: true})
	if err != nil {
		w.t.Fatalf("create %s: %v", typ, err)
	}
	return res.Entity.ID
}

// byKey returns the stored entity of typ whose oscal_id is value.
func (w *world) byKey(typ, value string) *entity.Entity {
	w.t.Helper()
	for e, err := range w.st.ListEntities(w.ctx, store.EntityQuery{Type: typ, Faces: store.AllFaces()}) {
		if err != nil {
			w.t.Fatal(err)
		}
		if normalize(e.Properties["oscal_id"]) == value {
			return e
		}
	}
	return nil
}

// controls counts the stored controls.
func (w *world) controls() int {
	w.t.Helper()
	n := 0
	for _, err := range w.st.ListEntities(w.ctx, store.EntityQuery{Type: "control", Faces: store.AllFaces()}) {
		if err != nil {
			w.t.Fatal(err)
		}
		n++
	}
	return n
}

func (w *world) relations(typ string) []string {
	w.t.Helper()
	var out []string
	for r, err := range w.st.ListRelations(w.ctx, store.RelationQuery{Type: typ}) {
		if err != nil {
			w.t.Fatal(err)
		}
		out = append(out, r.From+"->"+r.To)
	}
	return out
}

// hideGate reads as inner does, except for the ids in hidden.
type hideGate struct {
	inner  ReadGate
	hidden map[string]bool
}

func (g hideGate) GetAddress(ctx context.Context, addr string) (*entity.Entity, error) {
	id, _, _ := strings.Cut(addr, entity.StateRefSeparator)
	if g.hidden[id] {
		return nil, store.ErrNotFound
	}
	return g.inner.GetAddress(ctx, addr)
}

// failGate fails every read with a non-ErrNotFound error.
type failGate struct{}

func (failGate) GetAddress(context.Context, string) (*entity.Entity, error) {
	return nil, errors.New("gate down")
}
