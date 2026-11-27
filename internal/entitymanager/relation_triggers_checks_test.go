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

const relTriggerChecksSchema = `
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
      title: {type: string}
  project:
    label: Project
    id_prefix: "P-"
    properties:
      title: {type: string}
relations:
  owned_by: {from: [task], to: [person]}
  in_project: {from: [task], to: [project]}
  tagged: {from: [task], to: [project]}
validations:
  - name: done-needs-owner
    description: A done task has an owner.
    entity_type: task
    enforce: write
    when: ["status=done"]
    then_condition: "related(entity, 'owned_by')"
`

const relTriggerChecksFlows = `
triggers:
  finish-on-project:
    on: {relation_created: in_project}
    outputs:
      - set: {properties: {status: done}}
  count-from-title:
    on: {relation_created: tagged}
    outputs:
      - set: {properties: {count: "{{new.title}}"}}
`

func newRelTriggerChecksManager(
	t *testing.T, st store.Store, mode entitymanager.ValidationMode,
) *entitymanager.Manager {
	t.Helper()
	meta, err := metamodel.Parse([]byte(relTriggerChecksSchema))
	require.NoError(t, err)
	set, err := transitions.CompileTransitions(meta)
	require.NoError(t, err)
	rules, err := triggers.Parse([]byte(relTriggerChecksFlows), meta)
	require.NoError(t, err)
	engine, err := triggers.New(meta, rules)
	require.NoError(t, err)
	runner, err := autocascade.New(autocascade.Deps{Engine: engine})
	require.NoError(t, err)
	mgr, err := entitymanager.New(entitymanager.Deps{
		Store: st, Meta: meta, Templater: nopTemplater{}, Audit: audit.NewMemory(),
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
		{ID: "U-1", Type: "person", Properties: map[string]any{"title": "Owner"}},
		{ID: "P-1", Type: "project", Properties: map[string]any{"title": "Proj"}},
	} {
		require.NoError(t, st.CreateEntity(ctx, e))
	}
	return mgr
}

// TestRelationTriggers_SetRunsUpdateChecks pins that a relation trigger's
// set meets the checks of an ordinary update (#2636): a write-enforced
// relation rule, and validation errors as the deployment's mode decides.
func TestRelationTriggers_SetRunsUpdateChecks(t *testing.T) {
	cases := []struct {
		name    string
		mode    entitymanager.ValidationMode
		owner   bool
		rel     string
		prop    string
		wantErr bool
		want    any
	}{
		{"relation rule refuses done without owner", entitymanager.ValidationAdvisory, false,
			"in_project", "status", true, nil},
		{"relation rule admits done with owner", entitymanager.ValidationAdvisory, true,
			"in_project", "status", false, "done"},
		{"strict mode refuses a type error", entitymanager.ValidationStrict, false,
			"tagged", "count", true, nil},
		{"advisory mode keeps a type error as a warning", entitymanager.ValidationAdvisory, false,
			"tagged", "count", false, "notanumber"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			st := memstore.New()
			m := newRelTriggerChecksManager(t, st, tc.mode)
			ctx := context.Background()
			if tc.owner {
				_, err := m.CreateRelation(ctx,
					entity.RelationKey{From: "T-1", Type: "owned_by", To: "U-1"}, entity.RelationOptions{})
				require.NoError(t, err)
			}
			_, err := m.CreateRelation(ctx,
				entity.RelationKey{From: "T-1", Type: tc.rel, To: "P-1"}, entity.RelationOptions{})
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tc.want, propOf(t, st, "T-1", "", tc.prop))
		})
	}
}
