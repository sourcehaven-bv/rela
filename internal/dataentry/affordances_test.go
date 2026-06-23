package dataentry

import (
	"context"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// TestTranslateVerb_Roundtrip pins the phase-1 verb vocabulary against
// acl.Op constants. The grep test (AC10) only proves no one *else*
// constructs WriteRequest{Op:...}; this proves the central translation
// is correct.
func TestTranslateVerb_Roundtrip(t *testing.T) {
	cases := []struct {
		verb string
		op   acl.Op
	}{
		{"create", acl.OpCreate},
		{"update", acl.OpUpdate},
		{"delete", acl.OpDelete},
		{"rename", acl.OpRename},
	}
	for _, c := range cases {
		t.Run(c.verb, func(t *testing.T) {
			req := translateVerb(c.verb, "ticket", "TKT-001")
			if req.Op != c.op {
				t.Errorf("Op = %q, want %q", req.Op, c.op)
			}
			s, ok := req.Subject.(acl.EntitySubject)
			if !ok {
				t.Errorf("Subject = %T, want acl.EntitySubject", req.Subject)
			}
			if s.Type != "ticket" || s.ID != "TKT-001" {
				t.Errorf("Subject = %+v, want {Type:ticket, ID:TKT-001}", s)
			}
		})
	}
}

// TestTranslateVerb_UnknownPanics asserts the "unreachable for the
// closed set" contract. If a future change adds a verb to
// [perItemVerbs] / [perCollectionVerbs] without adding the matching
// translateVerb case, this is the test that fails loudly instead of
// the production deserializer silently returning the zero WriteRequest
// (which would map every misspelled verb to OpCreate).
func TestTranslateVerb_UnknownPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic on unknown verb; got none")
		}
	}()
	translateVerb("transition:done", "ticket", "")
}

// AC1: read-only principal sees all per-item verbs as false.
func TestComputeActions_ReadOnly(t *testing.T) {
	app := newTestAppV1(t)
	app.acl = acl.ReadOnlyACL{}

	e := &entity.Entity{ID: "TKT-001", Type: "ticket"}
	got := app.computeActions(t.Context(), e)

	for _, v := range []string{"update", "delete", "rename"} {
		if got[v] {
			t.Errorf("_actions[%q] = true under ReadOnlyACL, want false", v)
		}
	}
}

// AC2: NopACL principal sees all per-item verbs as true.
func TestComputeActions_NopACL(t *testing.T) {
	app := newTestAppV1(t)
	// app.acl is already acl.NopACL via the test fixture wiring.

	e := &entity.Entity{ID: "TKT-001", Type: "ticket"}
	got := app.computeActions(t.Context(), e)

	for _, v := range []string{"update", "delete", "rename"} {
		if !got[v] {
			t.Errorf("_actions[%q] = false under NopACL, want true", v)
		}
	}
}

// AC4: collection-scope verb computed under ReadOnlyACL is false.
func TestComputeCollectionActions_ReadOnly(t *testing.T) {
	app := newTestAppV1(t)
	app.acl = acl.ReadOnlyACL{}

	got := app.computeCollectionActions(t.Context(), "ticket")
	if got["create"] {
		t.Errorf("_actions.create = true under ReadOnlyACL, want false")
	}
}

// TestComputeActions_MixedTypeDeclarative is AC12 (TKT-LFT2). A
// Declarative policy grants write on `ticket` but not on `feature`;
// `computeActions` should return all-true for the ticket and
// all-false for the feature, demonstrating cross-type variance.
// (Per-row within-type variance is gated on TKT-XZEY since ACL v0's
// WriteRequest carries no entity ID.)
func TestComputeActions_MixedTypeDeclarative(t *testing.T) {
	app := newTestAppV1(t)
	d, err := acl.NewDeclarative(&acl.Policy{
		UserEntityType: "person",
		Roles: map[string]acl.RoleDef{
			"ticket-writer": {Create: []string{"ticket"}, Update: []string{"ticket"}, Delete: []string{"ticket"}},
		},
		Assignments: map[string]string{
			"test-user": "ticket-writer",
		},
	}, acl.NullGraph{}, acl.NullGraphQueryer{})
	if err != nil {
		t.Fatalf("acl.NewDeclarative: %v", err)
	}
	app.acl = d

	// Declarative looks up the principal by Principal.User against
	// Assignments. Stamp a matching User on ctx.
	ctx := principal.With(t.Context(), principal.Principal{
		User: "test-user",
		Tool: principal.ToolDataEntry,
	})

	ticket := &entity.Entity{ID: "TKT-001", Type: "ticket"}
	got := app.computeActions(ctx, ticket)
	for _, v := range []string{"update", "delete", "rename"} {
		if !got[v] {
			t.Errorf("ticket _actions[%q] = false under ticket-writer role, want true", v)
		}
	}

	feature := &entity.Entity{ID: "FEAT-001", Type: "feature"}
	got = app.computeActions(ctx, feature)
	for _, v := range []string{"update", "delete", "rename"} {
		if got[v] {
			t.Errorf("feature _actions[%q] = true under ticket-writer role, want false", v)
		}
	}
}

// TestComputeActions_NoAuditNoise is AC8 — read-time `AuthorizeWrite`
// calls in computeActions are not writes, so they must not produce
// audit records. The audit sink lives on entitymanager.Manager (the
// write path); the read path doesn't touch it. We wire a Memory audit
// sink into the EntityManager and assert that a GET (which triggers
// per-entity + per-collection affordance computation) records zero
// audit entries.
func TestComputeActions_NoAuditNoise(t *testing.T) {
	cases := []acl.ACL{acl.NopACL{}, acl.ReadOnlyACL{}}
	for _, a := range cases {
		t.Run("", func(t *testing.T) {
			sink := audit.NewMemory()
			app := buildAppWithACLAndAudit(t, a, sink)
			seedEntity(app, &entity.Entity{
				ID: "TKT-001", Type: "ticket",
				Properties: map[string]interface{}{"title": "audit-test"},
			})

			// Per-entity GET triggers computeActions +
			// computeCollectionActions — both read-only.
			_ = fetchActions(t, app, "ticket", "tickets", "TKT-001")

			if got := len(sink.Records()); got != 0 {
				t.Errorf("expected 0 audit records on read path, got %d: %+v", got, sink.Records())
			}
		})
	}
}

// With no policy, "" and "none" both yield the permissive Nop.
func TestResolverFromProfile_None(t *testing.T) {
	for _, in := range []string{"", "none"} {
		t.Run(in, func(t *testing.T) {
			r, err := ResolverFromProfile(in, nil, nil, nil)
			if err != nil {
				t.Fatalf("ResolverFromProfile(%q): %v", in, err)
			}
			if _, ok := r.(NopFieldVerdictResolver); !ok {
				t.Fatalf("ResolverFromProfile(%q): got %T, want NopFieldVerdictResolver", in, r)
			}
		})
	}
}

// "demo" is a hard override — selected even when a policy with
// affordance grants is present (AC6).
func TestResolverFromProfile_Demo(t *testing.T) {
	r, err := ResolverFromProfile("demo", nil, nil, nil)
	if err != nil {
		t.Fatalf("ResolverFromProfile(demo): %v", err)
	}
	if _, ok := r.(DemoFieldVerdictResolver); !ok {
		t.Fatalf("ResolverFromProfile(demo): got %T, want DemoFieldVerdictResolver", r)
	}
}

func TestResolverFromProfile_Unknown_FallsBackToPolicyOrNone(t *testing.T) {
	// Unknown values must not panic and (absent a policy) fall back to
	// none. The warning log is observable in stderr; not captured here.
	r, err := ResolverFromProfile("not-a-real-profile", nil, nil, nil)
	if err != nil {
		t.Fatalf("unknown profile: %v", err)
	}
	if _, ok := r.(NopFieldVerdictResolver); !ok {
		t.Fatalf("unknown profile: got %T, want NopFieldVerdictResolver", r)
	}
}

func TestNopFieldVerdictResolver_ReturnsEmpty(t *testing.T) {
	r := NopFieldVerdictResolver{}
	e := &entity.Entity{ID: "TKT-1", Type: "ticket"}

	fv := r.FieldVerdicts(context.Background(), e)
	if fv.Writable != nil || fv.Visible != nil || fv.Options != nil {
		t.Fatalf("NopFieldVerdictResolver.FieldVerdicts: want zero, got %+v", fv)
	}

	rv := r.RelationVerdicts(context.Background(), e)
	if rv.Types != nil {
		t.Fatalf("NopFieldVerdictResolver.RelationVerdicts: want zero, got %+v", rv)
	}
}

func TestDemoFieldVerdictResolver_TicketFixture(t *testing.T) {
	r := DemoFieldVerdictResolver{}
	e := &entity.Entity{ID: "TKT-1", Type: "ticket"}

	fv := r.FieldVerdicts(context.Background(), e)

	if got, want := fv.Writable["kind"], false; got != want {
		t.Errorf("Writable[kind]: got %v, want %v", got, want)
	}
	if got, want := fv.Visible["priority"], false; got != want {
		t.Errorf("Visible[priority]: got %v, want %v", got, want)
	}
	if got, want := fv.Options["effort"]["l"], false; got != want {
		t.Errorf("Options[effort][l]: got %v, want %v", got, want)
	}
	if got, want := fv.Options["effort"]["xl"], false; got != want {
		t.Errorf("Options[effort][xl]: got %v, want %v", got, want)
	}
	if got, want := fv.Options["status"]["done"], false; got != want {
		t.Errorf("Options[status][done]: got %v, want %v", got, want)
	}
	// Sparseness: properties not in the fixture must NOT appear.
	if _, ok := fv.Writable["title"]; ok {
		t.Errorf("Writable should be sparse; title appeared")
	}
	if _, ok := fv.Options["status"]["review"]; ok {
		t.Errorf("Options[status] should be sparse; review appeared")
	}

	rv := r.RelationVerdicts(context.Background(), e)

	cases := []struct {
		typ          string
		wantCreate   bool
		wantRemove   bool
		metaField    string
		wantWritable bool
	}{
		{"affects", false, true, "", false},
		{"implements", true, false, "", false},
		{"has-planning", true, true, "note", false},
	}
	for _, c := range cases {
		t.Run(c.typ, func(t *testing.T) {
			v, ok := rv.Types[c.typ]
			if !ok {
				t.Fatalf("Types[%s] missing", c.typ)
			}
			if v.Creatable != c.wantCreate {
				t.Errorf("Types[%s].Creatable: got %v, want %v", c.typ, v.Creatable, c.wantCreate)
			}
			if v.Removable != c.wantRemove {
				t.Errorf("Types[%s].Removable: got %v, want %v", c.typ, v.Removable, c.wantRemove)
			}
			if c.metaField != "" {
				if got := v.Fields[c.metaField]; got != c.wantWritable {
					t.Errorf("Types[%s].Fields[%s]: got %v, want %v", c.typ, c.metaField, got, c.wantWritable)
				}
			}
		})
	}
}

func TestDemoFieldVerdictResolver_NonTicket_ReturnsEmpty(t *testing.T) {
	r := DemoFieldVerdictResolver{}
	e := &entity.Entity{ID: "X-1", Type: "feature"}

	if fv := r.FieldVerdicts(context.Background(), e); fv.Writable != nil || fv.Visible != nil || fv.Options != nil {
		t.Fatalf("non-ticket FieldVerdicts: want zero, got %+v", fv)
	}
	if rv := r.RelationVerdicts(context.Background(), e); rv.Types != nil {
		t.Fatalf("non-ticket RelationVerdicts: want zero, got %+v", rv)
	}
}

func TestDemoFieldVerdictResolver_NilEntity_ReturnsEmpty(t *testing.T) {
	r := DemoFieldVerdictResolver{}
	if fv := r.FieldVerdicts(context.Background(), nil); fv.Writable != nil {
		t.Fatalf("nil entity: should be zero, got %+v", fv)
	}
	if rv := r.RelationVerdicts(context.Background(), nil); rv.Types != nil {
		t.Fatalf("nil entity: should be zero, got %+v", rv)
	}
}

// fakeResolver lets tests inject arbitrary verdicts without
// instantiating an entity manager / metamodel.
type fakeResolver struct {
	fv FieldVerdicts
	rv RelationVerdicts
}

func (f fakeResolver) FieldVerdicts(context.Context, *entity.Entity) FieldVerdicts {
	return f.fv
}
func (f fakeResolver) RelationVerdicts(context.Context, *entity.Entity) RelationVerdicts {
	return f.rv
}

// byTypeResolver returns different verdicts depending on the entity
// type. Used by tests that need to prove the source-not-path-entity
// resolution: an incoming-direction edge should resolve the verdict
// against the SOURCE entity's type, not the path entity's type.
type byTypeResolver struct {
	rvByType map[string]RelationVerdicts
}

func (r byTypeResolver) FieldVerdicts(context.Context, *entity.Entity) FieldVerdicts {
	return FieldVerdicts{}
}
func (r byTypeResolver) RelationVerdicts(_ context.Context, e *entity.Entity) RelationVerdicts {
	if e == nil {
		return RelationVerdicts{}
	}
	return r.rvByType[e.Type]
}

func appWithResolver(r FieldVerdictResolver) *App {
	return &App{fieldResolver: r}
}

// verdictBuilder is a fluent helper for constructing fakeResolver
// fixtures without nested struct literals. Each setter mutates and
// returns the builder so calls chain; Build() snapshots a
// fakeResolver suitable for assignment to App.fieldResolver.
//
// Conventions:
//   - ReadOnly("field") marks a field non-writable.
//   - Hidden("field") marks a field non-visible.
//   - EnumDeny("field", "value") filters out a single enum option;
//     repeat per option to deny several.
//   - RelationDenyCreate / DenyRemove / DenyMeta wire the matching
//     relation verdicts.
//
// Defaults are permissive: any field / option / relation not
// explicitly denied is allowed. Matches the production semantics.
type verdictBuilder struct {
	fv FieldVerdicts
	rv RelationVerdicts
}

// newVerdicts starts an empty builder.
func newVerdicts() *verdictBuilder { return &verdictBuilder{} }

// ReadOnly marks a field as non-writable.
func (b *verdictBuilder) ReadOnly(field string) *verdictBuilder {
	if b.fv.Writable == nil {
		b.fv.Writable = make(map[string]bool)
	}
	b.fv.Writable[field] = false
	return b
}

// Hidden marks a field as non-visible.
func (b *verdictBuilder) Hidden(field string) *verdictBuilder {
	if b.fv.Visible == nil {
		b.fv.Visible = make(map[string]bool)
	}
	b.fv.Visible[field] = false
	return b
}

// EnumDeny filters out a single enum option for the named field.
// Call repeatedly to deny multiple options.
func (b *verdictBuilder) EnumDeny(field, option string) *verdictBuilder {
	if b.fv.Options == nil {
		b.fv.Options = make(map[string]map[string]bool)
	}
	if b.fv.Options[field] == nil {
		b.fv.Options[field] = make(map[string]bool)
	}
	b.fv.Options[field][option] = false
	return b
}

// RelationDenyCreate denies create on the named relation type.
// Removable + meta-field grants stay at their previous values (or
// permissive default).
func (b *verdictBuilder) RelationDenyCreate(relType string) *verdictBuilder {
	rv := b.relationVerdict(relType)
	rv.Creatable = false
	b.rv.Types[relType] = rv
	return b
}

// RelationDenyRemove denies remove on the named relation type.
func (b *verdictBuilder) RelationDenyRemove(relType string) *verdictBuilder {
	rv := b.relationVerdict(relType)
	rv.Removable = false
	b.rv.Types[relType] = rv
	return b
}

// RelationDenyMeta denies write on a named meta field of the named
// relation type. Creatable / Removable default to true (permissive)
// so the test focuses on the meta-field gate alone.
func (b *verdictBuilder) RelationDenyMeta(relType, metaField string) *verdictBuilder {
	rv := b.relationVerdict(relType)
	if rv.Fields == nil {
		rv.Fields = make(map[string]bool)
	}
	rv.Fields[metaField] = false
	b.rv.Types[relType] = rv
	return b
}

// relationVerdict returns the entry for relType, lazily initializing
// the Types map and applying permissive defaults for first-touch.
func (b *verdictBuilder) relationVerdict(relType string) RelationVerdict {
	if b.rv.Types == nil {
		b.rv.Types = make(map[string]RelationVerdict)
	}
	rv, ok := b.rv.Types[relType]
	if !ok {
		rv = RelationVerdict{Creatable: true, Removable: true}
	}
	return rv
}

// Build snapshots the builder into a fakeResolver.
func (b *verdictBuilder) Build() fakeResolver {
	return fakeResolver{fv: b.fv, rv: b.rv}
}

func TestComputeFieldAffordances_NopResolver_EmitsEmptyMap(t *testing.T) {
	a := appWithResolver(NopFieldVerdictResolver{})
	got := a.computeFieldAffordances(context.Background(), &entity.Entity{Type: "ticket"})
	if got == nil {
		t.Fatal("got nil, want empty map")
	}
	if len(got) != 0 {
		t.Errorf("got %v, want empty map (sparse: no deviations)", got)
	}
}

func TestComputeFieldAffordances_SparseWritable(t *testing.T) {
	// title=true is the default (writable) and must NOT appear in
	// output; kind=false deviates and must be emitted.
	a := appWithResolver(fakeResolver{
		fv: FieldVerdicts{
			Writable: map[string]bool{
				"title": true,
				"kind":  false,
			},
		},
	})
	got := a.computeFieldAffordances(context.Background(), &entity.Entity{Type: "ticket"})
	if _, ok := got["title"]; ok {
		t.Errorf("title should be absent (sparse: writable=true is default)")
	}
	kind, ok := got["kind"]
	if !ok {
		t.Fatalf("kind should be present")
	}
	if kind.Writable == nil || *kind.Writable {
		t.Errorf("kind.Writable: got %v, want pointer-to-false", kind.Writable)
	}
}

func TestComputeFieldAffordances_HiddenFieldsOmittedFromMap(t *testing.T) {
	a := appWithResolver(fakeResolver{
		fv: FieldVerdicts{
			Writable: map[string]bool{"priority": false}, // also marked read-only
			Visible:  map[string]bool{"priority": false}, // but hidden takes precedence
			Options:  map[string]map[string]bool{"priority": {"high": false}},
		},
	})
	got := a.computeFieldAffordances(context.Background(), &entity.Entity{Type: "ticket"})
	if _, ok := got["priority"]; ok {
		t.Errorf("hidden field must NOT appear in _fields (closed-world); got %+v", got)
	}
}

func TestComputeFieldAffordances_SparseOptions(t *testing.T) {
	// allowed values (backlog, in-progress) are the default and must
	// be absent from output; done=false deviates and must be emitted.
	a := appWithResolver(fakeResolver{
		fv: FieldVerdicts{
			Options: map[string]map[string]bool{
				"status": {
					"backlog":     true,
					"in-progress": true,
					"done":        false,
				},
			},
		},
	})
	got := a.computeFieldAffordances(context.Background(), &entity.Entity{Type: "ticket"})
	status, ok := got["status"]
	if !ok {
		t.Fatalf("status should be present")
	}
	if _, ok := status.Options["backlog"]; ok {
		t.Errorf("backlog should be absent (sparse: allowed=true is default)")
	}
	if allowed, ok := status.Options["done"]; !ok || allowed {
		t.Errorf("done should be present and false, got %v ok=%v", allowed, ok)
	}
}

func TestComputeFieldAffordances_OptionsWithoutWritableEntry(t *testing.T) {
	a := appWithResolver(fakeResolver{
		fv: FieldVerdicts{
			Options: map[string]map[string]bool{
				"effort": {"l": false, "xl": false},
			},
		},
	})
	got := a.computeFieldAffordances(context.Background(), &entity.Entity{Type: "ticket"})
	effort, ok := got["effort"]
	if !ok {
		t.Fatalf("effort should be present")
	}
	if effort.Writable != nil {
		t.Errorf("Writable should be nil (no override), got %v", effort.Writable)
	}
	if len(effort.Options) != 2 {
		t.Errorf("Options: got %d entries, want 2", len(effort.Options))
	}
}

func TestComputeFieldAffordances_AllOptionsAllowed_NoEntry(t *testing.T) {
	a := appWithResolver(fakeResolver{
		fv: FieldVerdicts{
			Options: map[string]map[string]bool{
				"status": {"backlog": true, "ready": true},
			},
		},
	})
	got := a.computeFieldAffordances(context.Background(), &entity.Entity{Type: "ticket"})
	if _, ok := got["status"]; ok {
		t.Errorf("status should be absent when all options allowed")
	}
}

func TestHiddenProperties_NopResolver_ReturnsNil(t *testing.T) {
	a := appWithResolver(NopFieldVerdictResolver{})
	if got := a.hiddenProperties(context.Background(), &entity.Entity{Type: "ticket"}); got != nil {
		t.Errorf("got %v, want nil", got)
	}
}

func TestHiddenProperties_OnlyHiddenEntries(t *testing.T) {
	a := appWithResolver(fakeResolver{
		fv: FieldVerdicts{
			Visible: map[string]bool{
				"title":    true,
				"priority": false,
			},
		},
	})
	got := a.hiddenProperties(context.Background(), &entity.Entity{Type: "ticket"})
	if _, ok := got["priority"]; !ok {
		t.Errorf("priority should be in hidden set: %v", got)
	}
	if _, ok := got["title"]; ok {
		t.Errorf("title should NOT be in hidden set (visible=true): %v", got)
	}
}

func TestComputeRelationAffordances_NopResolver_EmitsEmptyMap(t *testing.T) {
	a := appWithResolver(NopFieldVerdictResolver{})
	got := a.computeRelationAffordances(context.Background(), &entity.Entity{Type: "ticket"})
	if got == nil {
		t.Fatal("got nil, want empty map")
	}
	if len(got) != 0 {
		t.Errorf("got %v, want empty", got)
	}
}

func TestComputeRelationAffordances_SparseCreatableRemovable(t *testing.T) {
	a := appWithResolver(fakeResolver{
		rv: RelationVerdicts{
			Types: map[string]RelationVerdict{
				"affects":    {Creatable: false, Removable: true},
				"implements": {Creatable: true, Removable: false},
				"depends-on": {Creatable: true, Removable: true}, // all defaults; should be absent
			},
		},
	})
	got := a.computeRelationAffordances(context.Background(), &entity.Entity{Type: "ticket"})

	if _, ok := got["depends-on"]; ok {
		t.Errorf("depends-on should be absent (all defaults)")
	}

	affects, ok := got["affects"]
	if !ok {
		t.Fatalf("affects should be present")
	}
	if affects.Creatable == nil || *affects.Creatable {
		t.Errorf("affects.Creatable: got %v, want false-pointer", affects.Creatable)
	}
	if affects.Removable != nil {
		t.Errorf("affects.Removable: got %v, want nil (sparse: removable=true default)", affects.Removable)
	}

	implements, ok := got["implements"]
	if !ok {
		t.Fatalf("implements should be present")
	}
	if implements.Creatable != nil {
		t.Errorf("implements.Creatable: got %v, want nil", implements.Creatable)
	}
	if implements.Removable == nil || *implements.Removable {
		t.Errorf("implements.Removable: got %v, want false-pointer", implements.Removable)
	}
}

func TestComputeRelationAffordances_MetaFieldOverrides(t *testing.T) {
	a := appWithResolver(fakeResolver{
		rv: RelationVerdicts{
			Types: map[string]RelationVerdict{
				"has-planning": {
					Creatable: true,
					Removable: true,
					Fields: map[string]bool{
						"note": false,
						"role": true, // default; absent
					},
				},
			},
		},
	})
	got := a.computeRelationAffordances(context.Background(), &entity.Entity{Type: "ticket"})
	hp, ok := got["has-planning"]
	if !ok {
		t.Fatalf("has-planning should be present (meta-field deviation)")
	}
	if hp.Creatable != nil || hp.Removable != nil {
		t.Errorf("Creatable/Removable should be nil (defaults), got %v/%v", hp.Creatable, hp.Removable)
	}
	if _, present := hp.Fields["role"]; present {
		t.Errorf("role should be absent (writable=true default)")
	}
	note, ok := hp.Fields["note"]
	if !ok {
		t.Fatalf("note should be present")
	}
	if note.Writable == nil || *note.Writable {
		t.Errorf("note.Writable: got %v, want false-pointer", note.Writable)
	}
}
