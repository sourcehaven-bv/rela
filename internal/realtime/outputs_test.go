package realtime

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/realtimecfg"
	"github.com/Sourcehaven-BV/rela/internal/realtimelimits"
)

// outputsRealtime declares breach on tickets with graph outputs. alice
// owns TKT-1 (lead), erin holds privacy.officer (dpo), mo moderates.
// assessedSet is the set of the contained -> assessed transition.
func outputsRealtime(assessedSet string) string {
	return `
version: 1
channel_types:
  breach:
    attach_to: [ticket]
    read: { from: attached }
    roles:
      lead:   { from: entity.owner }
      member: { from: attached }
      dpo:    { permission: privacy.officer }
      mod:    { permission: chat.moderate }
    message_types:
      text:      { properties: { body: { type: string } }, post: [member] }
      contained: { properties: { note: { type: markdown } }, post: [lead, member] }
      assessed:  { properties: { risk: { type: risk-level, required: true } }, post: [lead, dpo] }
      notified:  { properties: { reference: { type: string, required: true } }, post: [dpo] }
      dismiss:   { properties: { reason: { type: string, required: true } }, post: [lead] }
      flag:      { properties: { note: { type: string } }, post: [member, dpo] }
    message_ops:
      remove: { roles: [mod] }
    lifecycle:
      close:  { roles: [member] }
      reopen: { roles: [member] }
    session:
      initial: detected
      on_final: close
      states:
        detected:
          on:
            contained:
              to: contained
              roles: [lead]
              outputs:
                - create: { type: doc, properties: { title: "Contained by {{message.author}}: {{message.note}}" }, relate: { type: about } }
            dismiss: { to: dismissed, roles: [lead] }
            flag:    { to: contained, roles: [dpo] }
        contained:
          on:
            assessed:
              to: assessed
              roles: [lead, dpo]
              when: "message.risk ~= 'none'"
              outputs:
                - set: { properties: { ` + assessedSet + ` } }
        assessed:
          on:
            notified:
              to: notified
              roles: [dpo]
              outputs:
                - set: { properties: { status: notified, title: "{{message.reference}}" } }
        notified:  { final: true }
        dismissed: { final: true }
`
}

var outCh = RawAddress{EntityType: "ticket", Anchor: "TKT-1", Type: "breach"}

var (
	outLead = func() *fakeAccess { return user("alice") }
	outDPO  = func() *fakeAccess { return user("erin", "privacy.officer") }
	outMod  = func() *fakeAccess { return user("mo", "chat.moderate") }
)

// fakeOutputs is an OutputWriter that records what it is asked.
type fakeOutputs struct {
	mu     sync.Mutex
	checks []OutputTarget
	reqs   []OutputRequest
	// deny refuses Check by kind; "relate" refuses a create with relate.
	deny map[string]*DenialError
	// errs are returned by the next Write calls in order; nil succeeds.
	errs     []error
	warnings []string
	// join applies the create's progress, as a joined store does.
	join  channels.Store
	n     int
	write func(r OutputRequest) // called on each Write
	// poster is what Poster reports.
	poster OutputPoster
}

func (f *fakeOutputs) Poster(context.Context) (OutputPoster, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.poster, nil
}

func (f *fakeOutputs) Check(_ context.Context, t OutputTarget) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.checks = append(f.checks, t)
	if d := f.deny[string(t.Kind)]; d != nil {
		return d
	}
	if d := f.deny["relate"]; d != nil && t.Relate != "" {
		return d
	}
	return nil
}

func (f *fakeOutputs) Write(ctx context.Context, r OutputRequest) (OutputWritten, error) {
	f.mu.Lock()
	f.reqs = append(f.reqs, r)
	var err error
	if len(f.errs) > 0 {
		err, f.errs = f.errs[0], f.errs[1:]
	}
	hook := f.write
	f.n++
	n := f.n
	f.mu.Unlock()
	if hook != nil {
		hook(r)
	}
	if err != nil {
		return OutputWritten{}, err
	}
	out := OutputWritten{Warnings: f.warnings}
	if r.Target.Kind != realtimecfg.OutputCreate {
		return out, nil
	}
	id := fmt.Sprintf("DOC-%d", 100+n)
	out.Entity = &channels.Promoted{ID: id, Type: r.Target.Type}
	switch {
	case f.join != nil:
		m, aerr := r.Progress.Apply(ctx, f.join, id, r.Target.Type)
		if aerr != nil {
			return OutputWritten{}, aerr
		}
		out.Marked, out.Message = true, m
	case r.Started != nil:
		if serr := r.Started(ctx); serr != nil {
			return OutputWritten{}, serr
		}
	}
	return out, nil
}

func (f *fakeOutputs) requests() []OutputRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]OutputRequest(nil), f.reqs...)
}

func newOutputsHarness(t *testing.T, w *fakeOutputs, assessedSet ...string) *harness {
	t.Helper()
	set := "status: assessed, due: \"{{today}}\""
	if len(assessedSet) > 0 {
		set = assessedSet[0]
	}
	return newHarness(t, withYAML(outputsRealtime(set)), withOutputs(w))
}

// notice returns the notice at seq.
func (h *harness) notice(t *testing.T, seq int64) channels.Message {
	t.Helper()
	m, err := h.store.Get(context.Background(), outKey, seq)
	require.NoError(t, err)
	require.True(t, isNotice(m.Type, m.TriggeredBy), "seq %d is a notice", seq)
	return m
}

// toAssessed drives the breach channel to assessed: contained (create,
// seq 1-2) and assessed (set, seq 3-4).
func (h *harness) toAssessed(t *testing.T) {
	t.Helper()
	h.postTo(t, outLead(), outCh, "contained", props("note", "door locked"))
	h.postTo(t, outLead(), outCh, "assessed", props("risk", "high"))
}

func TestOutputs_MessageSet(t *testing.T) {
	t.Parallel()
	w := &fakeOutputs{}
	h := newOutputsHarness(t, w)
	h.toAssessed(t)
	res := h.postTo(t, outDPO(), outCh, "notified", props("reference", "AP-1"))

	reqs := w.requests()
	require.Len(t, reqs, 3)
	set := reqs[2]
	require.Equal(t, OutputTarget{
		Kind: realtimecfg.OutputSet, AnchorID: "TKT-1", AnchorType: "ticket", AttachTo: []string{"ticket"},
		Fields: map[string]any{"status": "notified", "title": nil},
	}, set.Target, "a templated value is nil in Fields")
	require.Equal(t, map[string]any{"status": "notified", "title": "AP-1"}, set.Props)
	require.Equal(t, "session:TKT-1/breach/assessed->notified#3/0", set.Origin)
	require.Nil(t, set.Retry, "the inline write runs as the request")

	require.NotNil(t, res.Outputs)
	require.Equal(t, OutputsDone, res.Outputs.Status)
	require.Equal(t, "erin", res.Outputs.Writer.User)
	require.Equal(t, "data-entry", res.Outputs.Writer.Tool, "the writer keeps the poster's tool")
	require.Equal(t, []OutputResultView{
		{Kind: "set", Status: "done", Props: []string{"status", "title"}},
		{Kind: "close", Status: "done"},
	}, res.Outputs.Results)
	require.Equal(t, "notified", res.Session.State)

	head, err := h.store.Head(context.Background(), outKey)
	require.NoError(t, err)
	require.Equal(t, channels.StateClosed, head.State, "on_final closed the channel")
	trail, err := h.store.Audit(context.Background(), "TKT-1")
	require.NoError(t, err)
	var closed *channels.AuditRecord
	for i := range trail {
		if trail[i].Op == channels.AuditLifecycle && trail[i].To == string(channels.StateClosed) {
			closed = &trail[i]
		}
	}
	require.NotNil(t, closed)
	require.Equal(t, "session:breach/on_final", closed.TriggeredBy)

	n := h.notice(t, 6)
	require.Equal(t, channels.ActionDone, n.Action.Status)
	view := h.readAll(t, outLead(), outCh).Messages[5]
	require.NotNil(t, view.Outputs)
	require.Nil(t, view.Action, "the outputs block replaces the action")
	require.Empty(t, view.Actions, "a notice offers no actions")
}

func TestOutputs_MessageCreate(t *testing.T) {
	t.Parallel()
	w := &fakeOutputs{}
	h := newOutputsHarness(t, w)
	h.refs.views["DOC-101"] = RefView{ID: "DOC-101", Type: "doc", Title: "Contained"}
	res := h.postTo(t, outLead(), outCh, "contained", props("note", "door locked"))
	reqs := w.requests()
	require.Len(t, reqs, 1)
	require.Equal(t, OutputTarget{
		Kind: realtimecfg.OutputCreate, AnchorID: "TKT-1", AnchorType: "ticket", Type: "doc", Relate: "about",
		AttachTo: []string{"ticket"},
	},
		reqs[0].Target)
	require.Equal(t, map[string]any{"title": "Contained by alice: door locked"}, reqs[0].Props)
	require.Equal(t, []OutputResultView{{
		Kind: "create", Status: "done", Entity: &RefValue{ID: "DOC-101", Type: "doc", Title: "Contained"},
	}}, res.Outputs.Results)
	require.Equal(t, "contained", res.Session.State)
}

func TestOutputs_LiteralNotAuthored(t *testing.T) {
	t.Parallel()
	w := &fakeOutputs{}
	h := newOutputsHarness(t, w)
	h.toAssessed(t)
	set := w.requests()[1]
	require.Equal(t, map[string]any{"status": "assessed", "due": "2026-10-12"}, set.Props)
	require.Equal(t, map[string]any{"due": "2026-10-12"}, set.Authored, "only interpolated values are authored")
}

func TestOutputs_PreCheckRefuses(t *testing.T) {
	t.Parallel()
	denied := &DenialError{Status: http.StatusForbidden, Type: "denied", Title: "no", Rule: "rule-1"}
	const createKey = "channel_types.breach.session.states.detected.on.contained.outputs[0]"
	for name, deny := range map[string]string{
		"update denied": "set", "create denied": "create", "relate denied": "relate",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			w := &fakeOutputs{deny: map[string]*DenialError{deny: denied}}
			h := newOutputsHarness(t, w)
			typ, p, key := "contained", props("note", "x"), createKey
			if deny == "set" {
				h.postTo(t, outLead(), outCh, "contained", props("note", "x"))
				typ, p = "assessed", props("risk", "high")
				key = "channel_types.breach.session.states.contained.on.assessed.outputs[0]"
			}
			before := h.store.appends.Load()
			_, err := h.svc.Post(as(outLead()), outCh, PostInput{Type: typ, Props: p})
			d := requireDenial(t, err, http.StatusForbidden, TypeSessionOutputDenied)
			require.Contains(t, d.Title, key)
			require.Equal(t, "rule-1", d.Rule)
			require.Equal(t, before, h.store.appends.Load(), "nothing is stored")
		})
	}

	t.Run("no pre-check without a firing transition", func(t *testing.T) {
		t.Parallel()
		w := &fakeOutputs{deny: map[string]*DenialError{"set": denied, "create": denied}}
		h := newOutputsHarness(t, w)
		// bob lacks lead: contained fires nothing for him.
		h.postTo(t, user("bob"), outCh, "contained", props("note", "x"))
		require.Empty(t, w.checks)
		h.postTo(t, outDPO(), outCh, "flag", props("note", "x"))
		// The guard of assessed does not hold for risk none.
		h.postTo(t, outLead(), outCh, "assessed", props("risk", "none"))
		require.Empty(t, w.checks)
	})

	t.Run("degraded", func(t *testing.T) {
		t.Parallel()
		w := &fakeOutputs{}
		h := newOutputsHarness(t, w)
		h.postTo(t, outLead(), outCh, "contained", props("note", "x"))
		snap := h.cfg.Load()
		snap.Degraded = true
		h.cfg.set(snap)
		// dpo holds its role by permission, which a degraded snapshot keeps.
		_, err := h.svc.Post(as(outDPO()), outCh, PostInput{Type: "assessed", Props: props("risk", "high")})
		d := requireDenial(t, err, http.StatusForbidden, TypeSessionOutputDenied)
		require.Contains(t, d.Title, "degraded")
		h.postTo(t, user("bob"), outCh, "text", props("body", "a post without outputs"))
	})

	t.Run("no outputs in this process", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, withYAML(outputsRealtime("status: x")))
		_, err := h.svc.Post(as(outLead()), outCh, PostInput{Type: "contained", Props: props("note", "x")})
		d := requireDenial(t, err, http.StatusForbidden, TypeSessionOutputDenied)
		require.Contains(t, d.Title, "not available in this process")
	})
}

func TestOutputs_PreCheckRace(t *testing.T) {
	t.Parallel()
	w := &fakeOutputs{}
	h := newOutputsHarness(t, w)
	k := outKey
	h.postTo(t, user("bob"), outCh, "text", props("body", "hi"))
	// The head the call reads says detected; the store moves to contained
	// before the append, where assessed fires a set the pre-check never
	// asked about.
	h.store.beforeAppend = func() {
		_, err := h.store.Store.Append(context.Background(), k, channels.NewMessage{
			Type: "flag", Author: channels.Author{User: "erin"}, At: h.clock.Now(), Props: props("note", "x"),
			Session: &channels.SessionStep{
				Decide: func(channels.Channel) (channels.SessionMove, bool) {
					return channels.SessionMove{From: "detected", To: "contained"}, true
				},
				NoticeType: realtimecfg.SessionNoticeType, NoticeBy: systemAuthor(),
				TriggeredBy: channels.SessionMarker + "breach", Outcome: &channels.SessionOutcome{},
				Due: func(channels.Channel) channels.SessionSchedule { return channels.SessionSchedule{} },
			},
		})
		require.NoError(t, err)
	}
	res := h.postTo(t, outLead(), outCh, "assessed", props("risk", "high"))
	require.Equal(t, "contained", res.Session.State, "the decider does not move into unchecked outputs")
	require.Nil(t, res.Outputs)
	require.Empty(t, w.requests())
}

func TestOutputs_ValidationFails(t *testing.T) {
	t.Parallel()
	w := &fakeOutputs{errs: []error{&OutputError{Code: OutputCodeInvalid, Props: []string{"title"}}}}
	h := newOutputsHarness(t, w)
	res := h.postTo(t, outLead(), outCh, "contained", props("note", "x"))
	require.Equal(t, "contained", res.Session.State, "the move stays")
	require.Equal(t, OutputsFailed, res.Outputs.Status)
	require.Equal(t, []OutputResultView{{Kind: "create", Status: "failed", Code: "invalid", Props: []string{"title"}}},
		res.Outputs.Results)
	stale, err := h.store.StalePending(context.Background(), h.clock.Now().Add(time.Hour))
	require.NoError(t, err)
	require.Empty(t, stale, "no retry is scheduled")
}

func TestOutputs_Warnings(t *testing.T) {
	t.Parallel()
	w := &fakeOutputs{warnings: []string{WarnPromoteFollowUpFailed}}
	h := newOutputsHarness(t, w)
	res := h.postTo(t, outLead(), outCh, "contained", props("note", "x"))
	require.Equal(t, OutputsDone, res.Outputs.Status)
	require.Equal(t, []string{WarnPromoteFollowUpFailed}, res.Outputs.Results[0].Warnings)
}

func TestOutputs_Order(t *testing.T) {
	t.Parallel()
	w := &fakeOutputs{errs: []error{nil, &OutputError{Code: OutputCodeForbidden}}}
	yaml := outputsRealtime("status: assessed")
	yaml = replaceOnce(t, yaml, `                - create: { type: doc, properties: { title: "Contained by {{message.author}}: {{message.note}}" }, relate: { type: about } }`,
		`                - create: { type: doc, properties: { title: one } }
                - create: { type: doc, properties: { title: two } }
                - set: { properties: { status: three } }`)
	h := newHarness(t, withYAML(yaml), withOutputs(w))
	res := h.postTo(t, outLead(), outCh, "contained", props("note", "x"))
	reqs := w.requests()
	require.Len(t, reqs, 2, "a failure stops the rest")
	require.Equal(t, "one", reqs[0].Props["title"])
	require.Equal(t, "two", reqs[1].Props["title"])
	require.Equal(t, []string{"done", "failed", "skipped"},
		[]string{res.Outputs.Results[0].Status, res.Outputs.Results[1].Status, res.Outputs.Results[2].Status})
}

func TestOutputs_DisjointSetsBothWritten(t *testing.T) {
	t.Parallel()
	w := &fakeOutputs{}
	h := newOutputsHarness(t, w, "due: \"{{today}}\"")
	h.postTo(t, outLead(), outCh, "contained", props("note", "x"))
	w.errs = []error{errors.New("transient")}
	res := h.postTo(t, outLead(), outCh, "assessed", props("risk", "high"))
	require.Equal(t, OutputsPending, res.Outputs.Status, "A is left for the runner")
	h.postTo(t, outDPO(), outCh, "notified", props("reference", "AP-1"))
	h.clock.Add(realtimelimits.OutputClaimTTL + time.Second)
	runner := h.outputsRunner(t, w)
	_, err := runner.Run(context.Background())
	require.NoError(t, err)
	reqs := w.requests()
	last := reqs[len(reqs)-1]
	require.Equal(t, map[string]any{"due": "2026-10-12"}, last.Props, "A writes its property")
	require.Equal(t, map[string]any{"status": "notified", "title": "AP-1"}, reqs[len(reqs)-2].Props)
}

func TestOutputs_SameSetSuperseded(t *testing.T) {
	t.Parallel()
	w := &fakeOutputs{}
	h := newOutputsHarness(t, w)
	h.postTo(t, outLead(), outCh, "contained", props("note", "x"))
	w.errs = []error{errors.New("transient")}
	h.postTo(t, outLead(), outCh, "assessed", props("risk", "high"))
	h.postTo(t, outDPO(), outCh, "notified", props("reference", "AP-1"))
	h.clock.Add(realtimelimits.OutputClaimTTL + time.Second)
	_, err := h.outputsRunner(t, w).Run(context.Background())
	require.NoError(t, err)
	reqs := w.requests()
	require.Equal(t, map[string]any{"due": "2026-10-12"}, reqs[len(reqs)-1].Props, "A skips status")
	n := h.notice(t, 4)
	require.Equal(t, []string{"due"}, n.Action.Outputs.Results[0].Props)

	// Every property superseded reads superseded.
	w2 := &fakeOutputs{}
	h2 := newOutputsHarness(t, w2, "status: assessed")
	h2.postTo(t, outLead(), outCh, "contained", props("note", "x"))
	w2.errs = []error{errors.New("transient")}
	h2.postTo(t, outLead(), outCh, "assessed", props("risk", "high"))
	h2.postTo(t, outDPO(), outCh, "notified", props("reference", "AP-1"))
	h2.clock.Add(realtimelimits.OutputClaimTTL + time.Second)
	_, err = h2.outputsRunner(t, w2).Run(context.Background())
	require.NoError(t, err)
	require.Equal(t, channels.OutputSuperseded, h2.notice(t, 4).Action.Outputs.Results[0].Status)
}

func TestOutputs_NoOutputsNoMark(t *testing.T) {
	t.Parallel()
	w := &fakeOutputs{}
	h := newOutputsHarness(t, w)
	res := h.postTo(t, outDPO(), outCh, "flag", props("note", "x"))
	require.Equal(t, "contained", res.Session.State)
	require.Nil(t, res.Outputs)
	require.Nil(t, h.notice(t, 2).Action)
}

func TestOutputs_FinalWithoutOutputs(t *testing.T) {
	t.Parallel()
	w := &fakeOutputs{}
	h := newOutputsHarness(t, w)
	res := h.postTo(t, outLead(), outCh, "dismiss", props("reason", "false alarm"))
	require.Equal(t, []OutputResultView{{Kind: "close", Status: "done"}}, res.Outputs.Results)
	head, err := h.store.Head(context.Background(), outKey)
	require.NoError(t, err)
	require.Equal(t, channels.StateClosed, head.State)
	require.Empty(t, w.requests())
}

func TestOutputs_ReopenDoesNotReclose(t *testing.T) {
	t.Parallel()
	w := &fakeOutputs{}
	h := newOutputsHarness(t, w)
	h.postTo(t, outLead(), outCh, "dismiss", props("reason", "false alarm"))
	_, err := h.svc.Lifecycle(as(user("bob")), outCh, "reopen")
	require.NoError(t, err)
	h.clock.Add(time.Hour)
	_, err = h.outputsRunner(t, w).Run(context.Background())
	require.NoError(t, err)
	head, err := h.store.Head(context.Background(), outKey)
	require.NoError(t, err)
	require.Equal(t, channels.StateOpen, head.State)
}

func TestOutputs_CauseRemovalBlocked(t *testing.T) {
	t.Parallel()
	w := &fakeOutputs{errs: []error{errors.New("transient")}}
	h := newOutputsHarness(t, w)
	h.postTo(t, outLead(), outCh, "contained", props("note", "x"))
	_, err := h.svc.Delete(as(outMod()), outCh, 1, "spam")
	requireDenial(t, err, http.StatusConflict, TypeOutputsPending)
	h.clock.Add(realtimelimits.OutputClaimTTL + time.Second)
	_, err = h.outputsRunner(t, w).Run(context.Background())
	require.NoError(t, err)
	_, err = h.svc.Delete(as(outMod()), outCh, 1, "spam")
	require.NoError(t, err)
}

func TestReleaseAction_RefusesOutputsMark(t *testing.T) {
	t.Parallel()
	w := &fakeOutputs{errs: []error{errors.New("transient")}}
	h := newOutputsHarness(t, w)
	h.postTo(t, outLead(), outCh, "contained", props("note", "x"))
	h.clock.Add(realtimelimits.PendingRelease + time.Hour)
	_, err := h.svc.ReleaseAction(as(outMod()), outCh, 2)
	requireDenial(t, err, http.StatusConflict, TypeOutputsPending)
}

func TestSweep_SkipsOutputsMark(t *testing.T) {
	t.Parallel()
	w := &fakeOutputs{errs: []error{errors.New("transient")}}
	yaml := replaceOnce(t, outputsRealtime("status: x"),
		"      text:      { properties: { body: { type: string } }, post: [member] }",
		"      text:      { properties: { body: { type: string } }, post: [member], actions: { accept: { roles: [member] } } }")
	h := newHarness(t, withYAML(yaml), withOutputs(w))
	h.postTo(t, outLead(), outCh, "contained", props("note", "x"))
	sw, err := NewSweeper(SweepDeps{
		Store: h.store, Config: h.cfg, Anchors: &fakeExistence{faces: map[string][]entity.Face{"TKT-1": {""}}}, Marker: h.local, Clock: h.clock.Now, DueWake: NewDueWake(),
	})
	require.NoError(t, err)
	h.clock.Add(realtimelimits.PendingRelease + time.Hour)
	rep, err := sw.RunLifecycle(context.Background())
	require.NoError(t, err)
	require.Empty(t, rep.StalePending)
}

// outputsRunner is a runner over h's store, config and local state.
func (h *harness) outputsRunner(t *testing.T, w OutputWriter) *Outputs {
	t.Helper()
	o, err := NewOutputs(OutputsDeps{Store: h.store, Config: h.cfg, Writer: w, Local: h.local, Clock: h.clock.Now})
	require.NoError(t, err)
	return o
}

// replaceOnce replaces the one occurrence of old in s.
func replaceOnce(t *testing.T, s, old, repl string) string {
	t.Helper()
	require.Equal(t, 1, strings.Count(s, old), "want one %q", old)
	return strings.Replace(s, old, repl, 1)
}

func TestOutputs_PostBudget(t *testing.T) {
	t.Parallel()
	w := &fakeOutputs{}
	h := newOutputsHarness(t, w, "status: assessed")
	h.postTo(t, outLead(), outCh, "contained", props("note", "x"))
	checks, writes := len(w.checks), len(w.requests())
	msgs, sets := h.store.messages.Load(), h.store.setActions.Load()
	res := h.postTo(t, outLead(), outCh, "assessed", props("risk", "high"))
	require.Equal(t, OutputsDone, res.Outputs.Status)
	require.Equal(t, 1, len(w.checks)-checks, "one AuthorizeUpdate")
	require.Equal(t, 1, len(w.requests())-writes, "one PatchEntity")
	require.Equal(t, int64(1), h.store.messages.Load()-msgs, "one supersede read")
	require.Equal(t, int64(2), h.store.setActions.Load()-sets, "a progress write and the done write")
}

// TestOutputsActionName pins the one action name channels and realtimecfg
// both declare, since neither imports the other.
func TestOutputsActionName(t *testing.T) {
	t.Parallel()
	require.Equal(t, channels.OutputsAction, realtimecfg.OutputsActionName)
}
