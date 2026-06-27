package acl_test

import (
	"context"
	"sort"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// World is a feature-test fixture: a memstore, a parsed acl.Policy, an
// acl.Declarative, and the helpers needed to make assertions against
// them. Built fluently — see TestFeature_UC* in features_test.go for
// canonical usage.
//
// World is single-goroutine by construction. It holds a *testing.T
// after Build so assertion helpers can call t.Errorf without each
// caller threading t.
//
// background that flows through every assertion. Threading ctx through
// every helper signature here would add ceremony without buying isolation.
//
//nolint:containedctx // test fixture: the ctx is a request-scoped
type World struct {
	t   *testing.T
	ctx context.Context

	policyYAML string
	store      store.Store
	acl        *acl.Declarative

	// Pending mutations applied at Build time. Held as data so the
	// builder is order-independent (you can declare a relation before
	// the entities it points at).
	entities  []pendingEntity
	relations []pendingRelation
}

type pendingEntity struct {
	id, typ string
}

type pendingRelation struct {
	from, typ, to string
}

// EntityOpt is a vararg option to Folder / Document / etc. Currently
// only Inside (containment) — the shape lets us grow opts without
// renaming callers.
type EntityOpt func(*pendingEntity, *World)

// Inside(parent) emits a belongs-to relation from the entity being
// built to `parent`. The parent does not need to exist at the call
// site — relations are resolved at Build time.
func Inside(parent string) EntityOpt {
	return func(e *pendingEntity, w *World) {
		w.relations = append(w.relations, pendingRelation{
			from: e.id, typ: "belongs-to", to: parent,
		})
	}
}

// NewWorld constructs an empty World. Chain Policy / Entity / Relation
// calls, then Build(t) to materialize.
func NewWorld() *World {
	return &World{ctx: context.Background()}
}

// Policy sets the acl.yaml content for this world. Must be called
// before Build; the policy is parsed during Build.
func (w *World) Policy(yaml string) *World {
	w.policyYAML = yaml
	return w
}

// Entity declares an entity of the given type and id, with optional
// opts (Inside, WithProperty).
func (w *World) Entity(id, typ string, opts ...EntityOpt) *World {
	e := pendingEntity{id: id, typ: typ}
	for _, opt := range opts {
		opt(&e, w)
	}
	w.entities = append(w.entities, e)
	return w
}

// Folder is shorthand for Entity(id, "folder", opts...).
func (w *World) Folder(id string, opts ...EntityOpt) *World {
	return w.Entity(id, "folder", opts...)
}

// Document is shorthand for Entity(id, "document", opts...).
func (w *World) Document(id string, opts ...EntityOpt) *World {
	return w.Entity(id, "document", opts...)
}

// Person is shorthand for Entity(id, "person").
func (w *World) Person(id string) *World {
	return w.Entity(id, "person")
}

// Team is shorthand for Entity(id, "team").
func (w *World) Team(id string) *World {
	return w.Entity(id, "team")
}

// Ticket is shorthand for Entity(id, "ticket").
func (w *World) Ticket(id string) *World {
	return w.Entity(id, "ticket")
}

// Project is shorthand for Entity(id, "project").
func (w *World) Project(id string) *World {
	return w.Entity(id, "project")
}

// Relation declares an arbitrary relation. Use the more specific
// builders (Inside, etc.) where they fit; Relation is the escape hatch.
func (w *World) Relation(from, typ, to string) *World {
	w.relations = append(w.relations, pendingRelation{
		from: from, typ: typ, to: to,
	})
	return w
}

// Build materializes the world: parses the policy, sets up an
// in-memory store, writes the entities and relations, and constructs
// the acl.Declarative. From this point on the World can be queried
// via Allow / Visible / Attribution and their Assert* sugar.
//
// Build fatals the test on setup error — a malformed policy or a
// relation pointing at an entity that doesn't exist indicates a bug
// in the test fixture, not behavior the test is meant to assert on.
func (w *World) Build(t *testing.T) *World {
	t.Helper()
	w.t = t

	policy, err := acl.LoadPolicyBytes([]byte(w.policyYAML))
	if err != nil {
		t.Fatalf("World.Build: load policy: %v", err)
	}

	ms := memstore.New()
	for _, e := range w.entities {
		if cErr := ms.CreateEntity(w.ctx, entity.New(e.id, e.typ)); cErr != nil {
			t.Fatalf("World.Build: create entity %s: %v", e.id, cErr)
		}
	}
	for _, r := range w.relations {
		if _, cErr := ms.CreateRelation(w.ctx, r.from, r.typ, r.to, nil); cErr != nil {
			t.Fatalf("World.Build: create relation %s --%s--> %s: %v", r.from, r.typ, r.to, cErr)
		}
	}
	w.store = ms

	d, err := acl.NewDeclarative(policy, acl.NewStoreGraph(ms), ms)
	if err != nil {
		t.Fatalf("World.Build: NewDeclarative: %v", err)
	}
	w.acl = d
	return w
}

// principal returns the canonical Principal for `actor`. All feature
// tests use Tool=data-entry; specific edge cases (e.g. CLI vs MCP) live
// in unit tests, not here.
func (*World) principalFor(actor string) principal.Principal {
	return principal.Principal{User: actor, Tool: principal.ToolDataEntry}
}

// requestFor opens a per-principal Request. Errors fatal the test —
// a malformed principal at the feature-test layer is a bug in the test,
// not behavior the test is asserting on. (Unknown-principal errors are
// covered by unit tests.)
func (w *World) requestFor(actor string) *acl.Request {
	w.t.Helper()
	req, err := w.acl.ForPrincipal(w.principalFor(actor))
	if err != nil {
		w.t.Fatalf("World.requestFor(%q): %v", actor, err)
	}
	return req
}

// ---- Primitives ---------------------------------------------------------

// Allow reports whether `actor` may perform `op` on `subject`.
func (w *World) Allow(actor string, op acl.Op, subject acl.Subject) bool {
	w.t.Helper()
	req := w.requestFor(actor)
	d := req.AuthorizeWrite(w.ctx, acl.WriteRequest{Op: op, Subject: subject})
	return d.Allow
}

// Visible returns the set of entity IDs of `entityType` that `actor`
// can read. Sorted, so equality comparisons are stable.
func (w *World) Visible(actor, entityType string) []string {
	w.t.Helper()
	req := w.requestFor(actor)
	rqr := req.ReadQuery(w.ctx, entityType)

	var ids []string
	switch {
	case rqr.AllowAll:
		for _, e := range listAllOfType(w.ctx, w.t, w.store, entityType) {
			ids = append(ids, e.ID)
		}
	case rqr.DenyAll:
		// nothing
	default:
		// RR-3D6Q: a zero ReadQueryResult (no AllowAll, no DenyAll,
		// nil Query) is a readQuery bug — without this check the next
		// line dereferences a nil pointer and the test panics with a
		// misleading "fixture panicked" rather than "the resolver
		// returned a malformed result." Fail loud at the boundary.
		if rqr.Query == nil {
			w.t.Fatalf("Visible(%q, %q): ReadQuery returned neither AllowAll, DenyAll, nor a Query — readQuery bug",
				actor, entityType)
		}
		for e, err := range w.store.GraphQuery(w.ctx, *rqr.Query) {
			if err != nil {
				w.t.Fatalf("Visible(%q, %q): GraphQuery: %v", actor, entityType, err)
			}
			ids = append(ids, e.ID)
		}
	}
	sort.Strings(ids)
	return ids
}

// CanSee reports whether `actor` is permitted to read entity
// (entityType, entityID) per Request.PermitsRead. Used by the
// PermitsRead-helper feature tests; mirrors the per-entity GET gate
// in dataentry.
func (w *World) CanSee(actor, entityType, entityID string) bool {
	w.t.Helper()
	req := w.requestFor(actor)
	ok, err := req.PermitsRead(w.ctx, entityType, entityID)
	if err != nil {
		w.t.Fatalf("CanSee(%q, %q, %q): PermitsRead: %v", actor, entityType, entityID, err)
	}
	return ok
}

// assertExists fails the test if any of `ids` is missing from the
// store. Used by AssertHidden / AssertContains (RR-2XZW): a typo
// in a test ticket ID would otherwise pass trivially —
// AssertHidden("alice", "ticket", "TKT-TYPO") returns success because
// the bogus ID isn't in any visible set, masking a real assertion.
func (w *World) assertExists(helper, entityType string, ids []string) {
	w.t.Helper()
	for _, id := range ids {
		e, err := w.store.GetEntity(w.ctx, id)
		if err != nil || e == nil {
			w.t.Fatalf("%s: entity %q does not exist in the store (likely a typo in the test ID)", helper, id)
		}
		if entityType != "" && e.Type != entityType {
			w.t.Fatalf("%s: entity %q is of type %q, not %q (test scenario mismatch)",
				helper, id, e.Type, entityType)
		}
	}
}

// Attribution returns the set of Source values that confer `role` on
// `actor` against `entity`. Empty if the role is not in the principal's
// effective set for that entity.
func (w *World) Attribution(actor, entityID, role string) []acl.Source {
	w.t.Helper()
	req := w.requestFor(actor)
	e, err := w.store.GetEntity(w.ctx, entityID)
	if err != nil {
		w.t.Fatalf("Attribution(%q, %q): GetEntity: %v", actor, entityID, err)
	}
	attrs := req.ForEntity(w.ctx, e.Type, e.ID)

	var sources []acl.Source
	for _, a := range attrs {
		if a.Role == role {
			sources = append(sources, a.Source)
		}
	}
	return sources
}

// ---- Assertion sugar ----------------------------------------------------

// AssertAllow fails the test if `actor` cannot perform `op` on `subject`.
func (w *World) AssertAllow(actor string, op acl.Op, subject acl.Subject) {
	w.t.Helper()
	if !w.Allow(actor, op, subject) {
		w.t.Errorf("AssertAllow(%q, %v, %+v): denied, want allowed", actor, op, subject)
	}
}

// AssertDeny fails the test if `actor` can perform `op` on `subject`.
func (w *World) AssertDeny(actor string, op acl.Op, subject acl.Subject) {
	w.t.Helper()
	if w.Allow(actor, op, subject) {
		w.t.Errorf("AssertDeny(%q, %v, %+v): allowed, want denied", actor, op, subject)
	}
}

// AssertVisible fails if `actor`'s visible set of `entityType` is not
// exactly the given ids (any order; the helper sorts both sides).
func (w *World) AssertVisible(actor, entityType string, want ...string) {
	w.t.Helper()
	got := w.Visible(actor, entityType)
	sort.Strings(want)
	if !slicesEqual(got, want) {
		w.t.Errorf("AssertVisible(%q, %q):\n  got:  %v\n  want: %v", actor, entityType, got, want)
	}
}

// AssertContains fails if any of `ids` is NOT in `actor`'s visible
// set of `entityType`. Allows extras — the looser shape for cases
// where you only care that specific entities appear. Every id must
// exist in the store (RR-2XZW: pre-check guards against typos that
// would silently pass).
func (w *World) AssertContains(actor, entityType string, ids ...string) {
	w.t.Helper()
	w.assertExists("AssertContains", entityType, ids)
	got := w.Visible(actor, entityType)
	gotSet := make(map[string]bool, len(got))
	for _, id := range got {
		gotSet[id] = true
	}
	var missing []string
	for _, id := range ids {
		if !gotSet[id] {
			missing = append(missing, id)
		}
	}
	if len(missing) > 0 {
		w.t.Errorf("AssertContains(%q, %q): missing %v from %v", actor, entityType, missing, got)
	}
}

// AssertHidden fails if any of `ids` IS in `actor`'s visible set of
// `entityType`. Allows extras (other entities that are also visible).
// Every id must exist in the store (RR-2XZW: pre-check guards
// against typos that would silently pass — a bogus id is by
// definition "not visible" without the resolver doing anything).
func (w *World) AssertHidden(actor, entityType string, ids ...string) {
	w.t.Helper()
	w.assertExists("AssertHidden", entityType, ids)
	got := w.Visible(actor, entityType)
	gotSet := make(map[string]bool, len(got))
	for _, id := range got {
		gotSet[id] = true
	}
	var leaked []string
	for _, id := range ids {
		if gotSet[id] {
			leaked = append(leaked, id)
		}
	}
	if len(leaked) > 0 {
		w.t.Errorf("AssertHidden(%q, %q): %v unexpectedly visible (full set: %v)", actor, entityType, leaked, got)
	}
}

// AssertPrimarySource fails if the primary source for (actor, entity,
// role) doesn't match `want`. Primary is the AC8a-sorted first
// attribution.
func (w *World) AssertPrimarySource(actor, entityID, role string, want acl.Source) {
	w.t.Helper()
	srcs := w.Attribution(actor, entityID, role)
	if len(srcs) == 0 {
		w.t.Errorf("AssertPrimarySource(%q, %q, %q): no sources confer this role", actor, entityID, role)
		return
	}
	primary := acl.PrimarySource(srcs)
	if primary != want {
		w.t.Errorf("AssertPrimarySource(%q, %q, %q):\n  got:  %s\n  want: %s",
			actor, entityID, role, primary, want)
	}
}

// AssertAttribution fails if the set of Sources conferring `role` on
// `actor` for `entity` is not exactly `want` (any order; equality by
// String() so the comparison reads naturally on failure).
func (w *World) AssertAttribution(actor, entityID, role string, want ...acl.Source) {
	w.t.Helper()
	got := w.Attribution(actor, entityID, role)

	gotStrs := make([]string, len(got))
	for i, s := range got {
		gotStrs[i] = s.String()
	}
	wantStrs := make([]string, len(want))
	for i, s := range want {
		wantStrs[i] = s.String()
	}
	sort.Strings(gotStrs)
	sort.Strings(wantStrs)

	if !slicesEqual(gotStrs, wantStrs) {
		w.t.Errorf("AssertAttribution(%q, %q, %q):\n  got:  %v\n  want: %v",
			actor, entityID, role, gotStrs, wantStrs)
	}
}

// ---- Internal helpers ---------------------------------------------------

func listAllOfType(ctx context.Context, t *testing.T, s store.Store, typ string) []*entity.Entity {
	t.Helper()
	var out []*entity.Entity
	for e, err := range s.ListEntities(ctx, store.EntityQuery{Type: typ}) {
		if err != nil {
			t.Fatalf("listAllOfType(%q): %v", typ, err)
		}
		out = append(out, e)
	}
	return out
}

func slicesEqual(a, b []string) bool {
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
