// Package realtime is the channel service: it authorizes, validates, rate
// limits and renders channel operations over a [channels.Store], and holds
// the ephemeral signals that are never stored.
//
// Every gate comes in through a consumer-side interface the wiring site
// implements ([AccessSource], [AnchorReader], [RefResolver],
// [ConfigSource]), so the package depends on neither the ACL nor the entity
// store. Each operation captures its state once: one config snapshot, one
// [Access] over one ACL request, one clock reading.
//
// The ephemeral signals, rate buckets and live change sets are process
// local (see [Local]). A multi-process deployment limits each process on its
// own, and a signal posted to one process is not seen by another.
package realtime

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/realtimecfg"
	"github.com/Sourcehaven-BV/rela/internal/realtimelimits"
)

// ErrRealtimeOff reports that the project has no realtime.yaml. The HTTP
// layer answers every channel route with its uniform not-found.
var ErrRealtimeOff = errors.New("realtime: off")

func errNil(name string) error { return fmt.Errorf("realtime: %s is required", name) }

// Deps are the collaborators of a [Service].
type Deps struct {
	Store   channels.Store
	Config  ConfigSource
	Access  AccessSource
	Anchors AnchorReader
	Refs    RefResolver
	// Local is the process-local state, shared by the services built over
	// one store.
	Local *Local
	// Promoter creates the entities of promoting actions. A process that
	// cannot write the graph passes [NoPromoter].
	Promoter Promoter
	// DueWake is told the session due times the service stores, so the
	// timer loop can run early. Shared like Local.
	DueWake *DueWake
	// Outputs writes session outputs to the graph. A process that cannot
	// write the graph passes [NoOutputs].
	Outputs OutputWriter
	// Holders finds the anchors whose role property names the caller, for
	// the inbox.
	Holders HolderFinder
}

// Service is the channel service.
//
// 40 -> 42 (TKT-2B2AIA): MarkRead and SubscribeLive (named SubscribeReads
// until TKT-9YRRM5), the read marker entry points the transports call. Their helpers are free functions.
// Ratchet target: TKT-N0IKN9.
//
//plimsoll:max-methods=42
type Service struct {
	store    channels.Store
	cfg      ConfigSource
	access   AccessSource
	anchors  AnchorReader
	refs     RefResolver
	local    *Local
	clock    Clock
	promoter Promoter
	dueWake  *DueWake
	outputs  outputEngine
	holders  HolderFinder
	// bounds are the inbox ceilings; a test lowers them on its own service.
	bounds inboxBounds
}

// New returns the service over d.
//
// Nil: every field of d is rejected.
func New(d Deps) (*Service, error) {
	switch {
	case d.Store == nil:
		return nil, errNil("Store")
	case d.Config == nil:
		return nil, errNil("Config")
	case d.Access == nil:
		return nil, errNil("Access")
	case d.Anchors == nil:
		return nil, errNil("Anchors")
	case d.Refs == nil:
		return nil, errNil("Refs")
	case d.Local == nil:
		return nil, errNil("Local")
	case d.Promoter == nil:
		return nil, errNil("Promoter")
	case d.DueWake == nil:
		return nil, errNil("DueWake")
	case d.Outputs == nil:
		return nil, errNil("Outputs")
	case d.Holders == nil:
		return nil, errNil("Holders")
	}
	if isNoPromoter(d.Promoter) && declaresPromote(d.Config.Load().Config) {
		slog.Warn("realtime: realtime.yaml declares promoting actions, but this process cannot promote; " +
			"they are not offered")
	}
	return &Service{
		store: d.Store, cfg: d.Config, access: d.Access, anchors: d.Anchors, refs: d.Refs,
		local: d.Local, clock: d.Local.clock, promoter: d.Promoter, dueWake: d.DueWake, holders: d.Holders,
		bounds:  defaultInboxBounds(),
		outputs: outputEngine{store: d.Store, cfg: d.Config, writer: d.Outputs, local: d.Local, clock: d.Local.clock},
	}, nil
}

// isNoPromoter reports whether p is a [NoPromoter]. It only chooses a log
// line: what is offered follows [Promoter.CheckCreate].
func isNoPromoter(p Promoter) bool {
	switch p.(type) {
	case NoPromoter, *NoPromoter:
		return true
	}
	return false
}

// declaresPromote reports whether c declares an action with promote_to.
func declaresPromote(c *realtimecfg.Compiled) bool {
	if c == nil {
		return false
	}
	for _, ct := range c.ChannelTypes() {
		for _, mt := range ct.MessageTypes {
			for _, a := range mt.Actions {
				if a.PromoteTo != nil {
					return true
				}
			}
		}
	}
	return false
}

// Subscribe registers a live consumer of channel changes.
func (s *Service) Subscribe() *Subscription { return s.local.Subscribe() }

// SubscribeLive registers a live consumer that serves many readers; see
// [Local.SubscribeLive].
func (s *Service) SubscribeLive() *Subscription { return s.local.SubscribeLive() }

// SubscribeAddressed registers a consumer that also takes the principal
// keys each post or edit addressed; see [Local.SubscribeAddressed].
func (s *Service) SubscribeAddressed() *Subscription { return s.local.SubscribeAddressed() }

// Config returns the realtime configuration in force now, or nil when
// realtime is off. It is for describing the API (the OpenAPI document);
// an operation loads its own snapshot.
func (s *Service) Config() *realtimecfg.Compiled { return s.cfg.Load().Config }

// Enabled reports whether realtime is on now. A route uses it to answer
// "off" before it reads the request; an operation still loads its own
// snapshot, which decides.
func (s *Service) Enabled() bool { return s.Config() != nil }

// Affordances are what the caller may do in a channel now.
type Affordances struct {
	// Post lists the message types the caller may post, sorted. A type's
	// `when:` is not evaluated here; it depends on the message.
	Post []string `json:"post"`
	// EditOwn and DeleteOwn report whether the caller passes the gate of
	// the operation. Whether a given message qualifies also depends on its
	// author, the edit window and the channel state.
	EditOwn   bool `json:"edit_own"`
	DeleteOwn bool `json:"delete_own"`
	// Remove reports whether the caller passes the remove gate. Such a
	// reader also sees who removed a message and why.
	Remove bool `json:"remove"`
	// Lifecycle lists the lifecycle operations the caller may run on the
	// channel now, in the order close, reopen, archive, delete. Empty for a
	// cardinality-one channel that holds no message yet.
	Lifecycle []string `json:"lifecycle"`
	// Typing reports whether the caller may send typing here now: the type
	// offers it, the caller may post a log message type and the channel is
	// open. The rate limit is not part of it.
	Typing bool `json:"typing"`
	// Transitions lists the message types whose post by the caller would
	// move the session out of its current state, if the transition's guard
	// holds, sorted. It is a subset of Post, and empty, never nil, for a
	// channel type without a session; see [call.sessionAffordances].
	Transitions []string `json:"transitions"`
	// Entries lists the transition entries of the current state the
	// caller's post could take, sorted by message type and entry: the
	// caller may post the type now, the entry admits them, and no earlier
	// entry takes their post first. Empty, never nil; see
	// [call.sessionAffordances].
	Entries []EntryRef `json:"entries"`
}

// HeadView is a channel's head: its metadata, the last message, the live
// signals, who else is typing and the caller's affordances. TypingNow is
// never nil.
type HeadView struct {
	Channel   ChannelView     `json:"channel"`
	Last      *MessageView    `json:"last,omitempty"`
	Ephemeral []EphemeralView `json:"ephemeral"`
	TypingNow []TypingView    `json:"typing_now"`
	Actions   Affordances     `json:"actions"`
	// Reader is whose marker Channel.Read shows; the zero reader when the
	// caller has no identity. A live consumer replays the head when that
	// reader's marker moves.
	Reader channels.Reader `json:"-"`
}

// Head returns the channel's head.
func (s *Service) Head(ctx context.Context, addr RawAddress) (HeadView, error) {
	c, err := s.open(ctx, addr)
	if err != nil {
		return HeadView{}, err
	}
	var msgs []channels.Message
	if c.head != nil && c.head.LastSeq > 0 {
		m, gerr := s.store.Get(c.ctx, c.key, c.head.LastSeq)
		if gerr != nil && !errors.Is(gerr, channels.ErrNotFound) {
			return HeadView{}, fmt.Errorf("realtime: read last message: %w", gerr)
		}
		if gerr == nil {
			msgs = append(msgs, m)
		}
	}
	sigs, typingNow := s.liveSignals(c)
	r, err := s.renderer(c, msgs, sigs)
	if err != nil {
		return HeadView{}, err
	}
	out := HeadView{
		Channel:   c.channelView(c.key, c.head),
		Ephemeral: []EphemeralView{}, TypingNow: typingNow, Actions: c.affordances(),
	}
	out.Reader, _ = callReader(c)
	if c.head != nil {
		marks, rerr := readsOf(s.store, s.bounds.readKeys, c, []channels.Key{c.key})
		if rerr != nil {
			return HeadView{}, rerr
		}
		if st, ok := marks[c.key]; ok {
			// An own post between Head and Reads moved both; the higher
			// last seq keeps read.seq at or below last_seq.
			out.Channel.LastSeq = max(out.Channel.LastSeq, st.LastSeq)
			out.Channel.Read = readView(st)
		}
	}
	if len(msgs) > 0 {
		v := r.message(msgs[0])
		out.Last = &v
	}
	for _, sg := range sigs {
		out.Ephemeral = append(out.Ephemeral, r.signal(sg))
	}
	return out, nil
}

// affordances answers what the caller may do in c's channel.
func (c *call) affordances() Affordances {
	return c.affordancesWith(c.postable())
}

// affordancesWith is [call.affordances] over a post set from
// [call.postable], so a caller that needs the set twice evaluates the post
// gates once.
func (c *call) affordancesWith(post map[string]bool) Affordances {
	a := Affordances{Post: []string{}, Lifecycle: []string{}, Transitions: []string{}, Entries: []EntryRef{}}
	if c.requireCaller() != nil {
		return a
	}
	a.Remove = c.moderator()
	a.Lifecycle = c.lifecycleOps()
	for _, name := range sortedKeys(c.ct.MessageTypes) {
		if post[name] {
			a.Post = append(a.Post, name)
		}
	}
	if c.state() == channels.StateOpen {
		a.EditOwn = c.editGate()
	}
	a.DeleteOwn = c.deleteOwnGate()
	a.Typing = c.typingAllowed()
	a.Transitions, a.Entries = c.sessionAffordances(post)
	return a
}

// postable is the set of message types the caller may post here now: they
// hold a post role, the channel is open, and the session's state is in
// the type's `in:`. It is nil for a caller without identity.
func (c *call) postable() map[string]bool {
	if c.requireCaller() != nil || c.state() != channels.StateOpen {
		return nil
	}
	out := map[string]bool{}
	s, ch := c.ct.Session(), sessionChannel(c.head, c.now)
	for name, mt := range c.ct.MessageTypes {
		if c.mayPost(mt) && sessionRefusal(s, mt, ch) == nil {
			out[name] = true
		}
	}
	return out
}

// renderer builds the renderer for c, resolving every ref of msgs and sigs
// in one batch. A property the store recorded as a ref renders as one
// whatever the configuration now says (see Local.storedRefs).
func (s *Service) renderer(c *call, msgs []channels.Message, sigs []signal) (*renderer, error) {
	r := &renderer{ct: c.ct, moderator: c.moderator(), call: c, userType: c.snap.Config.MentionUserType()}
	if len(msgs) == 0 && len(sigs) == 0 {
		return r, nil
	}
	stored, err := s.local.storedRefs(c.ctx, s.store, c.snap.Config, c.ct.Name)
	if err != nil {
		return nil, err
	}
	r.stored = stored
	ids := r.refIDs(msgs, sigs)
	if len(ids) == 0 {
		return r, nil
	}
	got, err := s.refs.Resolve(c.ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("realtime: resolve refs: %w", err)
	}
	r.refs = got
	return r, nil
}

// PostInput is a message to post.
type PostInput struct {
	Type    string
	Props   map[string]any
	ReplyTo int64
	// ExpectGeneration is the session generation the post was decided
	// against: the post is stored only when the session is still at it,
	// read under the store's lock, and is refused with 409 session_moved
	// otherwise. Nil: no precondition. It applies only to a log message
	// type of a channel type with a session.
	ExpectGeneration *int64
	// LogOnly is set by a transport that runs in a process other than the
	// server's, such as the CLI or stdio MCP. A signal lives only in the
	// process that receives it, so nobody would see one posted there; such
	// a post is refused with 422 signals_not_kept, after authorization.
	LogOnly bool
}

// PostResult is a posted message. A log message has Message set. An
// ephemeral signal has Ephemeral, ID and ExpiresAt set, and no seq: it is
// never stored.
type PostResult struct {
	Ephemeral bool
	ID        string
	ExpiresAt time.Time
	Message   *MessageView
	// Session is the channel's session after a log message, for a type
	// with a session: moved when the message fired a transition.
	Session *SessionView
	// Outputs is the outputs mark of the move the message caused, after
	// its inline run; nil when the move wrote no mark.
	Outputs *OutputsView
}

// Post posts a message. The checks run in this order: read access, the
// message type and the shape of [PostInput.ExpectGeneration], a post role
// of the message type, a transport that keeps the type (see
// [PostInput.LogOnly]), an open channel, the session state (`in:`), the
// session generation, the rate limit, the properties, the type's `when:`,
// the refs. A denied caller is not charged a token, and neither is a
// request whose type or expect_generation is unusable; an invalid message
// is. The store checks the session state and
// generation again under its lock (see [admitPost]); a refusal there,
// after a concurrent move, has spent a token like any failed append.
func (s *Service) Post(ctx context.Context, addr RawAddress, in PostInput) (PostResult, error) {
	c, openErr := s.open(ctx, addr)
	if openErr != nil {
		return PostResult{}, openErr
	}
	if err := c.requireCaller(); err != nil {
		return PostResult{}, err
	}
	mt := c.ct.MessageTypes[in.Type]
	if mt == nil {
		return PostResult{}, invalid("channel type %s has no message type %q", c.ct.Name, in.Type)
	}
	if err := expectGenerationShape(c.ct.Session(), mt, in.ExpectGeneration); err != nil {
		return PostResult{}, err
	}
	if !c.mayPost(mt) {
		return PostResult{}, postDenied(c.ct, mt)
	}
	ephemeral := mt.Persist == realtimecfg.PersistEphemeral
	if ephemeral && in.LogOnly {
		return PostResult{}, signalsNotKept(mt.Name)
	}
	if err := c.requireOpen(); err != nil {
		return PostResult{}, err
	}
	if e := sessionRefusal(c.ct.Session(), mt, sessionChannel(c.head, c.now)); e != nil {
		return PostResult{}, sessionWrongState(c.ct, mt, e)
	}
	if e := generationRefusal(c.ct.Session(), sessionChannel(c.head, c.now), in.ExpectGeneration); e != nil {
		return PostResult{}, sessionMoved(e)
	}
	if err := s.local.limiter.take(c.now, bucketIdentity(c.p), c.key, opPostPfx+mt.Name,
		postRate(c.snap.Config.Limits(), mt)); err != nil {
		return PostResult{}, err
	}
	maxBytes := c.snap.Config.Limits().MessageBytes
	if ephemeral {
		maxBytes = min(maxBytes, realtimelimits.MaxEphemeralBytes)
	}
	// A signal is never stored, so it addresses no one.
	var mplan *mentionPlan
	if !ephemeral {
		mplan = &mentionPlan{}
	}
	vp, refs, err := s.checkMessageKeeping(c, mt, in.Props, maxBytes, nil, mplan)
	if err != nil {
		return PostResult{}, err
	}
	if ephemeral {
		return s.postSignal(c, mt, vp, in.ReplyTo)
	}
	targets := mplan.targets(refs, c.snap.Config.MentionUserType(), c.p.User)
	if len(targets) > channels.MaxMentions {
		return PostResult{}, tooManyMentions()
	}
	err = s.record(c)
	if err != nil {
		return PostResult{}, err
	}
	sp := newSessionPost(c.ctx, c.ct, mt, vp.props, c.now, poster{roles: c.transitionRoles(mt)}, "",
		in.ExpectGeneration)
	plan, err := s.precheckOutputs(c, mt, sp)
	if err != nil {
		return PostResult{}, err
	}
	if sp.step != nil {
		sp.step.Mark = personMark(c.ct.Session(), mt.Name, authorOf(c.p), c.p.Roles(), c.now, plan)
	}
	m, err := s.store.Append(c.ctx, c.key, channels.NewMessage{
		Type: mt.Name, Author: authorOf(c.p), At: c.now, Props: vp.props, ReplyTo: in.ReplyTo, Session: sp.step,
		MarkRead: true, Mentions: targets,
	})
	sp.logGuardFailure(c.ctx)
	if err != nil {
		return PostResult{}, appendRefusal(c, mt, err)
	}
	noteDue(s.dueWake, sp.step)
	s.local.notePosted(c, m)
	v := rendererWith(c, refs).message(m)
	out := PostResult{Message: &v, Session: postedSession(c, sp.step)}
	if plan.marked && sp.step.Outcome.Moved {
		out.Outputs = s.runInline(c, sp.step.Outcome.Notice, plan)
		if out.Outputs != nil && out.Outputs.Status != OutputsPending {
			// The on_final close may have closed the channel, and a set
			// may have handed a role to someone else.
			out.Session = s.sessionAfterOutputs(c, out.Session, wroteRoleProperty(c.ct, out.Outputs))
		}
	}
	return out, nil
}

// appendRefusal maps a failed append of a post: a session precondition
// the store refused under its lock (the generation, then in:) is the same
// denial as the pre-check's; anything else is a store error.
func appendRefusal(c *call, mt *realtimecfg.MessageType, err error) error {
	if e, ok := errors.AsType[*channels.SessionGenerationError](err); ok {
		return sessionMoved(e)
	}
	if e, ok := errors.AsType[*channels.SessionStateError](err); ok {
		return sessionWrongState(c.ct, mt, e)
	}
	return storeError(err)
}

// expectGenerationShape refuses an expect_generation the post cannot use:
// a negative one, or one on a channel type without a session or on an
// ephemeral type. Nil when expect is nil or usable.
func expectGenerationShape(sess *realtimecfg.Session, mt *realtimecfg.MessageType, expect *int64) error {
	switch {
	case expect == nil:
		return nil
	case *expect < 0:
		return invalid("expect_generation must be at least 0")
	case sess == nil || mt.Persist == realtimecfg.PersistEphemeral:
		return invalid("expect_generation applies only to a log message type of a channel type with a session")
	}
	return nil
}

// notePosted tells the process-local state about m, the log message c
// just appended: it ends the poster's typing entry and wakes the people m
// addresses and the poster's inbox.
func (l *Local) notePosted(c *call, m channels.Message) {
	// A log post ends the poster's typing entry; the marks below wake the
	// readers for both.
	l.typing.clear(c.key, bucketIdentity(c.p))
	addressed := channels.MentionTargets(m.Mentions)
	if m.ReplyTarget != "" {
		addressed = append(addressed, m.ReplyTarget)
	}
	l.markAddressed(addressed, c.key)
	// The append moved the poster's marker (MarkRead), so the poster's
	// inbox learns the channel even when it held no marker on it before.
	if reader, ok := callReader(c); ok {
		l.markFor(reader, c.key)
	}
}

// runInline runs the mark the post's move wrote, as the poster, and
// renders it. A failure to read the mark leaves it to the runner.
func (s *Service) runInline(c *call, notice int64, plan *outputPlan) *OutputsView {
	unlock := s.outputs.local.lockOutputs(c.key)
	defer unlock()
	r, ok, err := s.outputs.load(c.ctx, c.key, notice)
	if err != nil || !ok {
		slog.WarnContext(c.ctx, "realtime: reading the outputs mark after a post failed; the runner runs it",
			"channel", c.key.String(), "seq", notice, "error", err)
		return nil
	}
	if c.head != nil {
		r.title = c.head.Title
	}
	r.plan = resolveMark(c.snap, c.key, r.cause, r.notice.Action.Outputs)
	if s.outputs.holdInline(c.ctx, &r, plan.claim) {
		s.outputs.run(c.ctx, &r)
	}
	rr, err := s.renderer(c, []channels.Message{r.notice}, nil)
	if err != nil {
		slog.WarnContext(c.ctx, "realtime: rendering the outputs mark failed", "error", err)
		return nil
	}
	return rr.outputsView(r.notice.Action)
}

// sessionAfterOutputs is the session view after an inline run, which may
// have closed the channel: the head is read again only then. When handoff
// is set, a set changed a named role's property, so the anchor is read
// again too and the session shows the new holders.
func (s *Service) sessionAfterOutputs(c *call, before *SessionView, handoff bool) *SessionView {
	h, err := s.store.Head(c.ctx, c.key)
	if err != nil {
		return before
	}
	if handoff {
		s.rereadAnchor(c)
	}
	return readerSession(c, &h)
}

// rendererWith is a renderer over refs already resolved for the caller.
// It needs no stored refs: it renders only the caller's own write, after a successful
// Service.record, which refuses a config that declares a recorded ref as
// another type, or after a removal, which keeps no properties.
func rendererWith(c *call, refs map[string]RefView) *renderer {
	return &renderer{
		ct: c.ct, refs: refs, moderator: c.moderator(), call: c, userType: c.snap.Config.MentionUserType(),
	}
}

// postRate is the rate of mt: its own, or the configured default for its
// persistence.
func postRate(l realtimecfg.Limits, mt *realtimecfg.MessageType) realtimelimits.Rate {
	if mt.Rate.Count > 0 && mt.Rate.Per > 0 {
		return mt.Rate
	}
	if mt.Persist == realtimecfg.PersistEphemeral {
		return l.EphemeralRate
	}
	return l.LogRate
}

func (c *call) requireOpen() error {
	if st := c.state(); st != channels.StateOpen {
		return deny(http.StatusConflict, TypeChannelNotOpen, fmt.Sprintf("the channel is %s", st))
	}
	return nil
}

// checkMessageKeeping validates props against mt, evaluates its `when:`
// and resolves its refs, and the user candidates of plan when set, in one
// batch. Every ref must name an entity the caller may read, of a type the
// property allows; any other ref is the same 422. A user candidate that
// does not resolve is no error: it is just not a target. An edit passes
// the stored refs it keeps in kept (property name to id; see
// [Service.Edit]): a kept ref skips the readability check, since the
// caller did not choose it.
//
// Nil: kept and plan are accepted and mean none.
func (s *Service) checkMessageKeeping(
	c *call, mt *realtimecfg.MessageType, props map[string]any, maxBytes int, kept map[string]string,
	plan *mentionPlan,
) (validProps, map[string]RefView, error) {
	vp, err := validateProps(mt, props, maxBytes)
	if err != nil {
		return validProps{}, nil, err
	}
	if !whenHolds(c.ctx, mt, vp.props, c.now) {
		return validProps{}, nil, forbidden(TypePostDenied,
			"the message does not meet the when: condition of "+mt.Name,
			"channel_types."+c.ct.Name+".message_types."+mt.Name+".when")
	}
	ids := vp.refs
	if plan != nil {
		*plan = planMentions(mt, vp.props, c.snap.Config.MentionUserType())
		ids = append(slices.Clone(ids), plan.users...)
		slices.Sort(ids)
		ids = slices.Compact(ids)
	}
	if len(ids) == 0 {
		return vp, nil, nil
	}
	got, err := s.refs.Resolve(c.ctx, ids)
	if err != nil {
		return validProps{}, nil, fmt.Errorf("realtime: resolve refs: %w", err)
	}
	for name, p := range mt.Properties {
		if p.Type != realtimecfg.PropertyRef {
			continue
		}
		id, ok := vp.props[name].(string)
		if !ok || (kept[name] == id && kept[name] != "") {
			continue
		}
		ref, readable := got[id]
		if !readable || (len(p.To) > 0 && !slices.Contains(p.To, ref.Type)) {
			return validProps{}, nil, invalidRef()
		}
	}
	return vp, got, nil
}

// postSignal keeps an ephemeral message in memory.
func (s *Service) postSignal(
	c *call, mt *realtimecfg.MessageType, vp validProps, replyTo int64,
) (PostResult, error) {
	if replyTo != 0 {
		return PostResult{}, invalid("an ephemeral message cannot reply")
	}
	ttl := signalTTL(mt)
	sg := signal{
		id: channels.NewName(), typ: mt.Name, author: authorOf(c.p), identity: bucketIdentity(c.p),
		at: c.now, expires: c.now.Add(ttl), props: vp.props, size: vp.size,
	}
	expired, err := s.local.ephemeral.add(c.key, sg, c.now)
	s.local.markEphemeral(expired...)
	if err != nil {
		return PostResult{}, err
	}
	s.local.markEphemeral(c.key)
	s.local.scheduleExpiry()
	return PostResult{Ephemeral: true, ID: sg.id, ExpiresAt: sg.expires}, nil
}

// record writes the type record of c's channel type to the store before a
// stored write, so boot and reload can refuse a change of its anchor, its
// cardinality or a ref property. The store records a type once and only
// adds refs after that; a record that conflicts with the held one refuses
// the write with a 409 channel_type_conflict. Local's cache skips the
// store call for a record this process already wrote.
//
// After the record it checks that the live realtime.yaml still gives the
// type the shape it recorded. A reload checks the store's records before
// it applies, so one that ran before this record could not see it;
// refusing here keeps a write from landing under a shape the live config
// no longer has. It compares shapes, not config generations, so a reload
// that leaves this type's shape alone refuses nothing.
func (s *Service) record(c *call) error {
	return recordType(c.ctx, s.store, s.local, s.cfg, c.snap, c.ct)
}

// recordType is Service.record for any writer of ct's channels under
// snap: the channel service and the trigger engine.
func recordType(
	ctx context.Context, st channels.Store, local *Local, cfg ConfigSource, snap Snapshot, ct realtimecfg.ChannelType,
) error {
	rec := typeRecord(ct)
	if local.needsRecord(ct.Name, rec) {
		err := st.RecordTypes(ctx, map[string]channels.TypeRecord{ct.Name: rec})
		if errors.Is(err, channels.ErrTypeShapeConflict) {
			local.forgetStoredRefs()
			return typeConflict(ct.Name)
		}
		if err != nil {
			return fmt.Errorf("realtime: record channel type %s: %w", ct.Name, err)
		}
		local.noteRecorded(ct.Name, rec)
	}
	live := cfg.Load().Config
	if live != snap.Config {
		lct, ok := live.ChannelType(ct.Name)
		if !ok || !typeRecord(lct).Equal(rec) {
			return configChanged()
		}
	}
	return nil
}

// typeRecord is the store record of ct's configured shape.
func typeRecord(ct realtimecfg.ChannelType) channels.TypeRecord {
	shape := ct.Shape()
	return channels.TypeRecord{
		Anchor:      channels.AnchorMode(shape.Anchor),
		Cardinality: channels.Cardinality(shape.Cardinality),
		Refs:        shape.Refs,
		Plain:       ct.PlainProperties(),
	}
}

// CreateInput is a named channel to create.
type CreateInput struct {
	Title string
}

// Create creates a named channel of type typ on the anchor. Only a type
// with cardinality many has named channels.
func (s *Service) Create(
	ctx context.Context, entityType, anchor, typ string, in CreateInput,
) (ChannelView, error) {
	c, openErr := s.begin(ctx)
	if openErr != nil {
		return ChannelView{}, openErr
	}
	if err := c.channelType(entityType, typ); err != nil {
		return ChannelView{}, err
	}
	if c.ct.Cardinality != realtimecfg.CardinalityMany || c.ct.Create == nil {
		return ChannelView{}, deny(http.StatusConflict, TypeNotCreatable,
			fmt.Sprintf("channel type %s has one channel per anchor, created by its first post", typ))
	}
	if err := s.resolveAnchor(c, anchor); err != nil {
		return ChannelView{}, err
	}
	// The caller becomes the creator, so a creator read role admits them.
	if c.ct.Read.Source.Kind != realtimecfg.SourceCreator && !c.can(c.ct.Read, realtimecfg.CapabilityRead) {
		return ChannelView{}, readDenied(c.ct)
	}
	if err := c.requireCaller(); err != nil {
		return ChannelView{}, err
	}
	if !c.gate(*c.ct.Create) {
		return ChannelView{}, gateDenied(TypeCreateDenied, "creating "+typ+" channels",
			"channel_types."+typ+".create", *c.ct.Create, c.ct.Roles)
	}
	if err := s.local.limiter.take(c.now, bucketIdentity(c.p), c.key, opCreate, realtimelimits.OpRate); err != nil {
		return ChannelView{}, err
	}
	if err := s.record(c); err != nil {
		return ChannelView{}, err
	}
	// The store counts the channels of this type on the anchor and creates
	// in one step, so concurrent creates cannot pass channels_per_entity.
	nc := channels.NewChannel{
		Type: typ, Title: in.Title, By: authorOf(c.p), At: c.now,
		Max: c.snap.Config.Limits().ChannelsPerEntity, MarkRead: true,
	}
	if sess := c.ct.Session(); sess != nil {
		nc.SessionDue = sessionDue(sess)
	}
	ch, err := s.store.CreateMany(c.ctx, c.key.Anchor, nc)
	if err != nil {
		return ChannelView{}, storeError(err)
	}
	s.dueWake.Note(ch.Session.NextDueAt)
	s.local.mark(ch.Key)
	// The create wrote the creator's marker at 0 (MarkRead).
	if reader, ok := callReader(c); ok {
		s.local.markFor(reader, ch.Key)
	}
	return c.channelView(ch.Key, &ch), nil
}

// TypeChannels are the channels of one type on an anchor that the caller
// may read.
type TypeChannels struct {
	Type        string                  `json:"type"`
	Label       string                  `json:"label,omitempty"`
	Cardinality realtimecfg.Cardinality `json:"cardinality"`
	// Create reports whether the caller may create a channel of the type
	// here.
	Create   bool          `json:"create"`
	Channels []ChannelView `json:"channels"`
}

// AnchorChannels are the channel types and channels of one anchor.
type AnchorChannels struct {
	Anchor string         `json:"anchor"`
	Types  []TypeChannels `json:"types"`
	// Reader is as on [HeadView].
	Reader channels.Reader `json:"-"`
}

// ListAnchor lists the channels on the anchor that the caller may read,
// grouped by type, in one anchor read and one channel list. A bare id lists
// the family channels; `ID@face` lists the family channels and those of
// that face. An empty state lists every state.
func (s *Service) ListAnchor(
	ctx context.Context, entityType, anchor string, state channels.State,
) (AnchorChannels, error) {
	c, err := s.begin(ctx)
	if err != nil {
		return AnchorChannels{}, err
	}
	addr, err := entity.ParseAddress(anchor)
	if err != nil {
		return AnchorChannels{}, ErrAnchorNotFound
	}
	_, named := addr.Named()
	var types []realtimecfg.ChannelType
	var props []string
	for _, ct := range c.snap.Config.ChannelTypes() {
		if entityType != "" && !slices.Contains(ct.AttachTo, entityType) {
			continue
		}
		if ct.Anchor == realtimecfg.AnchorFace && !named {
			continue
		}
		types = append(types, ct)
		props = append(props, roleProps(ct)...)
	}
	slices.Sort(props)
	view, err := s.anchors.Anchor(c.ctx, addr, slices.Compact(props))
	if err != nil {
		if errors.Is(err, ErrAnchorNotFound) {
			return AnchorChannels{}, ErrAnchorNotFound
		}
		return AnchorChannels{}, fmt.Errorf("realtime: read anchor: %w", err)
	}
	if entityType != "" && view.Type != entityType {
		return AnchorChannels{}, ErrAnchorNotFound
	}
	c.anchor = view
	stored, err := s.store.ListEntity(c.ctx, view.ID)
	if err != nil {
		return AnchorChannels{}, fmt.Errorf("realtime: list channels: %w", err)
	}
	out := AnchorChannels{Anchor: addr.String(), Types: []TypeChannels{}}
	out.Reader, _ = callReader(c)
	var keys []channels.Key
	for _, ct := range types {
		if !slices.Contains(ct.AttachTo, view.Type) {
			continue
		}
		if tc, tk, ok := c.listType(ct, stored, state); ok {
			out.Types = append(out.Types, tc)
			keys = append(keys, tk...)
		}
	}
	marks, err := readsOf(s.store, s.bounds.readKeys, c, keys)
	if err != nil {
		return AnchorChannels{}, err
	}
	i := 0
	for t := range out.Types {
		for j := range out.Types[t].Channels {
			if st, ok := marks[keys[i]]; ok {
				out.Types[t].Channels[j].Read = readView(st)
			}
			i++
		}
	}
	return out, nil
}

// listType collects the readable channels of ct from stored, and their
// keys in the same order. A type whose read role the caller fails is left
// out; for a creator read role, each channel is checked on its own.
func (c *call) listType(
	ct realtimecfg.ChannelType, stored []channels.Channel, state channels.State,
) (TypeChannels, []channels.Key, bool) {
	// Role answers are per channel type: two types may declare one role
	// name with different sources.
	c.ct, c.head, c.ops = ct, nil, opGates{}
	anchor := anchorFor(ct, c.anchor)
	c.key = channels.Key{Anchor: anchor, Type: ct.Name}
	creator := ct.Read.Source.Kind == realtimecfg.SourceCreator
	if !creator && !c.can(ct.Read, realtimecfg.CapabilityRead) {
		return TypeChannels{}, nil, false
	}
	tc := TypeChannels{Type: ct.Name, Label: ct.Label, Cardinality: ct.Cardinality, Channels: []ChannelView{}}
	var keys []channels.Key
	for i := range stored {
		ch := stored[i]
		if ch.Key.Type != ct.Name || ch.Key.Anchor != anchor || (state != "" && ch.State != state) {
			continue
		}
		if creator {
			c.head = &ch
			if !c.can(ct.Read, realtimecfg.CapabilityRead) {
				continue
			}
		}
		tc.Channels = append(tc.Channels, c.channelView(ch.Key, &ch))
		keys = append(keys, ch.Key)
	}
	c.head = nil
	tc.Create = ct.Cardinality == realtimecfg.CardinalityMany && ct.Create != nil &&
		c.requireCaller() == nil && c.gate(*ct.Create)
	return tc, keys, true
}
