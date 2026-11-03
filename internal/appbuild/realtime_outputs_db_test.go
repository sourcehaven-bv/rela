//go:build sqlite || postgres

package appbuild_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/realtime"
	"github.com/Sourcehaven-BV/rela/internal/realtimecfg"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// TestOutputs_CreateExactlyOnce pins the create of a session output where
// the channel store joins the entity store's transaction: a progress
// write that fails rolls back the entity and its relation, and a post
// then creates exactly one entity, recorded in the mark it committed with.
func TestOutputs_CreateExactlyOnce(t *testing.T) {
	svc, _ := outputsServices(t, setAndCreate)
	w, err := appbuild.OutputsFor(svc, true)
	require.NoError(t, err)

	// A zero progress was not made by the service, so applying it fails
	// inside the transaction.
	_, err = w.Write(promoteAs(t, svc, "alice"), realtime.OutputRequest{
		Target: realtime.OutputTarget{Kind: realtimecfg.OutputCreate, AnchorID: "INC-001", Type: "tl", Relate: "logs"},
		Props:  map[string]any{"title": "orphan"}, Origin: "session:t",
	})
	f, ok := errors.AsType[*realtime.OutputError](err)
	require.Truef(t, ok, "want an *OutputError, got %v", err)
	require.True(t, f.WroteNothing)
	require.Zero(t, timelines(t, svc))
	n := 0
	for _, err := range svc.Store().ListRelations(context.Background(), store.RelationQuery{Type: "logs"}) {
		require.NoError(t, err)
		n++
	}
	require.Zero(t, n, "the relation is rolled back with the entity")

	res, err := postAssess(t, svc, "alice", map[string]any{"risk": "high"})
	require.NoError(t, err)
	require.Equal(t, realtime.OutputsDone, res.Outputs.Status)
	require.Equal(t, 1, timelines(t, svc))
	require.NotNil(t, res.Outputs.Results[1].Entity)
}
