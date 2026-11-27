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
)

// relTriggerCheckSchema has a relation rule enforced on write (a done task
// has an owner) and an integer property that strict mode guards.
const relTriggerCheckSchema = `
version: "1.0"
entities:
  task:
    label: Task
    id_prefix: "T-"
    properties:
      title: {type: string}
      status: {type: string}
      count: {type: integer}
  person:
    label: Person
    id_prefix: "U-"
    properties:
      name: {type: string}
relations:
  owned_by: {from: [task], to: [person]}
  finishes: {from: [task], to: [person]}
  counts: {from: [task], to: [person]}
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
  count-from-title:
    on: {relation_created: counts}
    outputs:
      - set: {properties: {count: "{{new.title}}"}}
`

func newRelTriggerCheckManager(
	t *testing.T, st store.Store, mode entitymanager.ValidationMode,
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
		ACL: acl.NopACL{}, Transitions: set,
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
		{ID: "T-1", Type: "task", Properties: map[string]any{"title": "notanumber"}},
		{ID: "T-2", Type: "task", Properties: map[string]any{"title": "Owned"}},
		{ID: "U-1", Type: "person", Properties: map[string]any{"name": "Ann"}},
	} {
		require.NoError(t, st.CreateEntity(ctx, e))
	}
	_, err = st.CreateRelation(ctx, entity.RelationKey{From: "T-2", Type: "owned_by", To: "U-1"}, nil)
	require.NoError(t, err)
	return mgr, aud
}

// TestRelationTriggers_RunUpdateChecks pins that a relation trigger's
// property write runs the checks [entitymanager.Manager.PatchEntity] runs:
// a relation rule enforced on write and strict validation mode. A refused
// write fails the relation write, and a store that rolls back keeps
// neither the relation nor any audit record. Each refused trigger case has
// a patch case beside it that is refused the same way.
func TestRelationTriggers_RunUpdateChecks(t *testing.T) {
	tests := []struct {
		name    string
		mode    entitymanager.ValidationMode
		from    string
		relType string // empty: patch the property directly instead
		patch   map[string]any
		prop    string
		refused bool
		want    any
	}{
		{name: "rule refuses trigger", from: "T-1", relType: "finishes", prop: "status", refused: true},
		{name: "rule refuses patch", from: "T-1", patch: map[string]any{"status": "done"},
			prop: "status", refused: true},
		{name: "rule passes trigger with owner", from: "T-2", relType: "finishes", prop: "status", want: "done"},
		{name: "strict refuses trigger", mode: entitymanager.ValidationStrict, from: "T-1", relType: "counts",
			prop: "count", refused: true},
		{name: "strict refuses patch", mode: entitymanager.ValidationStrict, from: "T-1",
			patch: map[string]any{"count": "notanumber"}, prop: "count", refused: true},
		{name: "advisory saves trigger", from: "T-1", relType: "counts", prop: "count", want: "notanumber"},
	}
	for _, b := range relTriggerBackends {
		for _, tc := range tests {
			t.Run(b.name+"/"+tc.name, func(t *testing.T) {
				st := b.open(t)
				m, aud := newRelTriggerCheckManager(t, st, tc.mode)
				ctx := context.Background()
				key := entity.RelationKey{From: tc.from, Type: tc.relType, To: "U-1"}
				var err error
				if tc.relType == "" {
					_, err = m.PatchEntity(ctx, tc.from, entity.Patch{Properties: tc.patch})
				} else {
					_, err = m.CreateRelation(ctx, key, entity.RelationOptions{})
				}
				if !tc.refused {
					require.NoError(t, err)
					require.Equal(t, tc.want, propOf(t, st, tc.from, "", tc.prop))
					return
				}
				var invalid *entitymanager.ValidationError
				require.ErrorAs(t, err, &invalid)
				require.Nil(t, propOf(t, st, tc.from, "", tc.prop))
				if tc.relType == "" || !store.TxRollsBack(st) {
					return
				}
				_, getErr := st.GetRelation(ctx, key)
				require.ErrorIs(t, getErr, store.ErrNotFound)
				require.Empty(t, aud.Records())
			})
		}
	}
}
