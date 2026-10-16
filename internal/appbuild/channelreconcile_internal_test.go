package appbuild

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// An entity removed outside the manager and put back the same way, as a git
// checkout or stash does with its file, gets its channels back. When live
// channels occupy the id by then, the old batch stays tombstoned and is
// forgotten.
func TestChannelReconciler_RestoresOnExternalCreate(t *testing.T) {
	ctx := context.Background()
	f := newGuardFixture(t)
	f.seed(t, "inc-a", "draft")
	f.seed(t, "inc-b", "draft", "final")
	f.seed(t, "inc-c", "draft")
	appendTo(t, f.cs, chanKey("inc-a", "", "chat"))
	appendTo(t, f.cs, chanKey("inc-b", "final", "notes"))
	appendTo(t, f.cs, chanKey("inc-c", "", "chat"))
	detached := newExternalDetaches(externalDetachCap)
	stop := startChannelReconciler(f.st, f.cs, detached)
	t.Cleanup(stop)

	_, err := f.st.DeleteFamily(ctx, "inc-a", true)
	require.NoError(t, err)
	_, err = f.st.DeleteFace(ctx, entity.Ref{ID: "inc-b", Face: "final"})
	require.NoError(t, err)
	_, err = f.st.DeleteFamily(ctx, "inc-c", true)
	require.NoError(t, err)
	require.Eventually(t, func() bool {
		return len(entityKeys(t, f.cs, "inc-a")) == 0 && len(entityKeys(t, f.cs, "inc-b")) == 0 &&
			len(entityKeys(t, f.cs, "inc-c")) == 0
	}, 5*time.Second, 10*time.Millisecond)

	// Another face coming back does not restore a face batch.
	f.seed(t, "inc-b", "other")
	f.seed(t, "inc-a", "draft")
	f.seed(t, "inc-b", "final")
	appendTo(t, f.cs, chanKey("inc-c", "", "chat"))
	f.seed(t, "inc-c", "draft")
	require.Eventually(t, func() bool {
		return len(entityKeys(t, f.cs, "inc-a")) == 1 && len(entityKeys(t, f.cs, "inc-b")) == 1 &&
			detached.size() == 0
	}, 5*time.Second, 10*time.Millisecond)
	assert.Equal(t, []string{"inc-a/chat"}, entityKeys(t, f.cs, "inc-a"))
	assert.Equal(t, []string{"inc-b@final/notes"}, entityKeys(t, f.cs, "inc-b"))
	assert.Equal(t, []string{"inc-c/chat"}, entityKeys(t, f.cs, "inc-c"), "the occupant stays alone")
}

// A create through the manager forgets the batches for its id: the entity
// is new, and the alias hook clears the id for it.
func TestChannelAliases_CreateForgetsExternalDetaches(t *testing.T) {
	ctx := context.Background()
	f := newGuardFixture(t)
	appendTo(t, f.cs, chanKey("inc-a", "", "chat"))
	h, err := f.cs.Tombstone(ctx, "inc-a", "", time.Now())
	require.NoError(t, err)
	detached := newExternalDetaches(externalDetachCap)
	detached.add("inc-a", h)

	f.seed(t, "inc-a", "draft")
	a := channelAliasesFor(f.cs, f.st, detached)
	require.NoError(t, a.EntityCreated(ctx, "inc-a", "draft"))
	assert.Zero(t, detached.size())

	r := channelReconciler{store: f.st, channels: f.cs, now: time.Now, detached: detached}
	r.restore(ctx, "inc-a", "draft")
	assert.Empty(t, entityKeys(t, f.cs, "inc-a"), "the new entity does not get the old channels")
}

// The ledger holds a bounded number of ids and drops the oldest.
func TestExternalDetaches_Bounded(t *testing.T) {
	d := newExternalDetaches(2)
	stamp := channels.TombstoneStamp(time.Now())
	for _, id := range []string{"a", "b", "a", "c"} {
		d.add(id, channels.NewTombstoneHandle(channels.Anchor{ID: id}, stamp))
	}
	assert.Equal(t, 2, d.size())
	assert.NotContains(t, d.byID, "a", "the oldest id went")
	assert.Len(t, d.byID["b"], 1)
	assert.Len(t, d.byID["c"], 1)
	d.add("x", channels.TombstoneHandle{})
	assert.Equal(t, 2, d.size(), "a zero handle is nothing to remember")
	d.forget("b")
	assert.Equal(t, []string{"c"}, d.order)

	var none *externalDetaches
	none.add("a", channels.NewTombstoneHandle(channels.Anchor{ID: "a"}, stamp))
	none.forget("a")
	none.restore(context.Background(), "a", "", nil)
	assert.Zero(t, none.size())
}

// size reports how many ids d remembers.
func (d *externalDetaches) size() int {
	if d == nil {
		return 0
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	return len(d.byID)
}

// countingMarks counts the soft-delete mark reads.
type countingMarks struct {
	*memstore.MemStore
	reads *int
}

func (s countingMarks) SoftDelete() store.SoftDeleter {
	return countingSoftDeleter{SoftDeleter: s.MemStore.SoftDelete(), reads: s.reads}
}

type countingSoftDeleter struct {
	store.SoftDeleter
	reads *int
}

func (s countingSoftDeleter) ListMarked(ctx context.Context) ([]store.MarkedEntity, error) {
	*s.reads++
	return s.SoftDeleter.ListMarked(ctx)
}

// The reconciler reads the channels before the marks, so a delete of an id
// without channels costs no mark read; and a mark is found under case
// folding, as the stores compare ids.
func TestChannelReconciler_ChannelsFirstAndCaseFolded(t *testing.T) {
	ctx := context.Background()
	f := newGuardFixture(t)
	reads := 0
	st := countingMarks{MemStore: f.st, reads: &reads}
	r := channelReconciler{store: st, channels: f.cs, now: time.Now}
	require.NoError(t, r.reconcile(ctx, "inc-z", ""))
	assert.Zero(t, reads)

	f.seed(t, "inc-c", "draft")
	appendTo(t, f.cs, chanKey("inc-c", "", "chat"))
	mgr := f.manager(t, entitymanager.NopAnchorGuard{})
	_, err := entitymanager.SoftDeleteEntity(ctx, mgr, "inc-c")
	require.NoError(t, err)
	marked, err := isMarked(ctx, st, "INC-C")
	require.NoError(t, err)
	assert.True(t, marked)
	assert.Equal(t, 1, reads)
}
