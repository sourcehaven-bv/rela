package mcpwire

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/mcp"
	"github.com/Sourcehaven-BV/rela/internal/piles"
	"github.com/Sourcehaven-BV/rela/internal/piles/kvpiles"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/state"
	"github.com/Sourcehaven-BV/rela/internal/storage"
)

func TestPileFuncs_MapsService(t *testing.T) {
	rfs, err := storage.NewRootedFS(storage.NewMemFS(), "/")
	require.NoError(t, err)
	backend, err := kvpiles.New(state.NewFSKV(rfs), kvpiles.ProcessPrivate{})
	require.NoError(t, err)
	svc, err := piles.NewService(backend, piles.Options{})
	require.NoError(t, err)
	f := pileFuncs(svc)
	ctx := principal.With(context.Background(), principal.Principal{User: "alice", Tool: principal.ToolMCP})
	ref := entity.Ref{ID: "DOC-1", Face: entity.Face("draft")}

	added, err := f.PushToPile(ctx, mcp.PilePush{Pile: "Inbox", Refs: []entity.Ref{ref}, Create: true})
	require.NoError(t, err)
	require.Equal(t, 1, added)

	list, err := f.ListPiles(ctx)
	require.NoError(t, err)
	require.Len(t, list, 1)
	require.Equal(t, []entity.Ref{ref}, list[0].Items)

	byName, err := f.FindPile(ctx, "inbox")
	require.NoError(t, err)
	byID, err := f.FindPile(ctx, list[0].ID)
	require.NoError(t, err, "FindPile must also take a pile id")
	require.Equal(t, byName, byID)

	require.NoError(t, f.RemoveFromPile(ctx, byID.ID, []entity.Ref{ref}))
	after, err := f.FindPile(ctx, byID.ID)
	require.NoError(t, err)
	require.Empty(t, after.Items)

	_, err = f.FindPile(ctx, "missing")
	require.ErrorIs(t, err, piles.ErrNotFound)
}
