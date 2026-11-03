package appbuild

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/realtime"
	"github.com/Sourcehaven-BV/rela/internal/realtimecfg"
)

// testTimers returns a timer loop over cs for a worker fixture.
func testTimers(t *testing.T, cs channels.Store, cfg realtime.ConfigSource, m realtime.ChangeMarker) *realtime.Timers {
	t.Helper()
	timers, err := realtime.NewTimers(realtime.TimerDeps{Store: cs, Config: cfg, Marker: m, Clock: time.Now})
	require.NoError(t, err)
	return timers
}

// testOutputs returns a session outputs runner over cs for a worker
// fixture; its writer refuses every output.
func testOutputs(t *testing.T, cs channels.Store, cfg realtime.ConfigSource, local *realtime.Local) *realtime.Outputs {
	t.Helper()
	o, err := realtime.NewOutputs(realtime.OutputsDeps{
		Store: cs, Config: cfg, Writer: realtime.NoOutputs{}, Local: local, Clock: time.Now,
	})
	require.NoError(t, err)
	return o
}

// TestRealtimeWorker_RequiresTimers pins that the worker refuses parts
// without the timer loop or the wake.
func TestRealtimeWorker_RequiresTimers(t *testing.T) {
	f := newWorkerFixture(t)
	p := realtimeWorkerParts{
		sweeper: f.w.sweeper, triggers: f.w.triggers, timers: f.w.timers, outputs: f.w.outputs, dueWake: f.w.dueWake,
		changes: f.w.changes, lock: f.w.lock, wake: f.w.wake,
	}
	_, err := newRealtimeWorker(p)
	require.NoError(t, err)
	noTimers := p
	noTimers.timers = nil
	_, err = newRealtimeWorker(noTimers)
	require.Error(t, err)
	noWake := p
	noWake.dueWake = nil
	_, err = newRealtimeWorker(noWake)
	require.Error(t, err)
	noOutputs := p
	noOutputs.outputs = nil
	_, err = newRealtimeWorker(noOutputs)
	require.Error(t, err)
}

// guardTimerRealtime declares a timer type on the guard schema's incident.
const guardTimerRealtime = `version: 1
channel_types:
  watch:
    attach_to: [incident]
    roles:
      owner: { from: entity.owner }
    message_types:
      text: { properties: { body: { type: string } }, post: [owner] }
    session:
      initial: waiting
      states:
        waiting: { after: { soon: { period: 1m, to: done } } }
        done:    { final: true }
`

// TestChannelGuard_RestoreNotesDueWake pins that the undo of a guard's
// detach notes the resumed due time on the shared DueWake.
func TestChannelGuard_RestoreNotesDueWake(t *testing.T) {
	f := newGuardFixture(t)
	cfg, err := realtimecfg.Parse([]byte(guardTimerRealtime), f.meta, nil)
	require.NoError(t, err)
	f.guard.config = func() *realtimecfg.Compiled { return cfg }
	wake := realtime.NewDueWake()
	f.guard.dueWake = wake
	k := chanKey("INC-1", "", "watch")
	appendTo(t, f.cs, k)

	undo, err := f.guard.BeforeRename(t.Context(), "INC-1", func(context.Context) error { return nil })
	require.NoError(t, err)
	require.NotNil(t, undo)
	require.True(t, wake.Take().IsZero())
	require.NoError(t, undo.Restore(t.Context()))
	h, err := f.cs.Head(t.Context(), k)
	require.NoError(t, err)
	require.Equal(t, h.CreatedAt.Add(time.Minute), wake.Take(), "the restore notes the resumed due time")
}
