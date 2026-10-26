package filechannels

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/filelock"
	"github.com/Sourcehaven-BV/rela/internal/storage"
)

// jobMark is the mark of a job move: written as run_as, recorded on the
// notice by its Mark.
func jobMark(at time.Time, timer string) func(channels.SessionMove) *channels.Action {
	return func(channels.SessionMove) *channels.Action {
		return &channels.Action{PendingAt: at, Outputs: &channels.OutputMark{
			Writer: channels.Author{User: "system:bot", Tool: "realtime"}, Job: timer == "",
		}}
	}
}

// crashSegment removes log segment n of key, as a crash between meta.yaml
// and the log records would leave it.
func crashSegment(t *testing.T, mfs *storage.MemFS, n string) {
	t.Helper()
	dir := "/channels/" + enc("TKT-1") + "/" + none + "/room/" + none + "/"
	require.NoError(t, mfs.Remove(dir+"log-"+n+".jsonl"))
}

// TestJobMark_SurvivesTimerCrash pins that a timer move whose notice
// carries a job mark keeps the mark through a crash between meta.yaml and
// the log: the runner's scan replays the notice, lists the mark, and
// clears the stash.
func TestJobMark_SurvivesTimerCrash(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	mfs := storage.NewMemFS()
	s, err := newStore(mfs, "/channels", filelock.ProcessPrivate{}, 2)
	require.NoError(t, err)
	_, err = s.Append(ctx, key, msg("one"))
	require.NoError(t, err)
	m := msg("two")
	m.Session = moveStep()
	_, err = s.Append(ctx, key, m)
	require.NoError(t, err)
	_, err = s.Append(ctx, key, msg("four"))
	require.NoError(t, err)
	at := m.At.Add(time.Hour)
	step := channels.SessionStep{
		Decide: func(channels.Channel) (channels.SessionMove, bool) {
			return channels.SessionMove{From: "contained", To: "overdue"}, true
		},
		NoticeType: "session", NoticeBy: channels.Author{User: "system:realtime"}, TriggeredBy: "session:room",
		AuditTriggeredBy: "timer:room/contained/late", Timer: "late",
		Due:  func(channels.Channel) channels.SessionSchedule { return channels.SessionSchedule{} },
		Mark: jobMark(at, "late"),
	}
	_, err = s.SetSession(ctx, key, at, step)
	require.NoError(t, err)
	// seq 1-2 sit in log-0000, the move's notice (3) and seq 4 in
	// log-0001, and the timer's notice (5) alone in log-0002.
	crashSegment(t, mfs, "0002")

	stale, err := s.StalePending(ctx, at.Add(time.Second))
	require.NoError(t, err)
	require.Len(t, stale, 1)
	require.Equal(t, int64(5), stale[0].Seq)
	got, err := s.Get(ctx, key, 5)
	require.NoError(t, err)
	require.NotNil(t, got.Action)
	require.True(t, got.Action.Outputs.Job)
	require.Equal(t, "late", got.Action.Outputs.Timer)

	m2, err := s.readMeta(key)
	require.NoError(t, err)
	require.Empty(t, m2.JobRecords, "the scan clears the stash")
	next, err := s.Append(ctx, key, msg("six"))
	require.NoError(t, err)
	require.Equal(t, int64(6), next.Seq)
}

// TestJobMark_SurvivesTriggerCrash pins the same for a trigger's post: the
// message and its notice are replayed by the next append, in order.
func TestJobMark_SurvivesTriggerCrash(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	mfs := storage.NewMemFS()
	s, err := newStore(mfs, "/channels", filelock.ProcessPrivate{}, 2)
	require.NoError(t, err)
	for _, body := range []string{"one", "two"} {
		_, err = s.Append(ctx, key, msg(body))
		require.NoError(t, err)
	}
	m := msg("trigger")
	m.Author = channels.Author{User: "system:realtime"}
	m.Session = moveStep()
	m.Session.Mark = jobMark(m.At, "")
	_, err = s.Append(ctx, key, m)
	require.NoError(t, err)
	crashSegment(t, mfs, "0001")

	next, err := s.Append(ctx, key, msg("three"))
	require.NoError(t, err)
	require.Equal(t, int64(5), next.Seq, "the replayed message and notice keep seqs 3 and 4")
	all, err := s.Messages(ctx, key, channels.Page{})
	require.NoError(t, err)
	require.Len(t, all, 5)
	require.True(t, all[2].MovedSession)
	require.Equal(t, "session", all[3].Type)
	require.True(t, all[3].Action.Outputs.Job)
	stale, err := s.StalePending(ctx, m.At.Add(time.Hour))
	require.NoError(t, err)
	require.Len(t, stale, 1)
	require.Equal(t, int64(4), stale[0].Seq)
}

// TestJobMark_StashIgnoredWhenLogHasIt pins that a stash whose records the
// log already holds is cleared without writing them again.
func TestJobMark_StashIgnoredWhenLogHasIt(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s, err := newStore(storage.NewMemFS(), "/channels", filelock.ProcessPrivate{}, 2)
	require.NoError(t, err)
	m := msg("trigger")
	m.Session = moveStep()
	m.Session.Mark = jobMark(m.At, "")
	_, err = s.Append(ctx, key, m)
	require.NoError(t, err)
	_, err = s.StalePending(ctx, m.At.Add(time.Hour))
	require.NoError(t, err)
	all, err := s.Messages(ctx, key, channels.Page{})
	require.NoError(t, err)
	require.Len(t, all, 2)
	md, err := s.readMeta(key)
	require.NoError(t, err)
	require.Empty(t, md.JobRecords)
}
