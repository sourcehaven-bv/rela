//go:build !postgres && !memorybackend && !sqlite

package appbuild

import (
	"context"
	"errors"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/realtime"
	"github.com/Sourcehaven-BV/rela/internal/realtimecfg"
	"github.com/Sourcehaven-BV/rela/internal/script"
	"github.com/Sourcehaven-BV/rela/internal/search"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

const runAsSchema = `version: "1.0"
entities:
  doc:
    label: Doc
    id_prefix: "DOC-"
    id_type: sequential
    properties:
      title: {type: string}
      status: {type: string}
      escalated: {type: boolean}
      owner: {type: string}
`

// runAsPolicy grants system:breach-bot (and its successor -2) update on
// docs with a fields block; owner is only writable while the title is
// "never". root administers docs.
const runAsPolicy = `roles:
  breach-bot:
    read: [doc]
    update: [doc]
    fields:
      doc:
        - field: status
        - field: escalated
        - field: owner
          when: "entity.title == 'never'"
  admin:
    read: [doc]
    create: [doc]
    update: [doc]
assignments:
  system:breach-bot: breach-bot
  system:breach-bot-2: breach-bot
  root: admin
`

// runAsRealtime declares watch on docs. Its soon timer moves waiting to
// late and sets escalated; give-up moves late to the final done and sets
// status; a trigger's escalate moves waiting to escalated and sets status
// from the message; a trigger's wrap moves escalated to done with no
// outputs. on_final closes the channel.
func runAsRealtime(runAs string) string {
	return `version: 1
channel_types:
  watch:
    attach_to: [doc]
    roles:
      member: { from: attached }
    message_types:
      text:     { properties: { body: { type: string } }, post: [member] }
      escalate: { properties: { note: { type: string } }, post: [member] }
      wrap:     { properties: { note: { type: string } }, post: [member] }
    session:
      initial: waiting
      on_final: close
      run_as: ` + runAs + `
      states:
        waiting:
          on:
            escalate:
              to: escalated
              by: trigger
              outputs:
                - set: { properties: { status: "{{message.note}}" } }
          after:
            soon:
              period: 1m
              to: late
              outputs:
                - set: { properties: { escalated: true } }
        escalated:
          on:
            wrap: { to: done, by: trigger }
        late:
          after:
            give-up:
              period: 1m
              to: done
              outputs:
                - set: { properties: { status: given-up } }
        done: { final: true }
triggers:
  escalate:
    entity_type: doc
    on: enter
    when: "entity.status == 'hot'"
    channel: watch
    post: { type: escalate, props: { note: escalated } }
  wrap:
    entity_type: doc
    on: enter
    when: "entity.status == 'cold'"
    channel: watch
    post: { type: wrap, props: { note: done } }
`
}

// runAsEnv is an assembled long-running process over the run_as project,
// with a memory audit sink.
type runAsEnv struct {
	svc   *Services
	root  string
	audit *audit.Memory
}

func runAsServices(t *testing.T, files map[string]string) runAsEnv {
	t.Helper()
	root := t.TempDir()
	all := map[string]string{
		"schema.yaml": runAsSchema, "acl.yaml": runAsPolicy, "realtime.yaml": runAsRealtime("system:breach-bot"),
	}
	maps.Copy(all, files)
	for name, body := range all {
		if body == "" {
			continue
		}
		require.NoError(t, os.WriteFile(filepath.Join(root, name), []byte(body), 0o600))
	}
	for _, dir := range []string{".rela", "entities", "relations"} {
		require.NoError(t, os.MkdirAll(filepath.Join(root, dir), 0o750))
	}
	fs := storage.NewSafeFS(storage.NewOsFS())
	paths, err := project.Discover(root, fs)
	require.NoError(t, err)
	sink := audit.NewMemory()
	base, err := NewSharedBase(Config{
		FS: fs, Paths: paths, ScriptEngine: script.NewEngine(), Audit: sink,
	}, WithBackgroundAutomationJobs())
	require.NoError(t, err)
	st := memstore.New()
	svc, err := base.Assemble(st, search.New(st, search.NewLinearSearch()), nil, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = svc.Close() })
	return runAsEnv{svc: svc, root: root, audit: sink}
}

// bound is ctx stamped with user and bound to the policy.
func (e runAsEnv) bound(t *testing.T, user string) context.Context {
	t.Helper()
	ctx, err := acl.BindRequest(principal.With(context.Background(),
		principal.Principal{User: user, Tool: principal.ToolDataEntry}), e.svc.aclDeclarative)
	require.NoError(t, err)
	return ctx
}

// createDoc creates DOC-001 as root.
func (e runAsEnv) createDoc(t *testing.T, props map[string]any) {
	t.Helper()
	d := entity.New("", "doc")
	maps.Copy(d.Properties, props)
	_, err := e.svc.EntityManager().CreateEntity(e.bound(t, "root"), d, entity.CreateOptions{})
	require.NoError(t, err)
}

func (e runAsEnv) doc(t *testing.T) *entity.Entity {
	t.Helper()
	got, err := e.svc.Store().GetEntity(context.Background(), entity.Ref{ID: "DOC-001"})
	require.NoError(t, err)
	return got
}

// setStatus sets DOC-001's status as root.
func (e runAsEnv) setStatus(t *testing.T, status string) {
	t.Helper()
	_, err := e.svc.EntityManager().PatchEntity(e.bound(t, "root"), "DOC-001",
		entity.Patch{Properties: map[string]any{"status": status}})
	require.NoError(t, err)
}

var runAsKey = channels.Key{Anchor: channels.Anchor{ID: "DOC-001"}, Type: "watch"}

// fakeNow is a settable clock.
type fakeNow struct {
	mu  sync.Mutex
	now time.Time
}

func (c *fakeNow) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeNow) add(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

// runAsLoop is a timer loop and an outputs runner over env's stores, on a
// fake clock, as the worker builds them.
type runAsLoop struct {
	env     runAsEnv
	clock   *fakeNow
	timers  *realtime.Timers
	outputs *realtime.Outputs
}

func newRunAsLoop(t *testing.T, env runAsEnv) *runAsLoop {
	t.Helper()
	clock := &fakeNow{now: time.Now().UTC().Truncate(time.Second)}
	cfg := realtimeConfigSource{h: env.svc.realtime}
	timers, err := realtime.NewTimers(realtime.TimerDeps{
		Store: env.svc.channels, Config: cfg, Marker: env.svc.realtimeLocal, Clock: clock.Now,
	})
	require.NoError(t, err)
	w, err := newManagerOutputs(env.svc.entityManager, env.svc.store, env.svc.meta,
		txChannelsFor(env.svc.store, env.svc.channels), env.svc.aclDeclarative)
	require.NoError(t, err)
	outputs, err := realtime.NewOutputs(realtime.OutputsDeps{
		Store: env.svc.channels, Config: cfg, Writer: w, Local: env.svc.realtimeLocal, Clock: clock.Now,
	})
	require.NoError(t, err)
	return &runAsLoop{env: env, clock: clock, timers: timers, outputs: outputs}
}

// leaderCtx is the worker's context: system:realtime.
func leaderCtx() context.Context {
	return principal.With(context.Background(),
		principal.Principal{User: principal.UserRealtime, Tool: principal.ToolRealtime})
}

// open creates the watch channel of DOC-001 at the loop's clock, so its soon
// timer is due a minute later.
func (l *runAsLoop) open(t *testing.T) {
	t.Helper()
	created := l.clock.Now()
	_, err := l.env.svc.channels.Append(context.Background(), runAsKey, channels.NewMessage{
		Type: "text", Author: channels.Author{User: "alice"}, At: created,
		Session: &channels.SessionStep{
			Decide:      func(channels.Channel) (channels.SessionMove, bool) { return channels.SessionMove{}, false },
			NoticeType:  realtimecfg.SessionNoticeType,
			NoticeBy:    channels.Author{User: principal.UserRealtime},
			TriggeredBy: channels.SessionMarker + "watch",
			Due: func(channels.Channel) channels.SessionSchedule {
				return channels.SessionSchedule{NextDueAt: channels.NormTime(created.Add(time.Minute))}
			},
		},
	})
	require.NoError(t, err)
}

// fire advances the clock a minute and runs a timer round; it returns the
// marks the round wrote.
func (l *runAsLoop) fire(t *testing.T) int {
	t.Helper()
	l.clock.add(time.Minute)
	rep, err := l.timers.RunRound(leaderCtx())
	require.NoError(t, err)
	return rep.Marks
}

// run advances the clock a second and runs the outputs runner.
func (l *runAsLoop) run(t *testing.T) realtime.OutputsReport {
	t.Helper()
	l.clock.add(time.Second)
	rep, err := l.outputs.Run(leaderCtx())
	require.NoError(t, err)
	return rep
}

// marks returns the outputs marks of DOC-001's watch channel, oldest first.
func (e runAsEnv) marks(t *testing.T) []*channels.OutputMark {
	t.Helper()
	msgs, err := e.svc.channels.Messages(context.Background(), runAsKey, channels.Page{})
	require.NoError(t, err)
	var out []*channels.OutputMark
	for _, m := range msgs {
		if m.Action != nil && m.Action.Name == channels.OutputsAction {
			out = append(out, m.Action.Outputs)
		}
	}
	return out
}

// sessionRecords returns the graph audit records whose triggered_by is a
// session origin.
func (e runAsEnv) sessionRecords() []audit.Record {
	var out []audit.Record
	for _, r := range e.audit.Records() {
		if strings.HasPrefix(r.TriggeredBy, channels.SessionMarker) {
			out = append(out, r)
		}
	}
	return out
}

// TestOutputs_TimerSetAsRunAs pins a timer mark's run: the entity changes,
// and the audit record names run_as, tool realtime and the session origin.
func TestOutputs_TimerSetAsRunAs(t *testing.T) {
	env := runAsServices(t, nil)
	env.createDoc(t, map[string]any{"title": "Leak"})
	l := newRunAsLoop(t, env)
	l.open(t)
	require.Equal(t, 1, l.fire(t))
	require.Equal(t, 1, l.run(t).Ran)
	require.Equal(t, true, env.doc(t).Properties["escalated"])
	recs := env.sessionRecords()
	require.Len(t, recs, 1)
	require.Equal(t, audit.OpUpdateEntity, recs[0].Op)
	require.Equal(t, "system:breach-bot", recs[0].Principal.User)
	require.Equal(t, principal.ToolRealtime, recs[0].Principal.Tool)
	require.Equal(t, "session:DOC-001/watch/waiting->late#1/0", recs[0].TriggeredBy)
}

// TestOutputs_TimerFinalCloses pins a timer move into a final state on a
// type with on_final: close: the outputs run, then the channel closes.
func TestOutputs_TimerFinalCloses(t *testing.T) {
	env := runAsServices(t, nil)
	env.createDoc(t, map[string]any{"title": "Leak"})
	l := newRunAsLoop(t, env)
	l.open(t)
	l.fire(t)
	require.Equal(t, 1, l.fire(t))
	require.Equal(t, 2, l.run(t).Ran)
	require.Equal(t, "given-up", env.doc(t).Properties["status"])
	marks := env.marks(t)
	require.Len(t, marks, 2)
	last := marks[1]
	require.Len(t, last.Results, 2)
	require.Equal(t, channels.OutputDone, last.Results[0].Status)
	require.Equal(t, "close", last.Results[1].Kind)
	require.Equal(t, channels.OutputDone, last.Results[1].Status)
	h, err := env.svc.channels.Head(context.Background(), runAsKey)
	require.NoError(t, err)
	require.Equal(t, channels.StateClosed, h.State)
}

// TestOutputs_RunAsChanged pins a run_as change between the move and the
// run: every output fails run_as_changed and writes nothing, and the close
// still runs.
func TestOutputs_RunAsChanged(t *testing.T) {
	env := runAsServices(t, nil)
	env.createDoc(t, map[string]any{"title": "Leak"})
	l := newRunAsLoop(t, env)
	l.open(t)
	l.fire(t)
	l.fire(t)
	require.NoError(t, os.WriteFile(filepath.Join(env.root, "realtime.yaml"),
		[]byte(runAsRealtime("system:breach-bot-2")), 0o600))
	require.NoError(t, env.svc.realtime.Reload(context.Background()))
	require.Equal(t, 2, l.run(t).Ran)
	require.Nil(t, env.doc(t).Properties["escalated"])
	require.Nil(t, env.doc(t).Properties["status"])
	require.Empty(t, env.sessionRecords())
	marks := env.marks(t)
	require.Len(t, marks, 2)
	for _, m := range marks {
		require.Equal(t, channels.OutputFailed, m.Results[0].Status)
		require.Equal(t, realtime.OutputCodeRunAsChanged, m.Results[0].Code)
	}
	require.Equal(t, "close", marks[1].Results[1].Kind)
	require.Equal(t, channels.OutputDone, marks[1].Results[1].Status)
}

// TestOutputs_RunAsWriteReauthorized pins that the write asks the policy
// in force at the write, not the one the config was checked against: in a
// process whose acl.yaml no longer grants run_as the write is forbidden,
// and a field whose when: is false at the write is field_denied.
func TestOutputs_RunAsWriteReauthorized(t *testing.T) {
	env := runAsServices(t, nil)
	env.createDoc(t, map[string]any{"title": "Leak"})
	lost := runAsServices(t, map[string]string{
		// The bot still reads docs but lost update.
		"acl.yaml":      strings.Replace(runAsPolicy, "    read: [doc]\n    update: [doc]\n    fields:", "    read: [doc]\n    fields:", 1),
		"realtime.yaml": "",
	})
	lost.createDoc(t, map[string]any{"title": "Leak"})
	cases := []struct {
		name  string
		env   runAsEnv
		props map[string]any
		code  string
	}{
		{"grant lost", lost, map[string]any{"status": "x"}, realtime.OutputCodeForbidden},
		{"when false", env, map[string]any{"owner": "x"}, realtime.OutputCodeFieldDenied},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := tc.env
			w, err := newManagerOutputs(e.svc.entityManager, e.svc.store, e.svc.meta, nil, e.svc.aclDeclarative)
			require.NoError(t, err)
			_, err = w.Write(leaderCtx(), realtime.OutputRequest{
				Target: realtime.OutputTarget{Kind: realtimecfg.OutputSet, AnchorID: "DOC-001", AnchorType: "doc"},
				Props:  tc.props, Origin: "session:t", RunAs: "system:breach-bot",
			})
			f, ok := errors.AsType[*realtime.OutputError](err)
			require.Truef(t, ok, "want an *OutputError, got %v", err)
			require.Equal(t, tc.code, f.Code)
			require.Nil(t, e.doc(t).Properties["status"])
			require.Nil(t, e.doc(t).Properties["owner"])
		})
	}
}

// TestOutputs_UntypedAnchorRow pins the row read of a job target without an
// anchor type: the row's own type decides, and a missing row is not found.
func TestOutputs_UntypedAnchorRow(t *testing.T) {
	env := runAsServices(t, nil)
	env.createDoc(t, map[string]any{"title": "Leak"})
	docOnly := []string{"doc"}
	w, err := newManagerOutputs(env.svc.entityManager, env.svc.store, env.svc.meta, nil, env.svc.aclDeclarative)
	require.NoError(t, err)
	_, err = w.Write(leaderCtx(), realtime.OutputRequest{
		Target: realtime.OutputTarget{Kind: realtimecfg.OutputSet, AnchorID: "DOC-001", AttachTo: docOnly},
		Props:  map[string]any{"status": "untyped"}, Origin: "session:t", RunAs: "system:breach-bot",
	})
	require.NoError(t, err)
	require.Equal(t, "untyped", env.doc(t).Properties["status"])
	_, err = w.Write(leaderCtx(), realtime.OutputRequest{
		Target: realtime.OutputTarget{Kind: realtimecfg.OutputSet, AnchorID: "DOC-001", Face: "draft", AttachTo: docOnly},
		Props:  map[string]any{"status": "faced"}, Origin: "session:t", RunAs: "system:breach-bot",
	})
	require.NoError(t, err, "an unfaced row is read without the face a family set names")
	_, err = w.Write(leaderCtx(), realtime.OutputRequest{
		Target: realtime.OutputTarget{Kind: realtimecfg.OutputSet, AnchorID: "DOC-9", AttachTo: docOnly},
		Props:  map[string]any{"status": "x"}, Origin: "session:t", RunAs: "system:breach-bot",
	})
	f, ok := errors.AsType[*realtime.OutputError](err)
	require.Truef(t, ok, "want an *OutputError, got %v", err)
	require.Equal(t, realtime.OutputCodeNotFound, f.Code)

	// A row whose type is not one the channel attaches to is not the
	// anchor, as on the typed path.
	_, err = w.Write(leaderCtx(), realtime.OutputRequest{
		Target: realtime.OutputTarget{Kind: realtimecfg.OutputSet, AnchorID: "DOC-001", AttachTo: []string{"other"}},
		Props:  map[string]any{"status": "elsewhere"}, Origin: "session:t", RunAs: "system:breach-bot",
	})
	f, ok = errors.AsType[*realtime.OutputError](err)
	require.Truef(t, ok, "want an *OutputError, got %v", err)
	require.Equal(t, realtime.OutputCodeNotFound, f.Code)
	require.Equal(t, "faced", env.doc(t).Properties["status"])
}

// runWorker runs env's worker until the test ends.
func runWorker(t *testing.T, env runAsEnv) {
	t.Helper()
	require.NotNil(t, env.svc.realtimeWorker)
	ctx, cancel := context.WithCancel(context.Background())
	done := RunRealtimeWorker(ctx, env.svc)
	t.Cleanup(func() {
		cancel()
		<-done
	})
	require.Eventually(t, env.svc.realtimeWorker.leading.Load, 5*time.Second, 10*time.Millisecond)
	// Let the first rounds, which baseline the triggers, finish.
	time.Sleep(200 * time.Millisecond)
}

// TestRealtimeWorker_MarksWakeOutputs pins that a timer round that wrote a
// mark starts an outputs round at once: the set lands long before the
// minute tick that would otherwise run it.
func TestRealtimeWorker_MarksWakeOutputs(t *testing.T) {
	env := runAsServices(t, nil)
	env.createDoc(t, map[string]any{"title": "Leak"})
	runWorker(t, env)
	created := time.Now().Add(-58 * time.Second)
	due := channels.NormTime(created.Add(time.Minute))
	_, err := env.svc.channels.Append(context.Background(), runAsKey, channels.NewMessage{
		Type: "text", Author: channels.Author{User: "alice"}, At: created,
		Session: &channels.SessionStep{
			Decide:      func(channels.Channel) (channels.SessionMove, bool) { return channels.SessionMove{}, false },
			NoticeType:  realtimecfg.SessionNoticeType,
			NoticeBy:    channels.Author{User: principal.UserRealtime},
			TriggeredBy: channels.SessionMarker + "watch",
			Due:         func(channels.Channel) channels.SessionSchedule { return channels.SessionSchedule{NextDueAt: due} },
		},
	})
	require.NoError(t, err)
	env.svc.dueWake.Note(due)
	require.Eventually(t, func() bool {
		got, err := env.svc.Store().GetEntity(context.Background(), entity.Ref{ID: "DOC-001"})
		return err == nil && got.Properties["escalated"] == true
	}, 20*time.Second, 50*time.Millisecond, "the outputs run right after the timer round")
}

// TestOutputs_TriggerMoveWritesAsRunAs pins a trigger's post that fires a
// transition with outputs: the worker writes them as run_as, right after
// the trigger round.
func TestOutputs_TriggerMoveWritesAsRunAs(t *testing.T) {
	env := runAsServices(t, nil)
	env.createDoc(t, map[string]any{"title": "Leak"})
	runWorker(t, env)
	env.setStatus(t, "hot")
	require.Eventually(t, func() bool {
		got, err := env.svc.Store().GetEntity(context.Background(), entity.Ref{ID: "DOC-001"})
		return err == nil && got.Properties["status"] == "escalated"
	}, 20*time.Second, 50*time.Millisecond)
	recs := env.sessionRecords()
	require.Len(t, recs, 1)
	require.Equal(t, "system:breach-bot", recs[0].Principal.User)
	require.Equal(t, principal.ToolRealtime, recs[0].Principal.Tool)
	require.Equal(t, "session:DOC-001/watch/waiting->escalated#1/0", recs[0].TriggeredBy)
}

// TestOutputs_TriggerFinalCloseOnly pins a trigger move into a final state
// with no outputs: its mark only closes the channel.
func TestOutputs_TriggerFinalCloseOnly(t *testing.T) {
	env := runAsServices(t, nil)
	env.createDoc(t, map[string]any{"title": "Leak"})
	runWorker(t, env)
	env.setStatus(t, "hot")
	require.Eventually(t, func() bool {
		got, err := env.svc.Store().GetEntity(context.Background(), entity.Ref{ID: "DOC-001"})
		return err == nil && got.Properties["status"] == "escalated"
	}, 20*time.Second, 50*time.Millisecond)
	env.setStatus(t, "cold")
	// The close lands before the mark records its result, so wait for both.
	require.Eventually(t, func() bool {
		h, err := env.svc.channels.Head(context.Background(), runAsKey)
		if err != nil || h.State != channels.StateClosed {
			return false
		}
		marks := env.marks(t)
		return len(marks) == 2 && len(marks[1].Results) > 0
	}, 20*time.Second, 50*time.Millisecond)
	last := env.marks(t)[1]
	require.Equal(t, principal.UserRealtime, last.Writer.User)
	require.Len(t, last.Results, 1)
	require.Equal(t, "close", last.Results[0].Kind)
}

// TestOutputs_WriterFailsClosed pins that no output writes as the process:
// a job output with no run_as, a run_as that is one of rela's identities,
// and a retry rebuilt as system:realtime are each forbidden and write
// nothing.
func TestOutputs_WriterFailsClosed(t *testing.T) {
	env := runAsServices(t, nil)
	env.createDoc(t, map[string]any{"title": "Leak"})
	w, err := newManagerOutputs(env.svc.entityManager, env.svc.store, env.svc.meta, nil, env.svc.aclDeclarative)
	require.NoError(t, err)
	target := realtime.OutputTarget{Kind: realtimecfg.OutputSet, AnchorID: "DOC-001", AnchorType: "doc"}
	for name, r := range map[string]realtime.OutputRequest{
		"job without run_as": {Job: true},
		"rela run_as":        {Job: true, RunAs: principal.UserRealtime},
		"retry as rela": {Retry: &realtime.OutputIdentity{
			Writer: channels.Author{User: principal.UserRealtime, Tool: principal.ToolRealtime},
		}},
	} {
		r.Target, r.Props, r.Origin = target, map[string]any{"status": "x"}, "session:t"
		_, err := w.Write(leaderCtx(), r)
		f, ok := errors.AsType[*realtime.OutputError](err)
		require.Truef(t, ok, "%s: want an *OutputError, got %v", name, err)
		require.Equal(t, realtime.OutputCodeForbidden, f.Code, name)
		require.True(t, f.WroteNothing, name)
		require.Nil(t, env.doc(t).Properties["status"], name)
	}
}
