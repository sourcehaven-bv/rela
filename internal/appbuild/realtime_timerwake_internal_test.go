//go:build !postgres && !memorybackend && !sqlite

package appbuild

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/script"
	"github.com/Sourcehaven-BV/rela/internal/search"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

const timerWakeSchema = `version: "1.0"
entities:
  doc:
    label: Doc
    id_prefix: "DOC-"
    id_type: sequential
    properties:
      title: {type: string}
`

const timerWakeRealtime = `version: 1
channel_types:
  watch:
    attach_to: [doc]
    roles:
      member: { from: attached }
    message_types:
      text: { properties: { body: { type: string } }, post: [member] }
    session:
      initial: waiting
      states:
        waiting: { after: { soon: { period: 1m, to: done } } }
        done:    { final: true }
`

// timerWakeServices assembles a long-running process over a project with
// a timer type, as a server host does.
func timerWakeServices(t *testing.T) *Services {
	t.Helper()
	root := t.TempDir()
	for name, body := range map[string]string{"schema.yaml": timerWakeSchema, "realtime.yaml": timerWakeRealtime} {
		require.NoError(t, os.WriteFile(filepath.Join(root, name), []byte(body), 0o600))
	}
	for _, dir := range []string{".rela", "entities", "relations"} {
		require.NoError(t, os.MkdirAll(filepath.Join(root, dir), 0o750))
	}
	fs := storage.NewSafeFS(storage.NewOsFS())
	paths, err := project.Discover(root, fs)
	require.NoError(t, err)
	base, err := NewSharedBase(Config{
		FS: fs, Paths: paths, ScriptEngine: script.NewEngine(), Audit: audit.Nop{},
	}, WithBackgroundAutomationJobs())
	require.NoError(t, err)
	st := memstore.New()
	svc, err := base.Assemble(st, search.New(st, search.NewLinearSearch()), nil, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = svc.Close() })
	return svc
}

// TestRealtimeWorker_TimerWake pins the wake path end to end: the service
// and the worker share one DueWake, and a due time noted on it makes the
// leading worker fire the timer long before the next ceiling round. The
// session is created 58 seconds in the past, so its one-minute timer comes
// due two seconds after the write; the worker's own schedule would not run
// a round for up to a minute.
func TestRealtimeWorker_TimerWake(t *testing.T) {
	svc := timerWakeServices(t)
	require.NotNil(t, svc.realtimeWorker)
	require.NotNil(t, svc.dueWake)
	require.Same(t, svc.dueWake, svc.realtimeWorker.dueWake, "the service and the worker share one DueWake")

	ctx, cancel := context.WithCancel(context.Background())
	done := RunRealtimeWorker(ctx, svc)
	require.Eventually(t, svc.realtimeWorker.leading.Load, 5*time.Second, 10*time.Millisecond)
	// Let the first round, over an empty store, finish before the write.
	time.Sleep(200 * time.Millisecond)

	created := time.Now().Add(-58 * time.Second)
	due := channels.NormTime(created.Add(time.Minute))
	k := channels.Key{Anchor: channels.Anchor{ID: "DOC-1"}, Type: "watch"}
	_, err := svc.channels.Append(ctx, k, channels.NewMessage{
		Type: "text", Author: channels.Author{User: "alice"}, At: created,
		Session: &channels.SessionStep{
			Decide:      func(channels.Channel) (channels.SessionMove, bool) { return channels.SessionMove{}, false },
			NoticeType:  "session",
			NoticeBy:    channels.Author{User: "system:realtime"},
			TriggeredBy: channels.SessionMarker + "watch",
			Due:         func(channels.Channel) channels.SessionSchedule { return channels.SessionSchedule{NextDueAt: due} },
		},
	})
	require.NoError(t, err)
	svc.dueWake.Note(due)

	require.Eventually(t, func() bool {
		h, err := svc.channels.Head(ctx, k)
		return err == nil && h.Session.State == "done"
	}, 15*time.Second, 50*time.Millisecond, "the wake fires the timer well before the one-minute ceiling")

	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("the worker, and its timer, stop when ctx is canceled")
	}
	require.False(t, svc.realtimeWorker.leading.Load())
}
