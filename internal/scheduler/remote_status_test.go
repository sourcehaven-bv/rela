package scheduler

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/remotestatus"
	"github.com/Sourcehaven-BV/rela/internal/remotesync"
	"github.com/Sourcehaven-BV/rela/internal/schedulerstate"
)

// auditedWorkspace is a remote workspace with an audit sink.
type auditedWorkspace struct {
	remoteWorkspace
	sink *audit.Memory
}

func (w auditedWorkspace) Audit() audit.Audit { return w.sink }

func newAuditedWorkspace(t *testing.T) auditedWorkspace {
	t.Helper()
	return auditedWorkspace{remoteWorkspace: newRemoteWorkspace(t, "secretvalue"), sink: audit.NewMemory()}
}

const uncertainConflicts = `{"version":1,"conflicts":[{"local":"REQ-1","type":"requirement",` +
	`"reason":"uncertain","cause":"timeout","rule":"hold","since":"2026-11-11T00:00:00Z",` +
	`"seen":"2026-11-11T00:00:00Z","refused_fields":{"title":"d1"}}]}`

func TestRemoteStatuses(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	_, err := RemoteStatuses(ctx, nil)
	require.Error(t, err)

	ws := newRemoteWorkspace(t, "secretvalue")
	got, err := RemoteStatuses(ctx, ws)
	require.NoError(t, err)
	require.Len(t, got, 1)
	st := got[0]
	require.Equal(t, "cat", st.Remote.Name)
	require.Equal(t, "http://127.0.0.1:1", st.Remote.Settings.URL)
	require.Equal(t, remotesync.ConflictHold, st.Remote.Settings.Conflict)
	require.True(t, st.Remote.Run.LastRun.IsZero())
	require.True(t, st.Remote.Run.NextRun.IsZero(), "no run yet: a pull is due now")
	require.Equal(t, []remotestatus.MappedType{{
		Local: "requirement", Source: "requirement",
		Fields: []remotestatus.MappedField{{Local: "title", Remote: "title", Rule: remotesync.ConflictHold}},
	}}, st.Remote.Types)
	require.Empty(t, st.Linked)
	require.Empty(t, st.Conflicts)

	require.NoError(t, ws.State().Put(ctx, "remotesync/cat/conflicts.json", []byte(heldConflicts)))
	run, err := startManualRun(ctx, ws.SchedulerState(), "remote:cat", "r1")
	require.NoError(t, err)
	_, err = ws.SchedulerState().FinishRun(ctx, run.ID, schedulerstate.Outcome{At: time.Now()}, retryDelay)
	require.NoError(t, err)

	got, err = RemoteStatuses(ctx, ws)
	require.NoError(t, err)
	st = got[0]
	require.Len(t, st.Conflicts, 1)
	require.False(t, st.Remote.Run.LastRun.IsZero())
	require.Equal(t, st.Remote.Run.LastRun.Add(time.Hour), st.Remote.Run.NextRun)
	require.False(t, st.Remote.Run.Running)
}

func TestRemoteStatuses_NoRemotes(t *testing.T) {
	t.Parallel()
	ws := newMockWorkspace(t)
	got, err := RemoteStatuses(context.Background(), ws)
	require.NoError(t, err)
	require.Empty(t, got)
}

func TestScheduleNext(t *testing.T) {
	t.Parallel()
	last := time.Date(2026, 11, 11, 15, 30, 0, 0, time.UTC) // a Wednesday
	for _, tc := range []struct {
		raw  string
		want time.Time
	}{
		{"day", time.Date(2026, 11, 12, 0, 0, 0, 0, time.UTC)},
		{"monday", time.Date(2026, 11, 16, 0, 0, 0, 0, time.UTC)},
		{"wednesday", time.Date(2026, 11, 18, 0, 0, 0, 0, time.UTC)},
		{"2h", last.Add(2 * time.Hour)},
	} {
		t.Run(tc.raw, func(t *testing.T) {
			t.Parallel()
			s, err := parseSchedule(tc.raw)
			require.NoError(t, err)
			require.Equal(t, tc.want, s.Next(last))
			require.False(t, s.IsDue(last, tc.want.Add(-time.Second)))
			require.True(t, s.IsDue(last, tc.want))
		})
	}
}

func TestRemoteEntryActions_Audited(t *testing.T) {
	t.Parallel()
	ctx := principal.With(context.Background(), principal.Principal{User: "alice", Tool: principal.ToolDataEntry})
	ws := newAuditedWorkspace(t)

	require.NoError(t, ws.State().Put(ctx, "remotesync/cat/conflicts.json", []byte(heldConflicts)))
	_, err := ResolveRemoteEntry(ctx, ws, "cat", remotesync.Target{Local: "REQ-9"}, remotesync.TakeLocal,
		discardLogger())
	require.ErrorIs(t, err, remotesync.ErrNoConflict)
	require.Empty(t, ws.sink.Records())

	n, err := ResolveRemoteEntry(ctx, ws, "cat", remotesync.Target{Local: "REQ-1", Field: "title"},
		remotesync.TakeRemote, discardLogger())
	require.NoError(t, err)
	require.Equal(t, 1, n)
	ts, _ := taskState(t, ws, "remote:cat")
	require.True(t, ts.LastRun.IsZero(), "a resolve without a sync does not count as a pull")

	require.NoError(t, ws.State().Put(ctx, "remotesync/cat/conflicts.json", []byte(supersededConflicts)))
	_, err = DismissRemote(ctx, ws, "cat", remotesync.Target{Local: "REQ-1"}, discardLogger())
	require.NoError(t, err)

	require.NoError(t, ws.State().Put(ctx, "remotesync/cat/conflicts.json", []byte(uncertainConflicts)))
	require.ErrorIs(t, LinkRemote(ctx, ws, "cat", remotesync.Target{Local: "REQ-2"}, "requirement/R1", discardLogger()), remotesync.ErrNoConflict)
	require.Error(t, LinkRemote(ctx, ws, "cat", remotesync.Target{Local: "REQ-1"}, "cat/R1", discardLogger()))
	require.NoError(t, LinkRemote(ctx, ws, "cat", remotesync.Target{Local: "REQ-1"}, "requirement/R1", discardLogger()))

	recs := ws.sink.Records()
	require.Len(t, recs, 3)
	want := []struct{ op, summary string }{
		{audit.OpRemoteResolve, "remote=cat field=title entries=1 take=remote"},
		{audit.OpRemoteDismiss, "remote=cat entries=1"},
		{audit.OpRemoteLink, "remote=cat address=requirement/R1"},
	}
	for i, w := range want {
		require.Equal(t, w.op, recs[i].Op)
		require.Equal(t, w.summary, recs[i].Summary)
		require.Equal(t, &audit.Subject{Kind: "entity", Type: "requirement", ID: "REQ-1"}, recs[i].Subject)
		require.Equal(t, "alice", recs[i].Principal.User)
		require.NotContains(t, recs[i].Summary, "Theirs")
		require.NotContains(t, recs[i].Summary, "Ours")
	}

	linked, err := RemoteStatuses(ctx, ws)
	require.NoError(t, err)
	require.Equal(t, []remotesync.Linked{{Type: "requirement", Local: "REQ-1"}}, linked[0].Linked)
	require.Empty(t, linked[0].Conflicts)
}

func TestRemoteSubject_Relation(t *testing.T) {
	t.Parallel()
	list := []remotesync.Conflict{{Kind: remotesync.KindRelation, Local: "REQ-1--refines--REQ-2", Type: "refines"}}
	require.Equal(t, &audit.Subject{Kind: "relation", RelationType: "refines", FromID: "REQ-1", ToID: "REQ-2"},
		remoteSubject(list, "REQ-1--refines--REQ-2"))
	require.Nil(t, remoteSubject(list, "REQ-3"))
}
