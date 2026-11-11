package scheduler

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/remotesync"
)

const heldConflicts = `{"version":1,"conflicts":[{"local":"REQ-1","type":"requirement","address":"requirement/R1",` +
	`"field":"title","remote_field":"title","reason":"both_changed","rule":"hold","since":"2026-11-11T00:00:00Z",` +
	`"seen":"2026-11-11T00:00:00Z","remote_value":"Theirs","local_digest":"a","remote_digest":"b"}]}`

func TestRemoteConflictsAndResolve(t *testing.T) {
	t.Parallel()
	ctx := principal.With(context.Background(), principal.Principal{User: "alice", Tool: principal.ToolCLI})
	bind := func(ctx context.Context) (context.Context, error) { return ctx, nil }
	ws := newRemoteWorkspace(t, "secretvalue")

	_, err := RemoteConflicts(ctx, nil, "cat")
	require.Error(t, err)
	_, err = RemoteConflicts(ctx, ws, "other")
	require.ErrorContains(t, err, `no remote "other"`)
	list, err := RemoteConflicts(ctx, ws, "cat")
	require.NoError(t, err)
	require.Empty(t, list)

	require.NoError(t, ws.State().Put(ctx, "remotesync/cat/conflicts.json", []byte(heldConflicts)))
	list, err = RemoteConflicts(ctx, ws, "cat")
	require.NoError(t, err)
	require.Len(t, list, 1)
	require.Equal(t, "Theirs", list[0].RemoteValue)

	for _, tc := range []struct {
		ws   WorkspaceProvider
		bind IdentityBinder
		nilL bool
	}{{nil, bind, false}, {ws, nil, false}, {ws, bind, true}} {
		logger := discardLogger()
		if tc.nilL {
			logger = nil
		}
		_, _, err = ResolveRemote(ctx, tc.ws, tc.bind, "cat", "REQ-1", "", remotesync.TakeLocal, logger)
		require.Error(t, err)
	}

	// A resolve that names nothing fails before it takes the lock, so it
	// does not count as a run.
	_, _, err = ResolveRemote(ctx, ws, bind, "cat", "REQ-9", "", remotesync.TakeLocal, discardLogger())
	require.ErrorIs(t, err, remotesync.ErrNoConflict)
	_, _, err = ResolveRemote(ctx, ws, bind, "cat", "REQ-1", "", "both", discardLogger())
	require.Error(t, err)
	_, ok := taskState(t, ws, "remote:cat")
	require.False(t, ok)

	// A resolve applies, then syncs; nothing listens on the source's port,
	// so the sync fails and the run records it.
	n, _, err := ResolveRemote(ctx, ws, bind, "cat", "REQ-1", "title", remotesync.TakeLocal, discardLogger())
	require.Error(t, err)
	require.Equal(t, 1, n)
	list, err = RemoteConflicts(ctx, ws, "cat")
	require.NoError(t, err)
	require.Empty(t, list)
	ts, _ := taskState(t, ws, "remote:cat")
	require.Nil(t, ts.Active)
	require.Equal(t, 1, ts.Failures)
}

func TestRemoteDeps_PushRemoteGetsAPusher(t *testing.T) {
	t.Parallel()
	ws := newRemoteWorkspace(t, "secretvalue")
	for _, push := range []bool{false, true} {
		var extra []string
		if push {
			extra = []string{"push: true"}
		}
		remotes, err := remotesync.Parse(remotesYAML("", extra...), remoteMeta(t))
		require.NoError(t, err)
		deps, err := remoteDeps(ws, remotes.Mappings["cat"], "r", nil, discardLogger(), func(string) string { return "" })
		require.NoError(t, err)
		require.Equal(t, push, deps.Pusher != nil)
	}
}
