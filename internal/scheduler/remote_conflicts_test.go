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
	require.JSONEq(t, `"Theirs"`, string(list[0].RemoteValue))

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
		require.Equal(t, push, deps.RelationMetaGated != nil)
	}
}

const supersededConflicts = `{"version":1,"conflicts":[{"local":"REQ-1","type":"requirement",` +
	`"address":"requirement/R1","field":"title","remote_field":"title","reason":"superseded","rule":"latest",` +
	`"since":"2026-11-11T00:00:00Z","seen":"2026-11-11T00:00:00Z","remote_value":"Theirs","local_digest":"a",` +
	`"remote_digest":"b","superseded":{"winner":"remote","local_value":"Ours",` +
	`"local_updated":"2026-11-11T00:00:00Z","remote_updated":"2026-11-11T00:01:00Z"}}]}`

func TestDismissRemote(t *testing.T) {
	t.Parallel()
	ctx := principal.With(context.Background(), principal.Principal{User: "alice", Tool: principal.ToolCLI})
	ws := newRemoteWorkspace(t, "secretvalue")

	_, err := DismissRemote(ctx, nil, "cat", "REQ-1", "", discardLogger())
	require.Error(t, err)
	_, err = DismissRemote(ctx, ws, "cat", "REQ-1", "", nil)
	require.Error(t, err)
	_, err = DismissRemote(ctx, ws, "other", "REQ-1", "", discardLogger())
	require.ErrorContains(t, err, `no remote "other"`)

	// A held entry is not dismissed, and the refusal takes no run.
	require.NoError(t, ws.State().Put(ctx, "remotesync/cat/conflicts.json", []byte(heldConflicts)))
	_, err = DismissRemote(ctx, ws, "cat", "REQ-1", "title", discardLogger())
	require.Error(t, err)
	_, ok := taskState(t, ws, "remote:cat")
	require.False(t, ok)

	require.NoError(t, ws.State().Put(ctx, "remotesync/cat/conflicts.json", []byte(supersededConflicts)))
	n, err := DismissRemote(ctx, ws, "cat", "REQ-1", "", discardLogger())
	require.NoError(t, err)
	require.Equal(t, 1, n)
	list, err := RemoteConflicts(ctx, ws, "cat")
	require.NoError(t, err)
	require.Empty(t, list)
	ts, ok := taskState(t, ws, "remote:cat")
	require.True(t, ok)
	require.Nil(t, ts.Active)
	require.Zero(t, ts.Failures)
}

func TestPurgeRemoteSuperseded(t *testing.T) {
	t.Parallel()
	ctx := principal.With(context.Background(), principal.Principal{User: "alice", Tool: principal.ToolCLI})
	ws := newRemoteWorkspace(t, "secretvalue")

	_, err := PurgeRemoteSuperseded(ctx, nil, "REQ-1", discardLogger())
	require.Error(t, err)
	_, err = PurgeRemoteSuperseded(ctx, ws, "REQ-1", nil)
	require.Error(t, err)

	// No entry of the entity: nothing dropped, but the run is recorded.
	n, err := PurgeRemoteSuperseded(ctx, ws, "REQ-1", discardLogger())
	require.NoError(t, err)
	require.Zero(t, n)

	require.NoError(t, ws.State().Put(ctx, "remotesync/cat/conflicts.json", []byte(supersededConflicts)))
	n, err = PurgeRemoteSuperseded(ctx, ws, "REQ-2", discardLogger())
	require.NoError(t, err)
	require.Zero(t, n)
	n, err = PurgeRemoteSuperseded(ctx, ws, "REQ-1", discardLogger())
	require.NoError(t, err)
	require.Equal(t, 1, n)
	list, err := RemoteConflicts(ctx, ws, "cat")
	require.NoError(t, err)
	require.Empty(t, list)
	ts, ok := taskState(t, ws, "remote:cat")
	require.True(t, ok)
	require.Nil(t, ts.Active)
	require.Zero(t, ts.Failures)
}
