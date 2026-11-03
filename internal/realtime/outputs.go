package realtime

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/realtimecfg"
	"github.com/Sourcehaven-BV/rela/internal/realtimelimits"
)

// OutputWriter writes session outputs to the graph (TKT-98TR9A). The
// wiring site implements it over the field-gated entity manager, so the
// ACL, the field grants, validation, automations and the graph audit apply
// as they do to any write.
type OutputWriter interface {
	// Poster reports the identity facts of the caller on ctx that a retry
	// needs to judge the writer as the post did.
	Poster(ctx context.Context) (OutputPoster, error)
	// Check asks, before the post is stored, whether the caller on ctx may
	// write the output t. A refusal is a *[DenialError]. It is a yes or no
	// only: the write authorizes again on the row as it then stands.
	Check(ctx context.Context, t OutputTarget) error
	// Write writes one output. A refusal or a classified failure is an
	// *[OutputError]; any other error may be retried.
	Write(ctx context.Context, r OutputRequest) (OutputWritten, error)
}

// OutputPoster are the identity facts of a poster, recorded in the mark.
type OutputPoster struct {
	// Unmatched is set for a verified poster that matched no user entity.
	Unmatched bool
	// UserEntity is set when the poster names an existing user entity.
	UserEntity bool
}

// OutputTarget is what one output writes.
type OutputTarget struct {
	Kind realtimecfg.OutputKind
	// AnchorID is the channel's anchor: the entity a set writes, and the
	// one a create relates to. AnchorType is its entity type.
	AnchorID   string
	AnchorType string
	// Face is, for a set, the face to write: the channel's own face, or the
	// configured face of a family-anchored type. The writer drops it for an
	// anchor whose type declares no faces. For a create it is the face of
	// the new entity.
	Face entity.Face
	// Type and Relate are a create's entity type and relation to the
	// anchor.
	Type   string
	Relate string
	// AttachTo are the channel type's attach_to types. A set with no
	// AnchorType writes the anchor only when its row is one of them.
	AttachTo []string
	// Fields are, for a set, the properties it writes: a literal's value,
	// or nil for a template, whose value is known only after the post. The
	// pre-check field-gates them; a write takes OutputRequest.Props.
	Fields map[string]any
}

// OutputRequest is one output write.
type OutputRequest struct {
	Target OutputTarget
	// Props are the evaluated properties. Authored holds those whose value
	// came from the message or the session. A set field-gates every
	// property; a create field-gates only Authored, as a promote does.
	Props    map[string]any
	Authored map[string]any
	// Origin is the triggered_by of the graph audit records:
	// `session:<channel key>/<from>-><to>#<generation>/<index>`.
	Origin string
	// RunAs, when set, is the job identity to write as: the session's
	// run_as, for a job mark (see [channels.OutputMark.Job]). It wins over
	// Retry. The writer binds it as a plain principal: no claims, no user
	// entity, and only the grants acl.yaml assigns to it.
	RunAs string
	// Job marks a job mark's output. Its writer is RunAs and nothing else:
	// a job output with no RunAs is refused, never written as the process.
	Job bool
	// Retry, when set, is the writer to act as: a retry has no request, so
	// the writer rebuilds the poster from the mark.
	Retry *OutputIdentity
	// Started, set for a create, is called by a writer that cannot commit
	// the create with Progress, right before the create. A failure writes
	// nothing.
	Started func(ctx context.Context) error
	// Progress is the mark write a writer that can join the channel store
	// commits in the create's transaction.
	Progress OutputProgress
}

// OutputIdentity is who a retry writes as: the cause message's author, the
// role claims verified at the post, and the poster's identity facts.
type OutputIdentity struct {
	Writer channels.Author
	Roles  []string
	OutputPoster
}

// OutputProgress is the progress write that records a create as done.
type OutputProgress struct {
	key  channels.Key
	seq  int64
	next channels.Action
}

// Apply records the created entity id and type in the last result and
// writes the progress on st.
func (p OutputProgress) Apply(ctx context.Context, st channels.Store, id, typ string) (channels.Message, error) {
	if p.next.Outputs == nil || len(p.next.Outputs.Results) == 0 {
		return channels.Message{}, errors.New("realtime: outputs progress was not made by the service")
	}
	next := p.next
	next.Outputs = p.next.Outputs.Clone()
	next.Outputs.Results[len(next.Outputs.Results)-1].Entity = &channels.Promoted{ID: id, Type: typ}
	return st.SetAction(ctx, p.key, p.seq, channels.OutputsAction, channels.ActionPending, next)
}

// OutputWritten is a written output.
type OutputWritten struct {
	// Entity is the entity a create made.
	Entity *channels.Promoted
	// Warnings are the warning codes of the write.
	Warnings []string
	// Marked is set when the writer applied Progress in the create's
	// transaction; Message is then the marked notice.
	Marked  bool
	Message channels.Message
}

// Output failure codes (see [channels.OutputResult.Code]).
const (
	OutputCodeForbidden         = "forbidden"
	OutputCodeFieldDenied       = "field_denied"
	OutputCodeInvalid           = "invalid"
	OutputCodeConflict          = "conflict"
	OutputCodeNotFound          = "not_found"
	OutputCodeUnconfirmed       = "unconfirmed"
	OutputCodeExpired           = "expired"
	OutputCodeTransitionRemoved = "transition_removed"
	// OutputCodeRunAsChanged fails the outputs of a job mark whose writer
	// is no longer the session's run_as, so an old mark never writes as
	// an identity the operator replaced.
	OutputCodeRunAsChanged = "run_as_changed"
	OutputCodeError        = "error"
)

// OutputError is a classified output failure. Code never quotes an
// error text or an entity id.
type OutputError struct {
	Code string
	// Props are the properties a validation refusal named.
	Props []string
	// Retryable marks a failure a later attempt may not repeat, such as a
	// lost version race.
	Retryable bool
	// WroteNothing is set when the writer is sure nothing was written.
	WroteNothing bool
}

func (f *OutputError) Error() string { return "realtime: output failed: " + f.Code }

// NoOutputs is the [OutputWriter] of a process that cannot write the
// graph. A post that would fire outputs is refused there.
type NoOutputs struct{}

// Poster reports no facts.
func (NoOutputs) Poster(context.Context) (OutputPoster, error) { return OutputPoster{}, nil }

// Check refuses every output.
func (NoOutputs) Check(context.Context, OutputTarget) error {
	return deny(http.StatusForbidden, TypeSessionOutputDenied, "outputs are not available in this process")
}

// Write refuses every output.
func (NoOutputs) Write(context.Context, OutputRequest) (OutputWritten, error) {
	return OutputWritten{}, &OutputError{Code: OutputCodeForbidden, WroteNothing: true}
}

// OutputsView is a notice's outputs mark as a reader sees it. Status is
// pending while the mark runs, failed when any output failed, and done
// otherwise. The writer's role claims are never shown.
type OutputsView struct {
	Status  string             `json:"status"`
	Writer  AuthorView         `json:"writer"`
	Results []OutputResultView `json:"results,omitempty"`
}

// OutputResultView is one output's result. Entity is a [RefValue], so a
// created entity the reader cannot read is restricted.
type OutputResultView struct {
	Kind     string    `json:"kind"`
	Status   string    `json:"status"`
	Code     string    `json:"code,omitempty"`
	Props    []string  `json:"props,omitempty"`
	Face     string    `json:"face,omitempty"`
	Entity   *RefValue `json:"entity,omitempty"`
	Warnings []string  `json:"warnings,omitempty"`
}

// The kinds and statuses of an [OutputResultView], for a reader outside
// the channel store (see [channels.OutputResult]).
const (
	OutputKindSet    = channels.OutputKindSet
	OutputKindCreate = channels.OutputKindCreate
	OutputKindClose  = channels.OutputKindClose
	// OutputKindTransition is the one result of a mark whose transition
	// the config no longer declares.
	OutputKindTransition = channels.OutputKindTransition
	OutputResultDone     = channels.OutputDone
	OutputResultFailed   = channels.OutputFailed
)

// Output view statuses.
const (
	OutputsPending = "pending"
	OutputsDone    = "done"
	OutputsFailed  = "failed"
)

// outputsView renders a, an outputs action.
func (r *renderer) outputsView(a *channels.Action) *OutputsView {
	v := &OutputsView{Status: OutputsDone, Writer: authorView(a.Outputs.Writer)}
	for _, res := range a.Outputs.Results {
		rv := OutputResultView{
			Kind: res.Kind, Status: res.Status, Code: res.Code, Props: res.Props, Face: res.Face,
			Warnings: res.Warnings,
		}
		if res.Entity != nil {
			rv.Entity = r.ref(res.Entity.ID)
		}
		if res.Status == channels.OutputFailed {
			v.Status = OutputsFailed
		}
		v.Results = append(v.Results, rv)
	}
	if a.Status == channels.ActionPending {
		v.Status = OutputsPending
	}
	return v
}

// isOutputs reports whether a is a session notice's outputs mark.
func isOutputs(a *channels.Action) bool {
	return a != nil && a.Name == channels.OutputsAction && a.Outputs != nil
}

// outputsPending is the 409 for a release or a cause removal while a
// mark is pending.
func outputsPending(title string) *DenialError {
	return deny(http.StatusConflict, TypeOutputsPending, title)
}

// outputGate is the decider's backstop: once the pre-check ran, an entry
// with outputs moves the session only when it is the entry whose outputs
// were checked, named by (from, to); see [realtimecfg.Session.TransitionTo].
// The decider may still choose another entry under the store's lock than
// the pre-check did (a reminder counter in a guard can change in between);
// a move along an entry without outputs writes nothing, so it needs no
// check and the gate lets it through.
type outputGate struct {
	active   bool
	from, to string
}

func (g *outputGate) admits(t realtimecfg.Transition, from string) bool {
	return g == nil || !g.active || len(t.Outputs()) == 0 || (g.from == from && g.to == t.To)
}

// outputPlan is what the pre-check of a post found: the poster's identity
// facts, and the mark's claim. marked says the last Mark call marked.
type outputPlan struct {
	poster     OutputPoster
	anchorType string
	claim      string
	marked     bool
}

// precheckOutputs runs the transition mt fires in the state the call read
// against the decider, and asks the writer about each of its outputs. The
// post is refused when any is refused, when the snapshot is degraded or
// when the process cannot write outputs. A post that fires no outputs
// asks nothing.
func (s *Service) precheckOutputs(c *call, mt *realtimecfg.MessageType, sp sessionPost) (*outputPlan, error) {
	plan := &outputPlan{claim: channels.NewName(), anchorType: c.anchor.Type}
	if sp.step == nil {
		return plan, nil
	}
	ch := channels.Channel{State: channels.StateOpen, CreatedAt: c.now}
	if c.head != nil {
		ch = *c.head
	}
	move, moves := sp.step.Decide(ch)
	sp.gate.active = true
	if !moves {
		return plan, nil
	}
	t, _ := c.ct.Session().TransitionTo(move.From, mt.Name, move.To)
	outs := t.Outputs()
	if len(outs) == 0 {
		return plan, nil
	}
	if c.snap.Degraded {
		return nil, deny(http.StatusForbidden, TypeSessionOutputDenied,
			"outputs are suspended while realtime.yaml is degraded")
	}
	poster, err := s.outputs.writer.Poster(c.ctx)
	if err != nil {
		return nil, fmt.Errorf("realtime: read the poster's identity: %w", err)
	}
	plan.poster = poster
	for i, o := range outs {
		if err := s.outputs.writer.Check(c.ctx, outputTarget(c.key, plan.anchorType, c.ct.AttachTo, o)); err != nil {
			if d, ok := errors.AsType[*DenialError](err); ok {
				return nil, &DenialError{
					Status: http.StatusForbidden, Type: TypeSessionOutputDenied,
					Title: fmt.Sprintf("the poster may not write %s.outputs[%d]: %s", t.Key(), i, d.Title),
					Rule:  d.Rule,
				}
			}
			return nil, fmt.Errorf("realtime: check output: %w", err)
		}
	}
	sp.gate.from, sp.gate.to = move.From, move.To
	return plan, nil
}

// closesOn reports whether a move into to closes the channel.
func closesOn(sess *realtimecfg.Session, to string) bool {
	st, ok := sess.State(to)
	return ok && st.Final && sess.OnFinal() == realtimecfg.OnFinalClose
}

// personMark is the Mark callback of a person's post: a mark claimed by
// the request, for a move whose transition has outputs or that closes the
// channel.
func personMark(
	sess *realtimecfg.Session, mt string, writer channels.Author, roles []string, now time.Time, plan *outputPlan,
) func(channels.SessionMove) *channels.Action {
	return func(move channels.SessionMove) *channels.Action {
		t, _ := sess.TransitionTo(move.From, mt, move.To)
		closes := closesOn(sess, move.To)
		// Assigned, not set: a backend may run the decider more than once.
		plan.marked = len(t.Outputs()) > 0 || closes
		if !plan.marked {
			return nil
		}
		return &channels.Action{
			PendingAt: now.Add(realtimelimits.OutputClaimTTL),
			Outputs: &channels.OutputMark{
				Writer: writer, Roles: roles, Unmatched: plan.poster.Unmatched, UserEntity: plan.poster.UserEntity,
				AnchorType: plan.anchorType, Attempt: 1, Claim: plan.claim, ClaimedAt: now, Close: closes,
			},
		}
	}
}

// triggerMark is the Mark callback of a trigger's post, for the
// transition of message type mt: a job mark for a move whose transition
// has outputs or that closes the channel. Nobody claims it: the runner
// does on its next round. marked, when set, records whether the last call
// marked.
func triggerMark(
	ct realtimecfg.ChannelType, mt string, now time.Time, marked *bool,
) func(channels.SessionMove) *channels.Action {
	sess := ct.Session()
	return func(move channels.SessionMove) *channels.Action {
		t, _ := sess.TransitionTo(move.From, mt, move.To)
		a := jobMark(ct, len(t.Outputs()) > 0, move.To, now)
		if marked != nil {
			// Assigned, not set: a backend may run the decider more than once.
			*marked = a != nil
		}
		return a
	}
}

// jobMark is the unclaimed mark of a job move into to: a timer's, or one a
// trigger's post fired. It is nil when the move has no outputs and does
// not close the channel. With outputs the writer is the session's run_as;
// a close-only mark names [systemAuthor]. Both are job marks, so a mark
// whose outputs appear later runs them only as the run_as it names (see
// [OutputCodeRunAsChanged]).
func jobMark(ct realtimecfg.ChannelType, outputs bool, to string, now time.Time) *channels.Action {
	sess := ct.Session()
	closes := closesOn(sess, to)
	if !outputs && !closes {
		return nil
	}
	writer := systemAuthor()
	if outputs {
		runAs := sess.RunAs()
		writer = channels.Author{User: runAs, Tool: principal.ToolRealtime, PrincipalType: runAsType(runAs)}
	}
	return &channels.Action{PendingAt: now, Outputs: &channels.OutputMark{
		Writer: writer, AnchorType: anchorTypeOf(ct), Job: true, Close: closes,
	}}
}

// runAsType is the principal type a run_as writer shows: integration for
// an `integration:` name, system for a `system:` one.
func runAsType(runAs string) string {
	if strings.HasPrefix(runAs, principal.IntegrationPrefix) {
		return "integration"
	}
	return "system"
}

// anchorTypeOf is the anchor's entity type when ct attaches to one type,
// or empty: a job move does not read the anchor, so with several
// attach_to types the writer reads the row's type at the write.
func anchorTypeOf(ct realtimecfg.ChannelType) string {
	if len(ct.AttachTo) == 1 {
		return ct.AttachTo[0]
	}
	return ""
}

// outputTarget is what output o of a channel at key, anchored to an
// entity of anchorType, writes.
func outputTarget(key channels.Key, anchorType string, attachTo []string, o realtimecfg.Output) OutputTarget {
	t := OutputTarget{
		Kind: o.Kind, AnchorID: key.ID, AnchorType: anchorType, Face: entity.Face(o.Face), AttachTo: attachTo,
	}
	switch o.Kind {
	case realtimecfg.OutputSet:
		if t.Face == "" {
			t.Face = key.Face
		}
		t.Fields = make(map[string]any, len(o.PropertyNames()))
		for name, tpl := range o.Properties() {
			t.Fields[name] = nil
			if tpl.Literal() {
				t.Fields[name], _ = tpl.Eval(realtimecfg.OutputVars{})
			}
		}
	case realtimecfg.OutputCreate:
		t.Type, t.Relate = o.Type, o.Relate
	}
	return t
}

// outputOrigin is the triggered_by of output i of mark.
func outputOrigin(key channels.Key, mark *channels.OutputMark, i int) string {
	return channels.SessionMarker + key.String() + "/" + mark.From + "->" + mark.To + "#" +
		strconv.FormatInt(mark.Gen, 10) + "/" + strconv.Itoa(i)
}

// onFinalOrigin is the triggered_by of the on_final close of type typ.
func onFinalOrigin(typ string) string { return channels.SessionMarker + typ + "/on_final" }

// outputEngine runs outputs marks: inline after a post, and in the runner.
type outputEngine struct {
	store  channels.Store
	cfg    ConfigSource
	writer OutputWriter
	local  *Local
	clock  Clock
}

// markRun is one run of a held mark.
type markRun struct {
	key    channels.Key
	notice channels.Message
	// cause is the message that caused the move; zero for a timer move.
	cause channels.Message
	// plan is what the run's one config snapshot declares for the mark.
	plan markPlan
	// superseded holds the properties a later move's mark recorded as set,
	// read up to readTo. done is set once that read reached the log's end.
	superseded map[string]bool
	readTo     int64
	done       bool
	retry      bool
	title      string
}

// markPlan is what one config snapshot declares for a mark's move. A run
// resolves it once, so it never mixes the outputs of one snapshot with the
// run_as of another.
type markPlan struct {
	// outs are the move's outputs; removed says the config no longer
	// declares the move.
	outs    []realtimecfg.Output
	removed bool
	// runAs is the session's run_as.
	runAs string
	// attachTo are the channel type's attach_to types.
	attachTo []string
}

// resolveMark finds the transition entry or timer of mark's move on key in
// snap: by (mark.From, cause.Type, mark.To) and the mark's job flag (see
// [markEntry]), or by (mark.From, mark.Timer) with the same target state.
// A trigger's job mark whose transition no longer is one a trigger can
// fire is removed: its outputs were compiled for a poster. A
// mark that is not a job mark but names one of rela's own identities as
// its writer (a close-only trigger mark from before job marks) runs no
// outputs, so nothing writes as rela itself; its close still runs.
func resolveMark(snap Snapshot, key channels.Key, cause channels.Message, mark *channels.OutputMark) markPlan {
	removed := markPlan{removed: true}
	if snap.Config == nil {
		return removed
	}
	ct, ok := snap.Config.ChannelType(key.Type)
	if !ok || ct.Session() == nil {
		return removed
	}
	sess := ct.Session()
	p := markPlan{runAs: sess.RunAs(), attachTo: ct.AttachTo}
	if mark.Timer == "" {
		t, ok := markEntry(sess.Branches(mark.From, cause.Type), mark)
		if !ok || (mark.Job && !t.Job()) {
			return removed
		}
		if !mark.Job && principal.IsRelaIdentity(mark.Writer.User) {
			return p
		}
		p.outs = t.Outputs()
		return p
	}
	st, _ := sess.State(mark.From)
	for _, tm := range st.Timers() {
		if tm.Name == mark.Timer && tm.To == mark.To {
			p.outs = tm.Outputs()
			return p
		}
	}
	return removed
}

// markEntry picks the entry of a list that mark's move went along: the
// first that leads to mark.To and, for a job mark, that a trigger can
// fire. Entries without outputs may share a target, such as one a lead
// fires and one a trigger fires into a state that closes the channel; a
// trigger's close-only mark then names the trigger's entry, not the
// lead's. An entry with outputs is the only one with its target, so for
// it the target alone decides. ok is false when no entry leads to
// mark.To; a job mark with only person entries to mark.To returns the
// first, which resolveMark then removes.
func markEntry(list []realtimecfg.Transition, mark *channels.OutputMark) (realtimecfg.Transition, bool) {
	var first realtimecfg.Transition
	found := false
	for _, t := range list {
		if t.To != mark.To {
			continue
		}
		if !mark.Job || t.Job() {
			return t, true
		}
		if !found {
			first, found = t, true
		}
	}
	return first, found
}

// supersedePage is the page size of the supersede read.
const supersedePage = 64

// load reads the cause, the notice at seq and a first page of the messages
// after it. ok is false when seq does not hold a notice with an outputs
// mark caused by the message before it, or made by a timer. The caller holds the channel's
// outputs lock, so a later move's run in this process has either finished
// or not started.
func (e *outputEngine) load(ctx context.Context, key channels.Key, seq int64) (markRun, bool, error) {
	r := markRun{key: key, superseded: map[string]bool{}, readTo: seq}
	page, err := e.store.Messages(ctx, key, channels.Page{AfterSeq: max(seq-2, 0), Limit: supersedePage + 2})
	if err != nil {
		return r, false, err
	}
	var later []channels.Message
	for _, m := range page {
		switch {
		case m.Seq == seq-1:
			r.cause = m
		case m.Seq == seq:
			r.notice = m
		case m.Seq > seq:
			later = append(later, m)
		}
	}
	r.note(later, len(page) < supersedePage+2)
	ok := r.notice.Seq == seq && isNotice(r.notice.Type, r.notice.TriggeredBy) && isOutputs(r.notice.Action)
	if ok && r.notice.Action.Outputs.Timer != "" {
		r.cause = channels.Message{}
		return r, true, nil
	}
	ok = ok && r.notice.Action.Outputs.Cause == seq-1 && r.cause.Seq == seq-1
	return r, ok, nil
}

// note adds the sets the later messages' marks recorded. end says the
// messages reach the log's end.
func (r *markRun) note(later []channels.Message, end bool) {
	for p := range supersededProps(later) {
		r.superseded[p] = true
	}
	if n := len(later); n > 0 {
		r.readTo = later[n-1].Seq
	}
	r.done = end
}

// supersededBy returns the properties a later move's mark recorded as set.
// It reads the rest of the log after the notice the first time a set needs
// it, so a retry long after the move still sees every later set.
func (e *outputEngine) supersededBy(ctx context.Context, r *markRun) (map[string]bool, error) {
	for !r.done {
		page, err := e.store.Messages(ctx, r.key, channels.Page{AfterSeq: r.readTo, Limit: supersedePage})
		if err != nil {
			return nil, err
		}
		r.note(page, len(page) < supersedePage)
	}
	return r.superseded, nil
}

// outputLocks serializes the outputs runs of each channel in this process.
// An entry lives while a run holds or waits for it, so a slow run blocks
// only its own channel.
type outputLocks struct {
	mu    sync.Mutex
	locks map[string]*outputLock
}

type outputLock struct {
	mu   sync.Mutex
	refs int
}

// lockOutputs takes the in-process lock of key's outputs and returns its
// unlock.
func (l *Local) lockOutputs(key channels.Key) func() {
	ol := &l.outputLocks
	k := key.String()
	ol.mu.Lock()
	if ol.locks == nil {
		ol.locks = map[string]*outputLock{}
	}
	lk := ol.locks[k]
	if lk == nil {
		lk = &outputLock{}
		ol.locks[k] = lk
	}
	lk.refs++
	ol.mu.Unlock()
	lk.mu.Lock()
	return func() {
		lk.mu.Unlock()
		ol.mu.Lock()
		lk.refs--
		if lk.refs == 0 {
			delete(ol.locks, k)
		}
		ol.mu.Unlock()
	}
}

// holdInline reports whether the post's own claim still holds r's mark.
// A run that waited behind another run of the channel may find its claim
// near the end of its TTL: it renews it first, so a runner elsewhere does
// not take the mark while it runs. A claim already taken is left to its
// new holder.
func (e *outputEngine) holdInline(ctx context.Context, r *markRun, claim string) bool {
	a := r.notice.Action
	if a.Status != channels.ActionPending || a.Outputs.Claim != claim {
		return false
	}
	if e.clock().Before(a.Outputs.ClaimedAt.Add(realtimelimits.OutputClaimTTL / 2)) {
		return true
	}
	if err := e.write(ctx, r, a.Outputs, channels.ActionPending); err != nil {
		slog.WarnContext(ctx, "realtime: renewing the outputs claim failed; the runner runs the mark",
			"channel", r.key.String(), "seq", r.notice.Seq, "error", err)
		return false
	}
	return true
}

// run runs the held mark of r: the outputs not run yet, then the close,
// then the done write. The caller holds the channel's outputs lock (see
// [outputEngine.load]). A mark left pending is retried by the runner once
// its PendingAt passes.
//
// r.plan must be resolved (see [resolveMark]). A job mark whose writer is
// not the plan's run_as fails every output with [OutputCodeRunAsChanged];
// the close still runs.
func (e *outputEngine) run(ctx context.Context, r *markRun) {
	mark := r.notice.Action.Outputs.Clone()
	outs, removed := r.plan.outs, r.plan.removed
	switch {
	case removed:
		if n := len(mark.Results); n == 0 || mark.Results[n-1].Kind != channels.OutputKindClose {
			mark.Results = append(mark.Results, channels.OutputResult{
				Kind: channels.OutputKindTransition, Status: channels.OutputFailed, Code: OutputCodeTransitionRemoved,
			})
		}
	case mark.Job && len(outs) > 0 && (r.plan.runAs == "" || r.plan.runAs != mark.Writer.User):
		failRest(mark, outs, len(mark.Results), OutputCodeRunAsChanged)
	default:
		if !e.runOutputs(ctx, r, mark, outs) {
			return
		}
	}
	if mark.Close {
		mark.Results = append(mark.Results, e.close(ctx, r))
		mark.Close = false
	}
	if err := e.write(ctx, r, mark, channels.ActionDone); err != nil {
		slog.ErrorContext(ctx, "realtime: writing the outputs mark done failed; the runner retries it",
			"channel", r.key.String(), "seq", r.notice.Seq, "error", err)
	}
}

// runOutputs runs the outputs of mark from the first without a result. It
// reports false when the mark stays pending.
func (e *outputEngine) runOutputs(
	ctx context.Context, r *markRun, mark *channels.OutputMark, outs []realtimecfg.Output,
) bool {
	i := len(mark.Results)
	if i > 0 && mark.Results[i-1].Status == channels.OutputStarted {
		// A create that cannot commit with its progress was started and
		// never recorded: it may have written, so it is not repeated.
		mark.Results[i-1] = channels.OutputResult{
			Kind: channels.OutputKindCreate, Status: channels.OutputFailed, Code: OutputCodeUnconfirmed,
		}
		skipRest(mark, outs, i)
		return true
	}
	if i < len(outs) && r.retry {
		switch {
		case e.clock().After(r.notice.At.Add(realtimelimits.OutputRetryWindow)):
			failRest(mark, outs, i, OutputCodeExpired)
			return true
		case mark.Attempt > realtimelimits.MaxOutputAttempts:
			failRest(mark, outs, i, OutputCodeError)
			return true
		}
	}
	for ; i < len(outs); i++ {
		res, how := e.runOne(ctx, r, mark, i, outs[i])
		if how == outputLeftPending {
			return false
		}
		mark.Results = append(mark.Results, res)
		if res.Status == channels.OutputFailed {
			skipRest(mark, outs, i+1)
			return true
		}
		if how == outputMarked {
			continue
		}
		if err := e.write(ctx, r, mark, channels.ActionPending); err != nil {
			slog.WarnContext(ctx, "realtime: outputs progress write failed; the runner retries the mark",
				"channel", r.key.String(), "seq", r.notice.Seq, "error", err)
			return false
		}
	}
	return true
}

// skipRest records the outputs from i on as skipped.
func skipRest(mark *channels.OutputMark, outs []realtimecfg.Output, i int) {
	for _, o := range outs[min(i, len(outs)):] {
		mark.Results = append(mark.Results, channels.OutputResult{Kind: string(o.Kind), Status: channels.OutputSkipped})
	}
}

// failRest records the outputs from i on as failed with code.
func failRest(mark *channels.OutputMark, outs []realtimecfg.Output, i int, code string) {
	for _, o := range outs[min(i, len(outs)):] {
		mark.Results = append(mark.Results, channels.OutputResult{
			Kind: string(o.Kind), Status: channels.OutputFailed, Code: code,
		})
	}
}

// outputOutcome is how one output ended.
type outputOutcome int

const (
	// outputRan: the result is to be recorded by a progress write.
	outputRan outputOutcome = iota
	// outputMarked: the writer recorded it in the create's transaction.
	outputMarked
	// outputLeftPending: retry the output later.
	outputLeftPending
)

// runOne runs output i of mark.
func (e *outputEngine) runOne(
	ctx context.Context, r *markRun, mark *channels.OutputMark, i int, o realtimecfg.Output,
) (channels.OutputResult, outputOutcome) {
	res := channels.OutputResult{Kind: string(o.Kind)}
	props, authored := evalOutput(o, r.vars(mark))
	if o.Kind == realtimecfg.OutputSet {
		res.Face = o.Face
		later, err := e.supersededBy(ctx, r)
		if err != nil {
			slog.WarnContext(ctx, "realtime: reading the later moves of an outputs mark failed; it is retried",
				"channel", r.key.String(), "seq", r.notice.Seq, "error", err)
			return res, outputLeftPending
		}
		for name := range props {
			if later[name] {
				delete(props, name)
				delete(authored, name)
			}
		}
		if len(props) == 0 {
			res.Status = channels.OutputSuperseded
			return res, outputRan
		}
		res.Props = sortedKeys(props)
	}
	req := OutputRequest{
		Target: outputTarget(r.key, mark.AnchorType, r.plan.attachTo, o), Props: props, Authored: authored,
		Origin: outputOrigin(r.key, mark, i),
	}
	r.identify(&req, mark)
	started := false
	if o.Kind == realtimecfg.OutputCreate {
		req.Started = func(ctx context.Context) error {
			m := mark.Clone()
			m.Results = append(m.Results, channels.OutputResult{Kind: res.Kind, Status: channels.OutputStarted})
			if err := e.write(ctx, r, m, channels.ActionPending); err != nil {
				return err
			}
			started = true
			return nil
		}
		m := mark.Clone()
		m.Results = append(m.Results, channels.OutputResult{Kind: res.Kind, Status: channels.OutputDone})
		req.Progress = OutputProgress{key: r.key, seq: r.notice.Seq, next: e.next(r, m, channels.ActionPending)}
	}
	wctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), promoteTimeout)
	got, err := e.writer.Write(wctx, req)
	cancel()
	if err == nil {
		res.Status, res.Entity, res.Warnings = channels.OutputDone, got.Entity, got.Warnings
		if got.Marked {
			r.notice = got.Message
			e.local.markMessage(r.key)
			return res, outputMarked
		}
		return res, outputRan
	}
	f, classified := errors.AsType[*OutputError](err)
	wroteNothing := classified && f.WroteNothing
	switch {
	case started && !wroteNothing:
		res.Status, res.Code = channels.OutputFailed, OutputCodeUnconfirmed
	case classified && !f.Retryable:
		res.Status, res.Code, res.Props = channels.OutputFailed, f.Code, f.Props
	case mark.Attempt >= realtimelimits.MaxOutputAttempts:
		res.Status, res.Code = channels.OutputFailed, OutputCodeError
	default:
		slog.WarnContext(ctx, "realtime: a session output failed; it is retried",
			"channel", r.key.String(), "seq", r.notice.Seq, "output", i, "attempt", mark.Attempt, "error", err)
		if started {
			// The writer wrote nothing: drop the started record so the
			// retry may create.
			if werr := e.write(ctx, r, mark, channels.ActionPending); werr != nil {
				res.Status, res.Code = channels.OutputFailed, OutputCodeUnconfirmed
				return res, outputRan
			}
		}
		return res, outputLeftPending
	}
	// A started record, if any, is replaced by the result's progress write.
	return res, outputRan
}

// identify sets who req writes as: a job mark writes as the run_as of the
// plan, a retry rebuilds the poster from the mark, and a post's own run
// writes as the request's principal.
func (r *markRun) identify(req *OutputRequest, mark *channels.OutputMark) {
	switch {
	case mark.Job:
		req.Job, req.RunAs = true, r.plan.runAs
	case r.retry:
		req.Retry = &OutputIdentity{
			Writer: mark.Writer, Roles: mark.Roles,
			OutputPoster: OutputPoster{Unmatched: mark.Unmatched, UserEntity: mark.UserEntity},
		}
	}
}

// vars are the template values of r's mark.
func (r *markRun) vars(mark *channels.OutputMark) realtimecfg.OutputVars {
	return realtimecfg.OutputVars{
		Message: r.cause.Props, Author: r.cause.Author.User,
		From: mark.From, To: mark.To, Generation: mark.Gen,
		ChannelKey: r.key.String(), ChannelType: r.key.Type, ChannelTitle: r.title,
		At: r.notice.At,
	}
}

// evalOutput evaluates o's templates. A whole-value reference to an absent
// message property is left out.
func evalOutput(o realtimecfg.Output, vars realtimecfg.OutputVars) (props, authored map[string]any) {
	props, authored = map[string]any{}, map[string]any{}
	for name, t := range o.Properties() {
		v, ok := t.Eval(vars)
		if !ok {
			continue
		}
		props[name] = v
		if !t.Literal() {
			authored[name] = v
		}
	}
	return props, authored
}

// readsTitle reports whether any of outs reads the channel title.
func readsTitle(outs []realtimecfg.Output) bool {
	for _, o := range outs {
		for _, t := range o.Properties() {
			if t.ReadsTitle() {
				return true
			}
		}
	}
	return false
}

// supersededProps are the properties a later notice's mark recorded as
// set.
func supersededProps(later []channels.Message) map[string]bool {
	out := map[string]bool{}
	for _, m := range later {
		if !isNotice(m.Type, m.TriggeredBy) || !isOutputs(m.Action) {
			continue
		}
		for _, res := range m.Action.Outputs.Results {
			if res.Kind == channels.OutputKindSet && res.Status == channels.OutputDone {
				for _, p := range res.Props {
					out[p] = true
				}
			}
		}
	}
	return out
}

// close runs the on_final close of r's channel. A channel that is not
// open is left as it is. Like every output write it runs detached from the
// client, so a disconnect does not fail it.
func (e *outputEngine) close(ctx context.Context, r *markRun) channels.OutputResult {
	res := channels.OutputResult{Kind: channels.OutputKindClose, Status: channels.OutputDone}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), markTimeout)
	defer cancel()
	_, err := applyTransition(ctx, e.store, e.local, r.key, channels.StateOpen, channels.Transition{
		To: channels.StateClosed, By: systemAuthor(), At: e.clock(), TriggeredBy: onFinalOrigin(r.key.Type),
	})
	switch {
	case err == nil:
	case errors.Is(err, channels.ErrConflict), errors.Is(err, channels.ErrNotFound):
		res.Status = channels.OutputSkipped
	default:
		slog.ErrorContext(ctx, "realtime: the on_final close failed", "channel", r.key.String(), "error", err)
		res.Status, res.Code = channels.OutputFailed, OutputCodeError
	}
	return res
}

// next is the holder's next write of mark, with status.
func (e *outputEngine) next(r *markRun, mark *channels.OutputMark, status channels.ActionStatus) channels.Action {
	now := e.clock()
	next := *r.notice.Action
	next.Status, next.At, next.PendingAt = status, now, now.Add(realtimelimits.OutputClaimTTL)
	next.Outputs = mark.Clone()
	next.Outputs.ClaimedAt = now
	return next
}

// write stores the holder's next write of mark and keeps the stored
// notice in r.
func (e *outputEngine) write(
	ctx context.Context, r *markRun, mark *channels.OutputMark, status channels.ActionStatus,
) error {
	wctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), markTimeout)
	defer cancel()
	got, err := e.store.SetAction(wctx, r.key, r.notice.Seq, channels.OutputsAction, channels.ActionPending,
		e.next(r, mark, status))
	if err != nil {
		return err
	}
	r.notice = got
	e.local.markMessage(r.key)
	return nil
}
