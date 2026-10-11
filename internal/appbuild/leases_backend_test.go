package appbuild_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/appbuild/backendtest"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/lease"
	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// Every build wires a lease backend that outlives the Services over it: the
// KV document in the cache directory on fs, memory and sqlite (sqlite keeps
// leases out of rela.db), and the database on postgres (storeLeasesFor). A
// lease claimed through one Services is seen by the next one over the same
// project, which the in-process fallback would not do.
//
// The two Services run one after the other because sqlite admits one process
// per database file.
func TestLeases_BackendOutlivesServices(t *testing.T) {
	root := t.TempDir()
	writeMetamodelBody(t, root, `version: "1.0"
entities:
  ticket:
    label: Ticket
    plural: tickets
    id_type: manual
    properties:
      title:
        type: string
relations: {}
`)
	dsn := backendtest.DSN(t)
	ctx := principal.With(context.Background(), principal.Principal{User: "alice", Tool: principal.ToolCLI})

	first, err := newOnDisk(t, dsn, root)
	require.NoError(t, err)
	_, err = first.EntityManager().CreateEntity(ctx, entity.New("", "ticket"), entity.CreateOptions{ID: "T-1"})
	require.NoError(t, err)
	claimed, err := appbuild.Leases(first).Claim(ctx, lease.ClaimRequest{Addr: "T-1", Status: "triage"})
	require.NoError(t, err)
	require.NoError(t, first.Close())

	second, err := newOnDisk(t, dsn, root)
	require.NoError(t, err)
	defer second.Close()
	got, err := appbuild.Leases(second).GetReadable(context.Background(), []string{"T-1"})
	require.NoError(t, err)
	require.Contains(t, got, "T-1", "the lease survives the Services that claimed it")
	require.Equal(t, "triage", got["T-1"].Status)
	require.Equal(t, claimed.Lease.Token, got["T-1"].Token)
}
