package entitymanager_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/aclstore"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// relTriggerPolicy: writer may update tasks and both doc faces; linker may
// create tasks but not update them; granted holds only the relation grant
// for in_project; drafter may update the draft face of a doc only.
const relTriggerPolicy = `
roles:
  writer:
    read: ["*"]
    create: [task, "doc@draft", "doc@published"]
    update: [task, "doc@draft", "doc@published"]
  linker:
    read: ["*"]
    create: [task]
  granted:
    read: ["*"]
    permissions: [link-projects]
  drafter:
    read: ["*"]
    create: ["doc@draft", "doc@published"]
    update: ["doc@draft"]
assignments:
  writer: writer
  linker: linker
  granted: granted
  drafter: drafter
relation_grants:
  in_project:
    create: link-projects
`

// TestRelationTriggers_SetNeedsUpdateAuthorization pins that a trigger's
// property write is authorized as the caller's own update of the row it
// writes. A caller who may make the relation but not update the source is
// refused, and the relation is not kept on a store that rolls back.
func TestRelationTriggers_SetNeedsUpdateAuthorization(t *testing.T) {
	mentioned := entity.RelationKey{From: "D-1", Type: "mentions", To: "P-1"}
	tests := []struct {
		name    string
		user    string
		key     entity.RelationKey
		allowed bool
		// written is the source rows a write that is allowed sets prop on.
		prop    string
		written []entity.Face
	}{
		{"a caller who may update the source", "writer", taskKey("in_project", "P-1"), true, "linked", []entity.Face{""}},
		{"a caller who may only create the source type", "linker", taskKey("in_project", "P-1"), false, "", nil},
		{"a caller with only the relation grant", "granted", taskKey("in_project", "P-1"), false, "", nil},
		{"a caller who may update every face", "writer", mentioned, true, "cited", []entity.Face{"draft", "published"}},
		{"a face the caller may read but not update", "drafter", mentioned, false, "", nil},
	}
	for _, b := range relTriggerBackends {
		for _, tc := range tests {
			t.Run(b.name+"/"+tc.name, func(t *testing.T) {
				st := b.open(t)
				p, err := acl.LoadPolicyBytes([]byte(relTriggerPolicy))
				require.NoError(t, err)
				d, err := acl.NewDeclarative(p, aclstore.MustNewGraph(st), st)
				require.NoError(t, err)
				m, _ := newRelTriggerManagerWith(t, st, d)
				ctx := entitymanager.BindAs(asUser(tc.user), t, m)
				_, err = m.CreateRelation(ctx, tc.key, entity.RelationOptions{})
				_, getErr := st.GetRelation(context.Background(), tc.key)
				if tc.allowed {
					require.NoError(t, err)
					require.NoError(t, getErr)
					for _, face := range tc.written {
						require.NotNil(t, propOf(t, st, tc.key.From, face, tc.prop), "face %q", face)
					}
					return
				}
				var forbidden *acl.ForbiddenError
				require.ErrorAs(t, err, &forbidden)
				if store.TxRollsBack(st) {
					require.ErrorIs(t, getErr, store.ErrNotFound)
				}
			})
		}
	}
}
