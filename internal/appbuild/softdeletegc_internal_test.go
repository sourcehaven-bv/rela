package appbuild

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/jobs"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/statemachine"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
	"github.com/Sourcehaven-BV/rela/internal/templating"
)

type gcNopTemplater struct{}

func (gcNopTemplater) EntityTemplate(context.Context, string, string) (*templating.Template, error) {
	return nil, nil //nolint:nilnil // no template is not an error
}

func (gcNopTemplater) RelationTemplate(context.Context, string) (*templating.Template, error) {
	return nil, nil //nolint:nilnil // no template is not an error
}

// TestSoftDeleteGC_PurgesAfterTheDelay runs the GC through a real queue: an
// entity inside the undo window survives the ticks, and one past it is purged.
func TestSoftDeleteGC_PurgesAfterTheDelay(t *testing.T) {
	meta, err := metamodel.Parse([]byte(`version: "1.0"
entities:
  note:
    label: Note
    plural: notes
    id_prefix: "N-"
    id_type: sequential
    properties:
      title: {type: string}
`))
	require.NoError(t, err)
	st := memstore.New()
	sink := audit.NewMemory()
	mgr, err := entitymanager.New(entitymanager.Deps{
		Store: st, Meta: meta, Templater: gcNopTemplater{}, Audit: sink,
		ACL: acl.NopACL{}, Transitions: statemachine.EmptySet(), FieldGate: entitymanager.AllowAllFieldGate{},
	})
	require.NoError(t, err)

	q, err := jobs.NewMemoryQueue(context.Background(), slog.Default())
	require.NoError(t, err)
	require.NoError(t, q.Start(context.Background()))
	t.Cleanup(func() { closeJobQueue(q) })

	ctx := context.Background()
	res, err := mgr.CreateEntity(ctx, entity.New("", "note"), entity.CreateOptions{})
	require.NoError(t, err)
	id := res.Entity.ID
	_, err = entitymanager.SoftDeleteEntity(ctx, mgr, id)
	require.NoError(t, err)

	const delay = time.Second
	stop := startSoftDeleteGC(mgr, q, delay, 20*time.Millisecond)
	t.Cleanup(stop)

	time.Sleep(delay / 5)
	_, found, err := entitymanager.FindSoftDeleted(ctx, mgr, id)
	require.NoError(t, err)
	assert.True(t, found, "still inside the undo window")

	// The audit record is written after the purge, so wait for it.
	purged := func() bool {
		for _, r := range sink.Records() {
			if r.Op == audit.OpPurgeDeletedEntity {
				return true
			}
		}
		return false
	}
	require.Eventually(t, purged, 5*time.Second, 10*time.Millisecond)
	_, found, err = entitymanager.FindSoftDeleted(ctx, mgr, id)
	require.NoError(t, err)
	assert.False(t, found)
	_, err = st.GetEntity(ctx, id)
	require.ErrorIs(t, err, store.ErrNotFound)
}

func TestSoftDeleteGC_NothingToStartWithoutSupport(_ *testing.T) {
	startSoftDeleteGC(nil, nil, time.Second, time.Second)()
}
