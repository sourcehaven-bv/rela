package entitymanager_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// denyCreateACL hides DEC-1 like hideOneACL and refuses every create of
// one entity type.
type denyCreateACL struct {
	hideOneACL
	typ string
}

func (d denyCreateACL) AuthorizeWrite(_ context.Context, req acl.WriteRequest) acl.Decision {
	if s, ok := req.Subject.(acl.EntitySubject); ok && req.Op == acl.OpCreate && s.Type() == d.typ {
		return acl.Decision{Allow: false, Reason: "denied in test"}
	}
	return acl.Decision{Allow: true}
}

// batchOf is a batch entity of typ with a title.
func batchOf(typ, id, title string) entitymanager.BatchEntity {
	e := entity.New("", typ)
	e.SetString("title", title)
	return entitymanager.BatchEntity{Entity: e, Options: entity.CreateOptions{ID: id}}
}

// TestCreateBatch_HiddenEndIsMissing pins at the engine that an existing
// end the caller cannot read is refused exactly like an id that does not
// exist, as a tail and as a head, and that nothing is written.
func TestCreateBatch_HiddenEndIsMissing(t *testing.T) {
	t.Parallel()
	for name, rel := range map[string]func(id string) entitymanager.BatchRelation{
		"head": func(id string) entitymanager.BatchRelation {
			return entitymanager.BatchRelation{From: local(0), Type: "depends-on", To: existing(id)}
		},
		"tail": func(id string) entitymanager.BatchRelation {
			return entitymanager.BatchRelation{From: existing(id), Type: "addresses", To: local(1)}
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			mgr, st := writeBlindManager(t, hiding)
			run := func(id string) error {
				_, err := entitymanager.CreateBatch(writer(), mgr, entitymanager.Batch{
					Entities: []entitymanager.BatchEntity{
						batchOf("component", "svc", "svc"), batchOf("requirement", "", "r"),
					},
					Relations: []entitymanager.BatchRelation{rel(id)},
				})
				return err
			}
			sameAsMissing(t, name, run("DEC-1"), run("DEC-404"), entitymanager.ErrEntityNotFound)
			_, err := st.GetEntity(context.Background(), entity.Ref{ID: "svc"})
			require.ErrorIs(t, err, store.ErrNotFound, "nothing is written")
		})
	}
}

// TestCreateBatch_DeniedCreateWritesNothing pins that a create the ACL
// refuses in the middle of a batch ends it before any write, with the
// denial audited once and no success record.
func TestCreateBatch_DeniedCreateWritesNothing(t *testing.T) {
	t.Parallel()
	sink := audit.NewMemory()
	mgr, st := writeBlindManagerAudit(t, func(store.Store) acl.ACL {
		return denyCreateACL{typ: "requirement"}
	}, sink)
	_, err := entitymanager.CreateBatch(writer(), mgr, entitymanager.Batch{
		Entities: []entitymanager.BatchEntity{
			batchOf("component", "svc", "svc"),
			batchOf("requirement", "", "r"),
			batchOf("component", "web", "web"),
		},
	})
	require.ErrorIs(t, err, acl.ErrForbidden)
	op, ok := errors.AsType[*entitymanager.BatchOpError](err)
	require.True(t, ok)
	require.Equal(t, entitymanager.BatchOpEntity, op.Kind)
	require.Equal(t, 1, op.Index)
	var denied, success int
	for _, r := range sink.Records() {
		if r.Op == audit.OpDeniedWrite {
			denied++
		} else {
			success++
		}
	}
	require.Equal(t, 1, denied)
	require.Zero(t, success)
	_, gErr := st.GetEntity(context.Background(), entity.Ref{ID: "svc"})
	require.ErrorIs(t, gErr, store.ErrNotFound)
}
