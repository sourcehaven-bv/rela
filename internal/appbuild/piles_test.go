package appbuild_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/piles"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/search"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

func aliceCtx() context.Context {
	return principal.With(context.Background(), principal.Principal{User: "alice", Tool: principal.ToolCLI})
}

// Every build has a piles backend, and piles outlive the Services that wrote
// them: a second build over the same project (and, on postgres, the same
// schema) reads what the first created.
func TestPiles_WiredAndDurable(t *testing.T) {
	root := writeMinimalProject(t)
	build := discoverer(t)

	first, err := build(root)
	require.NoError(t, err)
	require.NotNil(t, first.Piles())
	_, err = first.Piles().Create(aliceCtx(), piles.CreateRequest{
		Name: "Inbox", Refs: []entity.Ref{{ID: "DOC-1", Face: entity.Face("draft")}},
	})
	require.NoError(t, err)
	first.Close()

	second, err := build(root)
	require.NoError(t, err)
	defer second.Close()
	got, err := second.Piles().ByName(aliceCtx(), "inbox")
	require.NoError(t, err, "a pile must survive a rebuild")
	require.Len(t, got.Items, 1)
}

// A re-assembly shares its predecessor's piles BACKEND, so a pile written
// through one is read through the other, but builds a fresh service, so the
// owner check follows the reloaded ACL (RR-SAYJ9F).
func TestPiles_ReassemblySharesStoreNotService(t *testing.T) {
	base := newSharedBase(t)
	st := memstore.New()
	origin, err := base.Assemble(st, search.New(st, search.NewLinearSearch()), nil, nil)
	require.NoError(t, err)
	defer origin.Close()
	successor, err := base.ForReassembly(origin).Assemble(
		origin.Store(), origin.Searcher(), origin.VisibleSearcher(), nil)
	require.NoError(t, err)
	defer successor.CloseAssembly()

	require.NotNil(t, origin.Piles())
	require.NotNil(t, successor.Piles())
	require.NotSame(t, origin.Piles(), successor.Piles(), "the service is rebuilt per assembly")

	created, err := origin.Piles().Create(aliceCtx(), piles.CreateRequest{Name: "Inbox"})
	require.NoError(t, err)
	got, err := successor.Piles().Get(aliceCtx(), created.ID)
	require.NoError(t, err, "the successor reads the same backend")
	require.Equal(t, "Inbox", got.Name)
}

// On the single-process tiers piles go to the node-local cache directory,
// never into a database: the sqlite build's rela.db is shipped to others. The
// postgres build keeps them in the database (pgpiles) and writes no file.
func TestPiles_KVBackendWritesCacheDir(t *testing.T) {
	root := writeMinimalProject(t)
	svc, err := discover(t, root)
	require.NoError(t, err)
	defer svc.Close()
	_, err = svc.Piles().Create(aliceCtx(), piles.CreateRequest{Name: "Inbox"})
	require.NoError(t, err)

	_, err = os.Stat(filepath.Join(svc.Paths().CacheDir, "piles.json"))
	if postgresBuild {
		require.ErrorIs(t, err, os.ErrNotExist, "the postgres build must not write piles to disk")
		return
	}
	require.NoError(t, err, "piles must be written to the node-local cache directory")
}

// A store without a piles backend of its own: on the single-process builds
// piles go to the cache directory under the cross-process lock file. On the
// postgres build that directory is not scoped to the schema, so a non-pgstore
// gets an in-memory backend instead and nothing is written (RR-NVOXIC).
func TestPiles_NonDatabaseStoreFallback(t *testing.T) {
	base := newSharedBase(t)
	st := memstore.New()
	svc, err := base.Assemble(st, search.New(st, search.NewLinearSearch()), nil, nil)
	require.NoError(t, err)
	defer svc.Close()
	_, err = svc.Piles().Create(aliceCtx(), piles.CreateRequest{Name: "Inbox"})
	require.NoError(t, err)

	cache := svc.Paths().CacheDir
	_, docErr := os.Stat(filepath.Join(cache, "piles.json"))
	_, lockErr := os.Stat(filepath.Join(cache, "piles.lock"))
	if postgresBuild {
		require.ErrorIs(t, docErr, os.ErrNotExist, "the postgres build must not share a cache-directory document")
		require.ErrorIs(t, lockErr, os.ErrNotExist)
		return
	}
	require.NoError(t, docErr)
	require.NoError(t, lockErr, "the cache-directory backend locks across processes")
}
