package entitymanager_test

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/flowcfg/transitions"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// scopedMetamodel is the to-do shape of TKT-5GK80Q9: a todo sits in one
// project, and may sit under a heading of that project. under_heading
// clears on a scope change; under_section refuses it.
const scopedMetamodel = `version: "1.0"
entities:
  todo: {label: Todo, id_prefix: "TODO-", id_type: sequential, properties: {title: {type: string}}}
  project: {label: Project, id_prefix: "PRJ-", id_type: sequential, properties: {title: {type: string}}}
  heading: {label: Heading, id_prefix: "HEAD-", id_type: sequential, properties: {title: {type: string}}}
relations:
  in_project:
    label: in project
    from: [todo]
    to: [project]
    max_outgoing: 1
  has_heading:
    label: has heading
    from: [project]
    to: [heading]
    owning: true
  under_heading:
    label: under heading
    from: [todo]
    to: [heading]
    within: in_project
    on_change: clear
  under_section:
    label: under section
    from: [todo]
    to: [heading]
    within: in_project
    on_change: reject
`

type scopedFixture struct {
	mgr *entitymanager.Manager
	st  store.Store
	log *audit.Memory
}

func newScopedFixture(t *testing.T, b concBackend, a acl.ACL) *scopedFixture {
	t.Helper()
	meta, err := metamodel.Parse([]byte(scopedMetamodel))
	require.NoError(t, err)
	st := b.open(t)
	log := audit.NewMemory()
	mgr, err := entitymanager.New(entitymanager.Deps{
		Store: st, Meta: meta, Templater: nopTemplater{}, Audit: log, ACL: a,
		Transitions: transitions.NoTransitions(), FieldGate: entitymanager.AllowAllFieldGate{},
		AnchorGuard: entitymanager.NopAnchorGuard{},
	})
	require.NoError(t, err)
	return &scopedFixture{mgr: mgr, st: st, log: log}
}

func (f *scopedFixture) mk(t *testing.T, typ string) string {
	t.Helper()
	e := entity.New("", typ)
	e.SetString("title", typ)
	res, err := f.mgr.CreateEntity(context.Background(), e, entity.CreateOptions{})
	require.NoError(t, err)
	return res.Entity.ID
}

func (f *scopedFixture) link(from, relType, to string) error {
	_, err := f.mgr.CreateRelation(context.Background(),
		entity.RelationKey{From: from, Type: relType, To: to}, entity.RelationOptions{})
	return err
}

func (f *scopedFixture) targets(t *testing.T, from, relType string) []string {
	t.Helper()
	var out []string
	q := store.RelationQuery{From: from, Type: relType, Direction: store.DirectionOutgoing}
	for r, err := range f.st.ListRelations(context.Background(), q) {
		require.NoError(t, err)
		out = append(out, r.To)
	}
	slices.Sort(out)
	return out
}

// world is two projects with a heading each, and a todo in the first.
type scopedWorld struct{ todo, p1, p2, h1, h2 string }

func (f *scopedFixture) world(t *testing.T) scopedWorld {
	t.Helper()
	w := scopedWorld{
		todo: f.mk(t, "todo"), p1: f.mk(t, "project"), p2: f.mk(t, "project"),
		h1: f.mk(t, "heading"), h2: f.mk(t, "heading"),
	}
	require.NoError(t, f.link(w.p1, "has_heading", w.h1))
	require.NoError(t, f.link(w.p2, "has_heading", w.h2))
	require.NoError(t, f.link(w.todo, "in_project", w.p1))
	return w
}

func skey(from, relType, to string) entity.RelationKey {
	return entity.RelationKey{From: from, Type: relType, To: to}
}

// requireScopeRefusal asserts err is the scope refusal of rel, and that it
// names no entity.
func requireScopeRefusal(t *testing.T, err error, rel string) {
	t.Helper()
	var ve *entitymanager.ValidationError
	require.ErrorAs(t, err, &ve)
	require.Len(t, ve.Errors, 1)
	got := ve.Errors[0]
	require.Equal(t, metamodel.ValidationErrorScope, got.Type)
	require.Equal(t, rel, got.Property)
	require.Equal(t, `relation "`+rel+`" must point to an entity owned by the target of "in_project"`, got.Message)
	for _, prefix := range []string{"TODO-", "PRJ-", "HEAD-"} {
		require.NotContains(t, err.Error(), prefix)
	}
}

func TestScoped_CreateRelation(t *testing.T) {
	for _, b := range concBackends {
		t.Run(b.name, func(t *testing.T) {
			f := newScopedFixture(t, b, acl.NopACL{})
			w := f.world(t)
			require.NoError(t, f.link(w.todo, "under_heading", w.h1))
			// A target owned by another project is refused, whatever on_change says.
			requireScopeRefusal(t, f.link(w.todo, "under_section", w.h2), "under_section")
			// An empty scope admits nothing.
			loose := f.mk(t, "todo")
			requireScopeRefusal(t, f.link(loose, "under_heading", w.h1), "under_heading")
			// A target with no owner admits nothing.
			orphan := f.mk(t, "heading")
			requireScopeRefusal(t, f.link(w.todo, "under_section", orphan), "under_section")
			require.Equal(t, []string{w.h1}, f.targets(t, w.todo, "under_heading"))
			require.Empty(t, f.targets(t, w.todo, "under_section"))
		})
	}
}

func TestScoped_ScopeChange(t *testing.T) {
	ctx := context.Background()
	for _, b := range concBackends {
		t.Run(b.name, func(t *testing.T) {
			t.Run("replace clears", func(t *testing.T) {
				f := newScopedFixture(t, b, acl.NopACL{})
				w := f.world(t)
				require.NoError(t, f.link(w.todo, "under_heading", w.h1))
				_, err := f.mgr.ReplaceRelations(ctx,
					[]entitymanager.RelationCreate{{Key: skey(w.todo, "in_project", w.p2)}},
					[]entity.RelationKey{skey(w.todo, "in_project", w.p1)})
				require.NoError(t, err)
				require.Equal(t, []string{w.p2}, f.targets(t, w.todo, "in_project"))
				require.Empty(t, f.targets(t, w.todo, "under_heading"))
				requireClearAudited(t, f.log, w.todo, w.h1)
			})
			t.Run("replace with new target in new scope", func(t *testing.T) {
				f := newScopedFixture(t, b, acl.NopACL{})
				w := f.world(t)
				require.NoError(t, f.link(w.todo, "under_section", w.h1))
				_, err := f.mgr.ReplaceRelations(ctx,
					[]entitymanager.RelationCreate{
						{Key: skey(w.todo, "in_project", w.p2)}, {Key: skey(w.todo, "under_section", w.h2)},
					},
					[]entity.RelationKey{skey(w.todo, "in_project", w.p1), skey(w.todo, "under_section", w.h1)})
				require.NoError(t, err)
				require.Equal(t, []string{w.h2}, f.targets(t, w.todo, "under_section"))
			})
			t.Run("replace rejects", func(t *testing.T) {
				f := newScopedFixture(t, b, acl.NopACL{})
				w := f.world(t)
				require.NoError(t, f.link(w.todo, "under_section", w.h1))
				_, err := f.mgr.ReplaceRelations(ctx,
					[]entitymanager.RelationCreate{{Key: skey(w.todo, "in_project", w.p2)}},
					[]entity.RelationKey{skey(w.todo, "in_project", w.p1)})
				requireScopeRefusal(t, err, "under_section")
				require.Equal(t, []string{w.p1}, f.targets(t, w.todo, "in_project"))
				require.Equal(t, []string{w.h1}, f.targets(t, w.todo, "under_section"))
			})
			t.Run("delete scope clears", func(t *testing.T) {
				f := newScopedFixture(t, b, acl.NopACL{})
				w := f.world(t)
				require.NoError(t, f.link(w.todo, "under_heading", w.h1))
				require.NoError(t, f.mgr.DeleteRelation(ctx, skey(w.todo, "in_project", w.p1)))
				require.Empty(t, f.targets(t, w.todo, "under_heading"))
				requireClearAudited(t, f.log, w.todo, w.h1)
			})
			t.Run("delete scope rejects", func(t *testing.T) {
				f := newScopedFixture(t, b, acl.NopACL{})
				w := f.world(t)
				require.NoError(t, f.link(w.todo, "under_section", w.h1))
				requireScopeRefusal(t, f.mgr.DeleteRelation(ctx, skey(w.todo, "in_project", w.p1)), "under_section")
				require.Equal(t, []string{w.p1}, f.targets(t, w.todo, "in_project"))
			})
			t.Run("unrelated delete leaves scoped edges", func(t *testing.T) {
				f := newScopedFixture(t, b, acl.NopACL{})
				w := f.world(t)
				require.NoError(t, f.link(w.todo, "under_heading", w.h1))
				require.NoError(t, f.mgr.DeleteRelation(ctx, skey(w.todo, "under_heading", w.h1)))
				require.Equal(t, []string{w.p1}, f.targets(t, w.todo, "in_project"))
			})
		})
	}
}

func TestScoped_Reparent(t *testing.T) {
	ctx := context.Background()
	for _, b := range concBackends {
		t.Run(b.name, func(t *testing.T) {
			for _, tc := range []struct {
				rel     string
				cleared bool
			}{{"under_heading", true}, {"under_section", false}} {
				t.Run(tc.rel, func(t *testing.T) {
					f := newScopedFixture(t, b, acl.NopACL{})
					w := f.world(t)
					require.NoError(t, f.link(w.todo, tc.rel, w.h1))
					// Move heading h1 from project p1 to project p2.
					_, err := f.mgr.ReplaceRelations(ctx,
						[]entitymanager.RelationCreate{{Key: skey(w.p2, "has_heading", w.h1)}},
						[]entity.RelationKey{skey(w.p1, "has_heading", w.h1)})
					if !tc.cleared {
						requireScopeRefusal(t, err, tc.rel)
						require.Equal(t, []string{w.h1}, f.targets(t, w.todo, tc.rel))
						require.Equal(t, []string{w.h1}, f.targets(t, w.p1, "has_heading"))
						return
					}
					require.NoError(t, err)
					require.Empty(t, f.targets(t, w.todo, tc.rel))
					require.Equal(t, []string{w.h1, w.h2}, f.targets(t, w.p2, "has_heading"))
				})
			}
		})
	}
}

// TestScoped_ClearNeedsDeleteGrant pins that a clear is a delete the caller
// must be allowed: a caller who may not delete under_heading edges cannot
// change the scope of a todo that has one.
func TestScoped_ClearNeedsDeleteGrant(t *testing.T) {
	f := newScopedFixture(t, concBackends[0], noScopedDeleteACL{})
	w := f.world(t)
	require.NoError(t, f.link(w.todo, "under_heading", w.h1))
	_, err := f.mgr.ReplaceRelations(context.Background(),
		[]entitymanager.RelationCreate{{Key: skey(w.todo, "in_project", w.p2)}},
		[]entity.RelationKey{skey(w.todo, "in_project", w.p1)})
	var forbidden *acl.ForbiddenError
	require.ErrorAs(t, err, &forbidden)
	require.Equal(t, []string{w.p1}, f.targets(t, w.todo, "in_project"))
	require.Equal(t, []string{w.h1}, f.targets(t, w.todo, "under_heading"))
}

// noScopedDeleteACL allows every write except deleting an under_heading edge.
type noScopedDeleteACL struct{ acl.NopACL }

func (noScopedDeleteACL) AuthorizeWrite(_ context.Context, req acl.WriteRequest) acl.Decision {
	if s, rel := req.Subject.(acl.RelationSubject); rel && req.Op == acl.OpDelete && s.Type == "under_heading" {
		return acl.Decision{Allow: false, RuleKind: "test", Reason: "no under_heading delete"}
	}
	return acl.Decision{Allow: true, RuleKind: "test"}
}

// TestScoped_ReparentHiddenSource pins that a re-parent never clears an edge
// of a source the caller cannot read: the write is refused with the scope
// refusal, not a 403, and the hidden edge stays.
func TestScoped_ReparentHiddenSource(t *testing.T) {
	gate := &scopedGateACL{}
	f := newScopedFixture(t, concBackends[0], gate)
	w := f.world(t)
	require.NoError(t, f.link(w.todo, "under_heading", w.h1))
	gate.hidden = w.todo
	_, err := f.mgr.ReplaceRelations(context.Background(),
		[]entitymanager.RelationCreate{{Key: skey(w.p2, "has_heading", w.h1)}},
		[]entity.RelationKey{skey(w.p1, "has_heading", w.h1)})
	requireScopeRefusal(t, err, "under_heading")
	require.Equal(t, []string{w.h1}, f.targets(t, w.todo, "under_heading"))
}

// TestScoped_ClearAuthorizedPerSource pins that each cleared edge is
// authorized on its own source: an allow for one todo does not carry over to
// another todo whose edge the same re-parent clears.
func TestScoped_ClearAuthorizedPerSource(t *testing.T) {
	gate := &scopedGateACL{}
	f := newScopedFixture(t, concBackends[0], gate)
	w := f.world(t)
	other := f.mk(t, "todo")
	require.NoError(t, f.link(other, "in_project", w.p1))
	require.NoError(t, f.link(w.todo, "under_heading", w.h1))
	require.NoError(t, f.link(other, "under_heading", w.h1))
	gate.noDelete = other
	_, err := f.mgr.ReplaceRelations(context.Background(),
		[]entitymanager.RelationCreate{{Key: skey(w.p2, "has_heading", w.h1)}},
		[]entity.RelationKey{skey(w.p1, "has_heading", w.h1)})
	var forbidden *acl.ForbiddenError
	require.ErrorAs(t, err, &forbidden)
	require.Equal(t, []string{w.h1}, f.targets(t, w.todo, "under_heading"))
	require.Equal(t, []string{w.h1}, f.targets(t, other, "under_heading"))
}

// scopedGateACL hides one entity from reads and refuses deleting the edges of
// another; it allows everything else.
type scopedGateACL struct {
	acl.NopACL
	hidden, noDelete string
}

func (g *scopedGateACL) PermitsReadFace(_ context.Context, _, id string, _ entity.Face) (bool, error) {
	return id != g.hidden, nil
}

func (g *scopedGateACL) AuthorizeWrite(_ context.Context, req acl.WriteRequest) acl.Decision {
	if s, rel := req.Subject.(acl.RelationSubject); rel && req.Op == acl.OpDelete && s.FromID == g.noDelete {
		return acl.Decision{Allow: false, RuleKind: "test", Reason: "no delete"}
	}
	return acl.Decision{Allow: true, RuleKind: "test"}
}

// TestScoped_CreateLinked covers a create of the scope target from the form
// of a todo: the new project owns no heading, so the todo's heading clears.
func TestScoped_CreateLinked(t *testing.T) {
	ctx := context.Background()
	for _, b := range concBackends {
		t.Run(b.name, func(t *testing.T) {
			f := newScopedFixture(t, b, acl.NopACL{})
			loose := f.mk(t, "todo")
			p := f.mk(t, "project")
			h := f.mk(t, "heading")
			require.NoError(t, f.link(p, "has_heading", h))
			require.NoError(t, f.link(loose, "in_project", p))
			require.NoError(t, f.link(loose, "under_heading", h))
			require.NoError(t, f.mgr.DeleteRelation(ctx, skey(loose, "under_heading", h)))
			require.NoError(t, f.link(loose, "under_heading", h))

			// Re-point the todo to a new project: in_project is single-valued,
			// so the old edge goes first.
			_, err := f.mgr.ReplaceRelations(ctx, nil, []entity.RelationKey{skey(loose, "in_project", p)})
			require.NoError(t, err)
			require.Empty(t, f.targets(t, loose, "under_heading"))

			require.NoError(t, f.link(loose, "in_project", p))
			require.NoError(t, f.link(loose, "under_heading", h))
			require.NoError(t, f.mgr.DeleteRelation(ctx, skey(loose, "in_project", p)))
			e := entity.New("", "project")
			e.SetString("title", "new")
			_, err = entitymanager.CreateLinked(ctx, f.mgr, entitymanager.LinkedCreate{
				Entity: e, RelateType: "in_project", RelateFrom: loose,
			})
			require.NoError(t, err)
			require.Len(t, f.targets(t, loose, "in_project"), 1)
			require.Empty(t, f.targets(t, loose, "under_heading"))

			// A new heading has no owner, so it cannot be linked as a target.
			hd := entity.New("", "heading")
			hd.SetString("title", "new")
			_, err = entitymanager.CreateLinked(ctx, f.mgr, entitymanager.LinkedCreate{
				Entity: hd, RelateType: "under_heading", RelateFrom: loose,
			})
			requireScopeRefusal(t, err, "under_heading")

			// A new todo has no scope edge, so no heading is in its scope.
			td := entity.New("", "todo")
			td.SetString("title", "new")
			_, err = entitymanager.CreateLinked(ctx, f.mgr, entitymanager.LinkedCreate{
				Entity: td, RelateType: "under_heading", RelateTo: h,
			})
			requireScopeRefusal(t, err, "under_heading")
		})
	}
}

// TestScoped_CyclicOwnershipData pins that ownership is read one hop: data
// in which entities own themselves or each other, which the write path never
// produces, is judged without looping.
func TestScoped_CyclicOwnershipData(t *testing.T) {
	ctx := context.Background()
	f := newScopedFixture(t, concBackends[0], acl.NopACL{})
	w := f.world(t)
	for _, k := range []entity.RelationKey{
		skey(w.h1, "has_heading", w.h1), skey(w.p1, "has_heading", w.p1),
		skey(w.h2, "has_heading", w.p2),
	} {
		_, err := f.st.CreateRelation(ctx, k, &store.RelationData{})
		require.NoError(t, err)
	}
	require.NoError(t, f.link(w.todo, "under_heading", w.h1))
	requireScopeRefusal(t, f.link(w.todo, "under_section", w.h2), "under_section")
	_, err := f.mgr.ReplaceRelations(ctx,
		[]entitymanager.RelationCreate{{Key: skey(w.todo, "in_project", w.p2)}},
		[]entity.RelationKey{skey(w.todo, "in_project", w.p1)})
	require.NoError(t, err)
	require.Empty(t, f.targets(t, w.todo, "under_heading"))
}

// TestScoped_PatchWithRelations covers the form's joined write: a property
// change with a scope change clears in the same transaction.
func TestScoped_PatchWithRelations(t *testing.T) {
	ctx := context.Background()
	f := newScopedFixture(t, concBackends[0], acl.NopACL{})
	w := f.world(t)
	require.NoError(t, f.link(w.todo, "under_heading", w.h1))
	row, err := f.st.GetEntity(ctx, entity.Ref{ID: w.todo, Face: entity.ImplicitFace})
	require.NoError(t, err)
	g, err := f.mgr.AuthorizeUpdate(ctx, row)
	require.NoError(t, err)
	_, _, err = entitymanager.PatchWithRelations(ctx, g, w.todo,
		entity.Patch{Properties: map[string]any{"title": "moved"}},
		[]entitymanager.RelationCreate{{Key: skey(w.todo, "in_project", w.p2)}},
		[]entity.RelationKey{skey(w.todo, "in_project", w.p1)})
	require.NoError(t, err)
	require.Empty(t, f.targets(t, w.todo, "under_heading"))
	requireClearAudited(t, f.log, w.todo, w.h1)
}

// TestScoped_ClearRunsNoAutomation pins that a clear fires no relation
// trigger of its own and does not re-enter the scope check: the manager
// runs no automation on relation writes, and a clear is a relation write.
func TestScoped_ClearRunsNoAutomation(t *testing.T) {
	ctx := context.Background()
	f := newScopedFixture(t, concBackends[0], acl.NopACL{})
	w := f.world(t)
	require.NoError(t, f.link(w.todo, "under_heading", w.h1))
	before := len(f.log.Records())
	_, err := f.mgr.ReplaceRelations(ctx,
		[]entitymanager.RelationCreate{{Key: skey(w.todo, "in_project", w.p2)}},
		[]entity.RelationKey{skey(w.todo, "in_project", w.p1)})
	require.NoError(t, err)
	// One create, one delete, one clear: nothing else was written.
	require.Len(t, f.log.Records()[before:], 3)
}

// requireClearAudited asserts the log holds exactly one clear of from's
// under_heading edge to to.
func requireClearAudited(t *testing.T, log *audit.Memory, from, to string) {
	t.Helper()
	n := 0
	for _, r := range log.Records() {
		if r.Op == audit.OpDeleteRelation && r.Subject != nil && r.Subject.FromID == from &&
			r.Subject.ToID == to && strings.HasPrefix(r.Summary, "cleared: out of scope of in_project") {
			n++
		}
	}
	require.Equal(t, 1, n, "clear audit records")
}
