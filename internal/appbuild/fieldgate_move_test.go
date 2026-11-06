package appbuild_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/appbuild/backendtest"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/script"
)

const moveGateMetamodel = `version: "1.0"
entities:
  task:
    label: Task
    plural: tasks
    id_prefix: "TASK-"
    id_type: sequential
    properties:
      title: {type: string}
relations:
  precedes:
    label: Precedes
    from: [task]
    to: [task]
    orderable: both
`

// alice may write the incoming order key of a precedes edge only when the
// edge's source is titled "open".
const moveGatePolicy = `roles:
  editor:
    read: [task]
    create: [task]
    update: [task]
    relations:
      task:
        - relation: precedes
          fields:
            - field: _order_in
              when: "entity.title == 'open'"
assignments:
  alice: editor
`

// TestFieldGate_IncomingMoveJudgesEverySource: on the field-gated handle,
// which MCP on rela-server writes through, an incoming move that rewrites
// another source's edge needs the order-key grant for that source too
// (BUG-FQZ5Q1L). The policy is compiled from acl.yaml.
func TestFieldGate_IncomingMoveJudgesEverySource(t *testing.T) {
	tests := []struct {
		name        string
		otherTitle  string
		wantRefused bool
	}{
		{name: "other source grants the order key", otherTitle: "open", wantRefused: false},
		{name: "other source refuses the order key", otherTitle: "locked", wantRefused: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			writeMetamodelBody(t, root, moveGateMetamodel)
			writePolicy(t, root, moveGatePolicy)
			svc, err := appbuild.Discover(root, script.NewEngine(), backendtest.Options(t)...)
			require.NoError(t, err)
			t.Cleanup(func() { _ = svc.Close() })

			ctx := asAlice(t, svc)
			mine := createTask(t, svc, "open")
			other := createTask(t, svc, tc.otherTitle)
			target := createTask(t, svc, "open")
			keys := []entity.RelationKey{
				{From: mine, Type: "precedes", To: target},
				{From: other, Type: "precedes", To: target},
			}
			for _, k := range keys {
				_, cErr := svc.EntityManager().CreateRelation(ctx, k, entity.RelationOptions{})
				require.NoError(t, cErr)
			}
			before := incomingOrders(ctx, t, svc, keys)

			mgr := entitymanager.FieldGated(svc.EntityManager())
			_, err = mgr.UpdateRelation(ctx, keys[0], entity.RelationOptions{
				Position: &entity.OrderPosition{Step: 1, Incoming: true},
			})
			after := incomingOrders(ctx, t, svc, keys)
			if tc.wantRefused {
				require.Error(t, err)
				require.Contains(t, err.Error(), metamodel.OrderPropertyIn)
				require.Equal(t, before, after, "a refused move wrote order values")
				return
			}
			require.NoError(t, err)
			require.NotEqual(t, before, after, "an allowed move left the order unchanged")
		})
	}
}

func incomingOrders(ctx context.Context, t *testing.T, svc *appbuild.Services, keys []entity.RelationKey) []any {
	t.Helper()
	out := make([]any, len(keys))
	for i, k := range keys {
		r, err := svc.Store().GetRelation(ctx, k)
		require.NoError(t, err)
		out[i] = r.Properties[metamodel.OrderPropertyIn]
	}
	return out
}
