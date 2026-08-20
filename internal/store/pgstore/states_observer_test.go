package pgstore_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store/pgstore"
)

type recordingObserver struct {
	puts []string
}

func (r *recordingObserver) EntityPut(e *entity.Entity) error {
	r.puts = append(r.puts, e.ID+"|"+string(e.Pointer))
	return nil
}
func (r *recordingObserver) EntityDelete(string) error                  { return nil }
func (r *recordingObserver) EntityRenamed(string, *entity.Entity) error { return nil }

// TestObservers_SkipNonDefaultStates is pgstore's mirror of the fs/mem
// pins (TKT-DOFYR1, RR-8U1PE2): observers key search documents by bare
// id, so a non-default state write must not reach them. Three backends,
// one rule — this is the test the PR-B review demanded after finding
// pgstore silently diverging (the gap was latent only because the pg
// search observer happens to be a no-op today).
func TestObservers_SkipNonDefaultStates(t *testing.T) {
	pool := newScopedPool(t)
	obs := &recordingObserver{}
	s, err := pgstore.New(pool, pgstore.WithObserver(obs))
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	ctx := context.Background()

	def := entity.New("PAGE-1", "page")
	require.NoError(t, s.CreateEntity(ctx, def))

	draft := entity.New("PAGE-1", "page")
	p, err := entity.ParsePointer("draft")
	require.NoError(t, err)
	draft.Pointer = p
	require.NoError(t, s.CreateEntity(ctx, draft))
	draft.SetString("title", "edited")
	require.NoError(t, s.UpdateEntity(ctx, draft))

	assert.Equal(t, []string{"PAGE-1|"}, obs.puts,
		"only the default face reaches observers")
}
