package entitymanager_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/autocascade"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/flowcfg/transitions"
	"github.com/Sourcehaven-BV/rela/internal/flowcfg/triggers"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

const relTriggerSchema = `
version: "1.0"
entities:
  task:
    label: Task
    id_prefix: "T-"
    properties:
      title: {type: string}
      linked: {type: string}
      code: {type: string, unique: true}
  project:
    label: Project
    id_prefix: "P-"
    properties:
      title: {type: string}
  doc:
    label: Doc
    id_prefix: "D-"
    faces:
      draft: {}
      published: {}
    properties:
      title: {type: string}
      cited: {type: string}
relations:
  in_project: {from: [task], to: [project]}
  queued: {from: [task], to: [project], orderable: incoming}
  tagged: {from: [task], to: [project]}
  claims: {from: [task], to: [project]}
  chain: {from: [task], to: [task]}
  fan: {from: [task], to: [task]}
  cites: {from: [doc], to: [project], scope: content}
  blocks: {from: [task], to: [project]}
  mentions: {from: [doc], to: [project]}
`

const relTriggerFlows = `
triggers:
  mark-linked:
    on: {relation_created: in_project}
    outputs:
      - set: {properties: {linked: "yes {{new.title}}"}}
  mark-unlinked:
    on: {relation_removed: in_project}
    outputs:
      - set: {properties: {linked: "no"}}
  mark-queued:
    on: {relation_created: queued}
    outputs:
      - set: {properties: {linked: "queued"}}
  claim-code:
    on: {relation_created: claims}
    outputs:
      - set: {properties: {code: "X"}}
  mark-cited:
    on: {relation_created: cites}
    outputs:
      - set: {properties: {cited: "yes"}}
  mark-mentioned:
    on: {relation_created: mentions}
    outputs:
      - set: {properties: {cited: "mentioned"}}
  refuse-blocked:
    on: {relation_created: blocks}
    outputs:
      - check: {where: "title!=One", severity: error, message: "{{new.title}} cannot be blocked"}
  grow-chain:
    on: {relation_created: chain}
    outputs:
      - create:
          type: task
          properties: {title: "link"}
          relate: {type: chain, from: subject}
          if_exists: replace
  grow-fan:
    on: {relation_created: fan}
    outputs:
      - create:
          type: task
          properties: {title: "left"}
          relate: {type: fan, from: subject}
          if_exists: replace
      - create:
          type: task
          properties: {title: "right"}
          relate: {type: fan, from: subject}
          if_exists: replace
`

type relTriggerBackend struct {
	name string
	open func(t *testing.T) store.Store
}

// relTriggerBackends is extended by the sqlite build-tagged file, so the
// rollback case runs on a store that rolls back.
var relTriggerBackends = []relTriggerBackend{
	{name: "memstore", open: func(*testing.T) store.Store { return memstore.New() }},
}

func newRelTriggerManager(t *testing.T, st store.Store) (*entitymanager.Manager, *audit.Memory) {
	t.Helper()
	return newRelTriggerManagerWith(t, st, acl.NopACL{})
}

func newRelTriggerManagerWith(t *testing.T, st store.Store, a acl.ACL) (*entitymanager.Manager, *audit.Memory) {
	t.Helper()
	meta, err := metamodel.Parse([]byte(relTriggerSchema))
	require.NoError(t, err)
	set, err := transitions.CompileTransitions(meta)
	require.NoError(t, err)
	rules, err := triggers.Parse([]byte(relTriggerFlows), meta)
	require.NoError(t, err)
	engine, err := triggers.New(meta, rules)
	require.NoError(t, err)
	runner, err := autocascade.New(autocascade.Deps{Engine: engine})
	require.NoError(t, err)
	aud := audit.NewMemory()
	mgr, err := entitymanager.New(entitymanager.Deps{
		Store: st, Meta: meta, Templater: nopTemplater{}, Audit: aud,
		ACL: a, Transitions: set,
		Automations: engine, Cascade: runner,
		FieldGate:      entitymanager.AllowAllFieldGate{},
		AnchorGuard:    entitymanager.NopAnchorGuard{},
		CopyReadGate:   entitymanager.AllowAllCopyReadGate{},
		CopyVisibility: allowAllCopyVisibility(t, st),
	})
	require.NoError(t, err)
	ctx := context.Background()
	for _, e := range []*entity.Entity{
		{ID: "T-1", Type: "task", Properties: map[string]any{"title": "One"}},
		{ID: "T-9", Type: "task", Properties: map[string]any{"title": "Nine", "code": "X"}},
		{ID: "P-1", Type: "project", Properties: map[string]any{"title": "Proj"}},
		{ID: "D-1", Type: "doc", Face: "draft"},
		{ID: "D-1", Type: "doc", Face: "published"},
	} {
		require.NoError(t, st.CreateEntity(ctx, e))
	}
	return mgr, aud
}

// taskKey names the edge of type typ from T-1 to to.
func taskKey(typ, to string) entity.RelationKey {
	return entity.RelationKey{From: "T-1", Type: typ, To: to}
}

func propOf(t *testing.T, st store.Store, id string, face entity.Face, name string) any {
	t.Helper()
	e, err := st.GetEntity(context.Background(), entity.Ref{ID: id, Face: face})
	require.NoError(t, err)
	return e.Properties[name]
}

// TestRelationTriggers_Fire pins which relation writes run their triggers
// and that a trigger's property write lands on the relation's source.
func TestRelationTriggers_Fire(t *testing.T) {
	tests := []struct {
		name  string
		write func(ctx context.Context, m *entitymanager.Manager) error
		id    string
		face  entity.Face
		prop  string
		want  any
	}{
		{"create fires on the source", func(ctx context.Context, m *entitymanager.Manager) error {
			_, err := m.CreateRelation(ctx, taskKey("in_project", "P-1"), entity.RelationOptions{})
			return err
		}, "T-1", "", "linked", "yes One"},
		{"delete fires on the source", func(ctx context.Context, m *entitymanager.Manager) error {
			if _, err := m.CreateRelation(ctx, taskKey("in_project", "P-1"), entity.RelationOptions{}); err != nil {
				return err
			}
			return m.DeleteRelation(ctx, taskKey("in_project", "P-1"))
		}, "T-1", "", "linked", "no"},
		{"replace fires remove then create", func(ctx context.Context, m *entitymanager.Manager) error {
			if _, err := m.CreateRelation(ctx, taskKey("tagged", "P-1"), entity.RelationOptions{}); err != nil {
				return err
			}
			_, err := m.ReplaceRelations(ctx,
				[]entitymanager.RelationCreate{{Key: taskKey("in_project", "P-1")}},
				[]entity.RelationKey{taskKey("tagged", "P-1")})
			return err
		}, "T-1", "", "linked", "yes One"},
		{"an unwatched relation fires nothing", func(ctx context.Context, m *entitymanager.Manager) error {
			_, err := m.CreateRelation(ctx, taskKey("tagged", "P-1"), entity.RelationOptions{})
			return err
		}, "T-1", "", "linked", nil},
		{"a tailed edge fires for its face", func(ctx context.Context, m *entitymanager.Manager) error {
			_, err := m.CreateRelation(ctx,
				entity.RelationKey{From: "D-1", FromFace: "draft", Type: "cites", To: "P-1"}, entity.RelationOptions{})
			return err
		}, "D-1", "draft", "cited", "yes"},
		{"a tailed edge leaves the other face alone", func(ctx context.Context, m *entitymanager.Manager) error {
			_, err := m.CreateRelation(ctx,
				entity.RelationKey{From: "D-1", FromFace: "draft", Type: "cites", To: "P-1"}, entity.RelationOptions{})
			return err
		}, "D-1", "published", "cited", nil},
		{"an entity delete does not fire relation_removed", func(ctx context.Context, m *entitymanager.Manager) error {
			if _, err := m.CreateRelation(ctx, taskKey("in_project", "P-1"), entity.RelationOptions{}); err != nil {
				return err
			}
			_, err := m.DeleteEntity(ctx, "P-1", true)
			return err
		}, "T-1", "", "linked", "yes One"},
		{"a patch with relations fires after the patch", func(ctx context.Context, m *entitymanager.Manager) error {
			g, err := m.AuthorizeUpdate(ctx, &entity.Entity{ID: "T-1", Type: "task"})
			if err != nil {
				return err
			}
			_, _, err = entitymanager.PatchWithRelations(ctx, g, "T-1",
				entity.Patch{Properties: map[string]any{"title": "Two"}},
				[]entitymanager.RelationCreate{{Key: taskKey("in_project", "P-1")}}, nil)
			return err
		}, "T-1", "", "linked", "yes Two"},
		{"a reorder fires nothing", func(ctx context.Context, m *entitymanager.Manager) error {
			if _, err := m.CreateRelation(ctx, taskKey("queued", "P-1"), entity.RelationOptions{}); err != nil {
				return err
			}
			if _, err := m.PatchEntity(ctx, "T-1", entity.Patch{MetaUnset: []string{"linked"}}); err != nil {
				return err
			}
			_, err := m.UpdateRelation(ctx, taskKey("queued", "P-1"),
				entity.RelationOptions{Properties: map[string]any{"_order_in": 5.0}})
			return err
		}, "T-1", "", "linked", nil},
	}
	for _, b := range relTriggerBackends {
		for _, tc := range tests {
			t.Run(b.name+"/"+tc.name, func(t *testing.T) {
				st := b.open(t)
				m, _ := newRelTriggerManager(t, st)
				require.NoError(t, tc.write(context.Background(), m))
				require.Equal(t, tc.want, propOf(t, st, tc.id, tc.face, tc.prop))
			})
		}
	}
}

// TestRelationTriggers_AuditNamesTheRule pins that the trigger's property
// write is audited as the rule's, after the relation's own record.
func TestRelationTriggers_AuditNamesTheRule(t *testing.T) {
	m, aud := newRelTriggerManager(t, memstore.New())
	_, err := m.CreateRelation(context.Background(), taskKey("in_project", "P-1"), entity.RelationOptions{})
	require.NoError(t, err)
	var ops []string
	for _, rec := range aud.Records() {
		ops = append(ops, rec.Op+" "+rec.TriggeredBy)
	}
	require.Equal(t, []string{
		audit.OpCreateRelation + " ",
		audit.OpUpdateEntity + " automation:mark-linked",
	}, ops)
}

// TestRelationTriggers_FailedSetFailsTheWrite pins that a refused trigger
// write fails the relation write, and that a store that rolls back keeps
// neither the relation nor its audit record.
func TestRelationTriggers_FailedSetFailsTheWrite(t *testing.T) {
	for _, b := range relTriggerBackends {
		t.Run(b.name, func(t *testing.T) {
			st := b.open(t)
			m, aud := newRelTriggerManager(t, st)
			ctx := context.Background()
			_, err := m.CreateRelation(ctx, taskKey("claims", "P-1"), entity.RelationOptions{})
			require.Error(t, err)
			_, getErr := st.GetRelation(ctx, taskKey("claims", "P-1"))
			require.Nil(t, propOf(t, st, "T-1", "", "code"))
			if store.TxRollsBack(st) {
				require.ErrorIs(t, getErr, store.ErrNotFound)
				require.Empty(t, aud.Records())
			} else {
				require.NoError(t, getErr)
			}
		})
	}
}

// TestRelationTriggers_ErrorCheckRefusesTheWrite pins that a failed check
// of severity error refuses the relation write.
func TestRelationTriggers_ErrorCheckRefusesTheWrite(t *testing.T) {
	for _, b := range relTriggerBackends {
		t.Run(b.name, func(t *testing.T) {
			st := b.open(t)
			m, _ := newRelTriggerManager(t, st)
			ctx := context.Background()
			_, err := m.CreateRelation(ctx, taskKey("blocks", "P-1"), entity.RelationOptions{})
			require.ErrorIs(t, err, entitymanager.ErrRelationTriggerCheck)
			require.ErrorContains(t, err, "One cannot be blocked")
			_, getErr := st.GetRelation(ctx, taskKey("blocks", "P-1"))
			if store.TxRollsBack(st) {
				require.ErrorIs(t, getErr, store.ErrNotFound)
			}
		})
	}
}

// TestRelationTriggers_ChainStops pins that a trigger whose outputs create
// the relation it watches stops at the budget, both for a chain and for a
// trigger that fans out, and that the root write still succeeds. Each rule
// replaces its last target, so every dispatch writes a new edge.
func TestRelationTriggers_ChainStops(t *testing.T) {
	tests := []struct {
		name    string
		relType string
	}{
		{"chain", "chain"},
		{"fan-out", "fan"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m, aud := newRelTriggerManager(t, memstore.New())
			ctx := context.Background()
			_, err := m.CreateRelation(ctx, taskKey(tc.relType, "T-9"), entity.RelationOptions{})
			require.NoError(t, err)
			edges := 0
			for _, rec := range aud.Records() {
				if rec.Op == audit.OpCreateRelation {
					edges++
				}
			}
			require.Greater(t, edges, 2, "the trigger did not run")
			require.LessOrEqual(t, edges, autocascade.MaxDepth, "the chain was not stopped")
		})
	}
}

// TestRelationTriggers_ExhaustedBudgetRefusesBeforeWriting pins the error a
// chain's last write gets, and that it writes nothing.
func TestRelationTriggers_ExhaustedBudgetRefusesBeforeWriting(t *testing.T) {
	st := memstore.New()
	m, _ := newRelTriggerManager(t, st)
	ctx := entitymanager.WithSpentRelationBudgetForTest(context.Background())
	_, err := m.CreateRelation(ctx, taskKey("in_project", "P-1"), entity.RelationOptions{})
	require.ErrorIs(t, err, entitymanager.ErrTriggerDepthExceeded)
	_, getErr := st.GetRelation(context.Background(), taskKey("in_project", "P-1"))
	require.ErrorIs(t, getErr, store.ErrNotFound)
}
