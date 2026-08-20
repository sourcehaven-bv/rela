package fsstore_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/fsstore"
)

// stateObserver records every observer callback with the state pointer
// (for puts/renames) so the tests can assert the Step-1 skip.
type stateObserver struct {
	puts    []string
	deletes []string
	renames []string
}

func (o *stateObserver) EntityPut(e *entity.Entity) error {
	o.puts = append(o.puts, e.ID+"|"+string(e.Pointer))
	return nil
}
func (o *stateObserver) EntityDelete(id string) error {
	o.deletes = append(o.deletes, id)
	return nil
}
func (o *stateObserver) EntityRenamed(oldID string, renamed *entity.Entity) error {
	o.renames = append(o.renames, oldID+"->"+renamed.ID+"|"+string(renamed.Pointer))
	return nil
}

func openStoreWithObserver(t *testing.T, fs *storage.MemFS, o *stateObserver) *fsstore.FSStore {
	t.Helper()
	cfg := newConfig(fs)
	cfg.Observers = []store.EntityObserver{o}
	s, err := fsstore.New(cfg)
	require.NoError(t, err)
	return s
}

// TestObservers_SkipNonDefaultStates is fsstore's mirror of the
// memstore pin (TKT-DOFYR1, RR-8U1PE2): observers — the search
// indexers — key documents by bare id, so non-default states must not
// reach them through ANY notify path: put, update, or rename.
func TestObservers_SkipNonDefaultStates(t *testing.T) {
	fs := storage.NewMemFS()
	ctx := context.Background()
	obs := &stateObserver{}
	s := openStoreWithObserver(t, fs, obs)
	defer func() { require.NoError(t, s.Close()) }()

	def := entity.New("REQ-1", "requirement")
	require.NoError(t, s.CreateEntity(ctx, def))
	draft := entity.New("REQ-1", "requirement")
	draft.Pointer = mustPtr(t, "draft")
	require.NoError(t, s.CreateEntity(ctx, draft))
	draft.Properties = map[string]any{"title": "edited"}
	require.NoError(t, s.UpdateEntity(ctx, draft))

	assert.Equal(t, []string{"REQ-1|"}, obs.puts,
		"only the default face reaches observers")

	// A family rename notifies with the DEFAULT face only.
	_, err := s.RenameEntity(ctx, "REQ-1", "REQ-2")
	require.NoError(t, err)
	assert.Equal(t, []string{"REQ-1->REQ-2|"}, obs.renames)
}

// TestObservers_HeadlessRenameSkips pins the review-found leak: a
// headless family (state file on disk with no default — tolerated by
// the load path) has NO default face, so a rename must notify NO
// observer rather than handing them a state that would overwrite the
// bare-id search document.
func TestObservers_HeadlessRenameSkips(t *testing.T) {
	fs := storage.NewMemFS()
	ctx := context.Background()

	// Hand-write a headless state file — the write path rejects this
	// shape, but the load path tolerates it (design doc §6).
	require.NoError(t, fs.MkdirAll("/entities/requirements", 0o755))
	require.NoError(t, fs.WriteFile("/entities/requirements/REQ-9@draft.md",
		[]byte("---\nid: REQ-9\ntype: requirement\ntitle: headless draft\n---\n"), 0o644))

	obs := &stateObserver{}
	s := openStoreWithObserver(t, fs, obs)
	defer func() { require.NoError(t, s.Close()) }()

	// The headless state loads (tolerance) and is addressable.
	got, err := s.GetEntityState(ctx, "REQ-9", mustPtr(t, "draft"))
	require.NoError(t, err)
	assert.Equal(t, "REQ-9", got.ID)

	_, err = s.RenameEntity(ctx, "REQ-9", "REQ-10")
	require.NoError(t, err)
	assert.Empty(t, obs.renames, "a headless family has no default face to hand observers")
	assert.Empty(t, obs.puts)

	// The rename itself still happened.
	_, err = s.GetEntityState(ctx, "REQ-10", mustPtr(t, "draft"))
	require.NoError(t, err)
}
