package entitymanager_test

import (
	"context"
	"slices"
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
)

// relTriggerCheckSchema has a relation rule enforced on write (a done task
// has an owner), an integer property that strict mode guards, a guarded
// state machine and a decimal property.
const relTriggerCheckSchema = `
version: "1.0"
types:
  stage:
    values: [open, approved]
    initial: open
    transitions:
      - {from: open, to: approved, guard: approve}
entities:
  task:
    label: Task
    id_prefix: "T-"
    properties:
      title: {type: string}
      status: {type: string}
      count: {type: integer}
      stage: {type: stage}
      amount: {type: decimal}
      touched: {type: string}
  person:
    label: Person
    id_prefix: "U-"
    properties:
      name: {type: string}
relations:
  owned_by: {from: [task], to: [person]}
  finishes: {from: [task], to: [person]}
  counts: {from: [task], to: [person]}
  approves: {from: [task], to: [person]}
  touches: {from: [task], to: [person]}
validations:
  - name: done-has-owner
    description: A done task has an owner.
    entity_type: task
    enforce: write
    when: ["status=done"]
    then_condition: "related(entity, 'owned_by')"
`

const relTriggerCheckFlows = `
triggers:
  finish:
    on: {relation_created: finishes}
    outputs:
      - set: {properties: {status: done}}
  own:
    on: {relation_created: owned_by}
    outputs:
      - set: {properties: {status: done}}
  disown:
    on: {relation_removed: owned_by}
    outputs:
      - set: {properties: {status: done}}
  count-from-title:
    on: {relation_created: counts}
    outputs:
      - set: {properties: {count: "{{new.title}}"}}
  approve:
    on: {relation_created: approves}
    outputs:
      - set: {properties: {stage: approved}}
  touch:
    on: {relation_created: touches}
    outputs:
      - set: {properties: {touched: "yes"}}
`

// newRelTriggerCheckManager seeds T-1 without an owner and T-2 with one,
// straight on st so no trigger fires. T-1 holds a decimal stored as a
// number, as a row written below the manager may.
func newRelTriggerCheckManager(
	t *testing.T, st store.Store, mode entitymanager.ValidationMode, guard transitions.PermissionGuard,
) (*entitymanager.Manager, *audit.Memory) {
	t.Helper()
	meta, err := metamodel.Parse([]byte(relTriggerCheckSchema))
	require.NoError(t, err)
	set, err := transitions.CompileTransitions(meta)
	require.NoError(t, err)
	rules, err := triggers.Parse([]byte(relTriggerCheckFlows), meta)
	require.NoError(t, err)
	engine, err := triggers.New(meta, rules)
	require.NoError(t, err)
	runner, err := autocascade.New(autocascade.Deps{Engine: engine})
	require.NoError(t, err)
	aud := audit.NewMemory()
	mgr, err := entitymanager.New(entitymanager.Deps{
		Store: st, Meta: meta, Templater: nopTemplater{}, Audit: aud,
		ACL: acl.NopACL{}, Transitions: set, TransitionGuard: guard,
		Automations: engine, Cascade: runner,
		ValidationMode: mode,
		FieldGate:      entitymanager.AllowAllFieldGate{},
		AnchorGuard:    entitymanager.NopAnchorGuard{},
		CopyReadGate:   entitymanager.AllowAllCopyReadGate{},
		CopyVisibility: allowAllCopyVisibility(t, st),
	})
	require.NoError(t, err)
	ctx := context.Background()
	for _, e := range []*entity.Entity{
		{ID: "T-1", Type: "task", Properties: map[string]any{"title": "notanumber", "stage": "open", "amount": 1.5}},
		{ID: "T-2", Type: "task", Properties: map[string]any{"title": "Owned", "stage": "open"}},
		{ID: "U-1", Type: "person", Properties: map[string]any{"name": "Ann"}},
	} {
		require.NoError(t, st.CreateEntity(ctx, e))
	}
	_, err = st.CreateRelation(ctx, entity.RelationKey{From: "T-2", Type: "owned_by", To: "U-1"}, nil)
	require.NoError(t, err)
	return mgr, aud
}

type relCheckOutcome int

const (
	relCheckSaved relCheckOutcome = iota
	relCheckInvalid
	relCheckForbidden
)

type relCheckWrite func(ctx context.Context, m *entitymanager.Manager) error

func relCheckLink(from, typ string) relCheckWrite {
	return func(ctx context.Context, m *entitymanager.Manager) error {
		_, err := m.CreateRelation(ctx, entity.RelationKey{From: from, Type: typ, To: "U-1"}, entity.RelationOptions{})
		return err
	}
}

func relCheckUnlink(from, typ string) relCheckWrite {
	return func(ctx context.Context, m *entitymanager.Manager) error {
		return m.DeleteRelation(ctx, entity.RelationKey{From: from, Type: typ, To: "U-1"})
	}
}

// relCheckPatch patches T-1, the task without an owner.
func relCheckPatch(props map[string]any) relCheckWrite {
	return func(ctx context.Context, m *entitymanager.Manager) error {
		_, err := m.PatchEntity(ctx, "T-1", entity.Patch{Properties: props})
		return err
	}
}

// TestRelationTriggers_RunUpdateChecks pins that a relation trigger's
// property write runs the checks [entitymanager.Manager.PatchEntity] runs:
// a relation rule enforced on write, judged against the edges the relation
// write leaves; strict validation mode; a transition guard; and decimals
// stored as text. Each refused trigger case has a patch case beside it that
// is refused the same way. A refused trigger fails the relation write, and
// a store that rolls back keeps the relation as it was and records no
// write.
func TestRelationTriggers_RunUpdateChecks(t *testing.T) {
	tests := []struct {
		name    string
		mode    entitymanager.ValidationMode
		guard   transitions.PermissionGuard
		write   relCheckWrite
		from    string
		prop    string
		want    any
		outcome relCheckOutcome
		// rel is the relation a trigger case writes; relKept is whether a
		// store that rolls back holds it after a refusal.
		rel     string
		relKept bool
	}{
		{name: "rule refuses trigger", write: relCheckLink("T-1", "finishes"), from: "T-1", prop: "status",
			outcome: relCheckInvalid, rel: "finishes"},
		{name: "rule refuses patch", write: relCheckPatch(map[string]any{"status": "done"}), from: "T-1",
			prop: "status", outcome: relCheckInvalid},
		{name: "rule passes trigger with owner", write: relCheckLink("T-2", "finishes"), from: "T-2",
			prop: "status", want: "done"},
		{name: "rule sees the edge the write creates", write: relCheckLink("T-1", "owned_by"), from: "T-1",
			prop: "status", want: "done"},
		{name: "rule sees the edge the write removes", write: relCheckUnlink("T-2", "owned_by"), from: "T-2",
			prop: "status", outcome: relCheckInvalid, rel: "owned_by", relKept: true},
		{name: "strict refuses trigger", mode: entitymanager.ValidationStrict, write: relCheckLink("T-1", "counts"),
			from: "T-1", prop: "count", outcome: relCheckInvalid, rel: "counts"},
		{name: "strict refuses patch", mode: entitymanager.ValidationStrict,
			write: relCheckPatch(map[string]any{"count": "notanumber"}), from: "T-1", prop: "count",
			outcome: relCheckInvalid},
		{name: "advisory saves trigger", write: relCheckLink("T-1", "counts"), from: "T-1", prop: "count",
			want: "notanumber"},
		{name: "guard refuses trigger", guard: denyAllGuard{}, write: relCheckLink("T-1", "approves"),
			from: "T-1", prop: "stage", want: "open", outcome: relCheckForbidden, rel: "approves"},
		{name: "guard refuses patch", guard: denyAllGuard{},
			write: relCheckPatch(map[string]any{"stage": "approved"}), from: "T-1", prop: "stage",
			want: "open", outcome: relCheckForbidden},
		{name: "guard passes trigger", write: relCheckLink("T-1", "approves"), from: "T-1", prop: "stage",
			want: "approved"},
		{name: "trigger stores decimal as text", write: relCheckLink("T-1", "touches"), from: "T-1",
			prop: "amount", want: "1.5"},
		{name: "patch stores decimal as text", write: relCheckPatch(map[string]any{"touched": "yes"}),
			from: "T-1", prop: "amount", want: "1.5"},
	}
	for _, b := range relTriggerBackends {
		for _, tc := range tests {
			t.Run(b.name+"/"+tc.name, func(t *testing.T) {
				st := b.open(t)
				guard := tc.guard
				if guard == nil {
					guard = allowAllGuard{}
				}
				m, aud := newRelTriggerCheckManager(t, st, tc.mode, guard)
				ctx := context.Background()
				err := tc.write(ctx, m)
				require.Equal(t, tc.want, propOf(t, st, tc.from, "", tc.prop))
				switch tc.outcome {
				case relCheckSaved:
					require.NoError(t, err)
					return
				case relCheckInvalid:
					var invalid *entitymanager.ValidationError
					require.ErrorAs(t, err, &invalid)
				case relCheckForbidden:
					var forbidden *acl.ForbiddenError
					require.ErrorAs(t, err, &forbidden)
					require.Equal(t, "transition-guard", forbidden.Decision.RuleKind)
				}
				ops := relCheckOps(aud)
				if tc.outcome == relCheckForbidden {
					require.Equal(t, 1, relCheckCount(ops, audit.OpDeniedWrite), "ops: %v", ops)
				}
				if tc.rel == "" || !store.TxRollsBack(st) {
					return
				}
				_, getErr := st.GetRelation(ctx, entity.RelationKey{From: tc.from, Type: tc.rel, To: "U-1"})
				if tc.relKept {
					require.NoError(t, getErr)
				} else {
					require.ErrorIs(t, getErr, store.ErrNotFound)
				}
				for _, op := range []string{audit.OpCreateRelation, audit.OpDeleteRelation, audit.OpUpdateEntity} {
					require.Zero(t, relCheckCount(ops, op), "a rolled-back %s was recorded: %v", op, ops)
				}
			})
		}
	}
}

func relCheckOps(aud *audit.Memory) []string {
	recs := aud.Records()
	ops := make([]string, 0, len(recs))
	for _, rec := range recs {
		ops = append(ops, rec.Op)
	}
	return ops
}

func relCheckCount(ops []string, op string) int {
	return len(slices.DeleteFunc(slices.Clone(ops), func(o string) bool { return o != op }))
}
