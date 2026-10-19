package realtime

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/realtimelimits"
)

const actionsRealtime = `
version: 1
channel_types:
  room:
    attach_to: [ticket]
    cardinality: one
    read: { from: attached }
    roles:
      member: { from: attached }
      lead:   { permission: chat.lead, label: lead }
      mod:    { permission: chat.moderate, label: moderator }
      agent:  { principal_type: agent }
    message_types:
      proposal:
        properties:
          summary: { type: string, required: true }
          about:   { type: ref, to: [ticket] }
          note:    { type: string }
        post: [member]
        actions:
          ack:    { roles: [member] }
          triage: { roles: [agent] }
          accept: { roles: [lead], denied: "Only the lead decides",
                    promote_to: { type: doc, map: { title: summary }, relate: { type: about } } }
      text: { properties: { body: { type: string } }, post: [member] }
    message_ops:
      edit_own: { roles: [member] }
      remove:   { roles: [mod] }
  free:
    attach_to: [ticket]
    cardinality: one
    read: { from: attached }
    roles:
      member: { from: attached }
      lead:   { permission: chat.lead }
    message_types:
      proposal:
        properties: { summary: { type: string, required: true } }
        post: [member]
        actions:
          accept: { roles: [lead], promote_to: { type: doc, map: { title: summary } } }
`

var freeRoom = RawAddress{EntityType: "ticket", Anchor: "TKT-1", Type: "free"}

// fakePromoter records promotes. With mark set it applies the mark on
// store, as a transactional promoter does. block, when set, holds every
// promote until it is closed.
type fakePromoter struct {
	mu     sync.Mutex
	reqs   []PromoteRequest
	calls  atomic.Int64
	err    error
	store  channels.Store
	mark   bool
	block  chan struct{}
	during func()
	warn   []entity.Warning
}

func (f *fakePromoter) Promote(ctx context.Context, r PromoteRequest) (PromoteResult, error) {
	f.calls.Add(1)
	f.mu.Lock()
	f.reqs = append(f.reqs, r)
	f.mu.Unlock()
	if f.block != nil {
		<-f.block
	}
	if f.during != nil {
		f.during()
	}
	if f.err != nil {
		return PromoteResult{}, f.err
	}
	res := PromoteResult{ID: "DOC-9", Type: r.Type, Warnings: f.warn}
	if f.mark {
		m, err := r.Mark.Apply(ctx, f.store, res.ID, res.Type)
		if err != nil {
			return PromoteResult{}, err
		}
		res.Marked, res.Message = true, m
	}
	return res, nil
}

func (f *fakePromoter) last() PromoteRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.reqs[len(f.reqs)-1]
}

func newActionHarness(t *testing.T, p Promoter) *harness {
	t.Helper()
	h := newHarness(t, withYAML(actionsRealtime), withPromoter(p))
	h.refs.views["DOC-9"] = RefView{ID: "DOC-9", Type: "doc", Title: "Promoted"}
	return h
}

func proposal(summary string) PostInput {
	return PostInput{Type: "proposal", Props: map[string]any{"summary": summary, "about": "TKT-2"}}
}

func decider() *fakeAccess   { return user("lena", "chat.lead") }
func moderator() *fakeAccess { return user("mo", "chat.moderate") }

// offered returns the _actions of message seq as a reads them.
func (h *harness) offeredOn(t *testing.T, a *fakeAccess, addr RawAddress, seq int64) []string {
	t.Helper()
	page, err := h.svc.Messages(as(a), addr, channels.Page{})
	require.NoError(t, err)
	for _, m := range page.Messages {
		if m.Seq == seq {
			return m.Actions
		}
	}
	t.Fatalf("message %d not on the page", seq)
	return nil
}

// TestAction_NotDeclared pins AC13: an undeclared action is not offered
// and answers 403 action_not_offered naming the rule.
func TestAction_NotDeclared(t *testing.T) {
	t.Parallel()
	h := newActionHarness(t, &fakePromoter{})
	m := h.post(t, user("ann"), proposal("x"))
	require.NotContains(t, h.offeredOn(t, user("ann"), room, m.Seq), "nope")
	_, err := h.svc.RunAction(as(user("ann")), room, m.Seq, "nope")
	d := requireDenial(t, err, http.StatusForbidden, TypeActionNotOffered)
	require.Equal(t, "channel_types.room.message_types.proposal.actions.nope", d.Rule)

	text := h.post(t, user("ann"), text("plain"))
	require.Nil(t, h.offeredOn(t, user("ann"), room, text.Seq))
	_, err = h.svc.RunAction(as(user("ann")), room, text.Seq, "ack")
	requireDenial(t, err, http.StatusForbidden, TypeActionNotOffered)
}

// TestAction_GateDenied pins AC13: a refused gate answers 403
// action_denied with the action's denied text.
func TestAction_GateDenied(t *testing.T) {
	t.Parallel()
	p := &fakePromoter{}
	h := newActionHarness(t, p)
	m := h.post(t, user("ann"), proposal("x"))
	require.Equal(t, []string{"ack"}, h.offeredOn(t, user("ann"), room, m.Seq))
	_, err := h.svc.RunAction(as(user("ann")), room, m.Seq, "accept")
	d := requireDenial(t, err, http.StatusForbidden, TypeActionDenied)
	require.Equal(t, "Only the lead decides", d.Title)
	_, err = h.svc.RunAction(as(readOnlyClient(decider())), room, m.Seq, "accept")
	requireDenial(t, err, http.StatusForbidden, TypeActionDenied)
	require.Zero(t, p.calls.Load(), "a refused action promotes nothing")
	got, err := h.store.Get(context.Background(), roomKey(), m.Seq)
	require.NoError(t, err)
	require.Nil(t, got.Action, "a refused action leaves the message alone")
}

// TestAction_NonPromoting pins that a plain action is one write to done,
// audited, after which the message offers no action.
func TestAction_NonPromoting(t *testing.T) {
	t.Parallel()
	h := newActionHarness(t, &fakePromoter{})
	m := h.post(t, user("ann"), proposal("x"))
	res, err := h.svc.RunAction(as(user("ann")), room, m.Seq, "ack")
	require.NoError(t, err)
	require.Equal(t, channels.ActionDone, res.Message.Action.Status)
	require.Equal(t, "ack", res.Message.Action.Name)
	require.Nil(t, res.Message.Actions)
	require.Empty(t, res.Warnings)

	_, err = h.svc.RunAction(as(user("ann")), room, m.Seq, "ack")
	requireDenial(t, err, http.StatusConflict, TypeActionTaken)
	_, err = h.svc.RunAction(as(decider()), room, m.Seq, "accept")
	requireDenial(t, err, http.StatusConflict, TypeActionTaken)
	require.Nil(t, h.offeredOn(t, decider(), room, m.Seq))

	trail, err := h.store.Audit(context.Background(), "TKT-1")
	require.NoError(t, err)
	var actions int
	for _, r := range trail {
		if r.Op == channels.AuditAction {
			actions++
			require.Equal(t, "ack", r.Action)
			require.Equal(t, "ann", r.By.User)
		}
	}
	require.Equal(t, 1, actions)
}

// TestAction_Promote pins AC17 and AC19 for a promoter that leaves the
// mark to the service: the request carries the mapped message props, the
// face, the relation, the anchor and the origin, and the message ends done
// with the promoted entity.
func TestAction_Promote(t *testing.T) {
	t.Parallel()
	p := &fakePromoter{warn: []entity.Warning{{Code: "required-missing", Path: "/x"}}}
	h := newActionHarness(t, p)
	m := h.post(t, user("ann"), proposal("Ship it"))
	require.Equal(t, []string{"accept", "ack"}, h.offeredOn(t, decider(), room, m.Seq))

	res, err := h.svc.RunAction(as(decider()), room, m.Seq, "accept")
	require.NoError(t, err)
	req := p.last()
	require.Equal(t, "doc", req.Type)
	require.Empty(t, req.Face)
	require.Equal(t, map[string]any{"title": "Ship it"}, req.Props)
	require.Equal(t, "about", req.Relate)
	require.Equal(t, channels.Anchor{ID: "TKT-1"}, req.Anchor)
	require.Equal(t, fmt.Sprintf("channel:TKT-1/room#%d/accept", m.Seq), req.Origin)
	require.Equal(t, channels.ActionDone, req.Mark.Done.Status)

	require.Equal(t, channels.ActionDone, res.Message.Action.Status)
	require.Equal(t, &RefValue{ID: "DOC-9", Type: "doc", Title: "Promoted"}, res.Message.Action.Promoted)
	require.Equal(t, p.warn, res.Warnings)
	stored, err := h.store.Get(context.Background(), roomKey(), m.Seq)
	require.NoError(t, err)
	require.Equal(t, &channels.Promoted{ID: "DOC-9", Type: "doc"}, stored.Action.Promoted)
}

// TestAction_PromoteMarked pins that the service does not mark again
// after a promoter that committed the mark with the entity.
func TestAction_PromoteMarked(t *testing.T) {
	t.Parallel()
	p := &fakePromoter{mark: true}
	h := newActionHarness(t, p)
	p.store = h.store
	m := h.post(t, user("ann"), proposal("x"))
	res, err := h.svc.RunAction(as(decider()), room, m.Seq, "accept")
	require.NoError(t, err)
	require.Empty(t, res.Warnings, "a second mark would conflict and warn")
	require.Equal(t, "DOC-9", res.Message.Action.Promoted.ID)
}

// TestAction_PromoteFailureReleases pins AC16 on the service side: a
// promote that wrote nothing answers its denial, and the action is back to
// none with a released audit record.
func TestAction_PromoteFailureReleases(t *testing.T) {
	t.Parallel()
	denial := forbidden(TypePromoteDenied, "no role grants create on doc", "acl")
	p := &fakePromoter{err: fmt.Errorf("%w: %w", ErrNothingWritten, denial)}
	h := newActionHarness(t, p)
	m := h.post(t, user("ann"), proposal("x"))
	_, err := h.svc.RunAction(as(decider()), room, m.Seq, "accept")
	requireDenial(t, err, http.StatusForbidden, TypePromoteDenied)
	got, err := h.store.Get(context.Background(), roomKey(), m.Seq)
	require.NoError(t, err)
	require.Nil(t, got.Action)
	require.Equal(t, []string{"accept", "ack"}, h.offeredOn(t, decider(), room, m.Seq), "the action can be tried again")

	trail, err := h.store.Audit(context.Background(), "TKT-1")
	require.NoError(t, err)
	require.Equal(t, channels.AuditReleased, trail[len(trail)-1].Status)
}

// TestAction_PromoteFailureStaysPending pins that an error after which the
// promoter may have written leaves the action pending, and that a pending
// action freezes the message.
func TestAction_PromoteFailureStaysPending(t *testing.T) {
	t.Parallel()
	p := &fakePromoter{err: errors.New("disk on fire")}
	h := newActionHarness(t, p)
	m := h.post(t, user("ann"), proposal("x"))
	_, err := h.svc.RunAction(as(decider()), room, m.Seq, "accept")
	require.Error(t, err)
	_, isDenial := errors.AsType[*DenialError](err)
	require.False(t, isDenial, "an unknown failure is a server error")

	got, err := h.store.Get(context.Background(), roomKey(), m.Seq)
	require.NoError(t, err)
	require.Equal(t, channels.ActionPending, got.ActionStatus())
	require.Nil(t, h.offeredOn(t, decider(), room, m.Seq))
	_, err = h.svc.RunAction(as(user("ann")), room, m.Seq, "ack")
	requireDenial(t, err, http.StatusConflict, TypeActionPending)
	_, err = h.svc.Edit(as(user("ann")), room, m.Seq, map[string]any{"summary": "changed"})
	requireDenial(t, err, http.StatusConflict, TypeActionPending)
	_, err = h.svc.Delete(as(moderator()), room, m.Seq, "spam")
	requireDenial(t, err, http.StatusConflict, TypeActionPending)
}

// TestAction_MarkFailureWarns pins that a promote whose mark fails answers
// 200 with promote_unconfirmed: the entity exists.
func TestAction_MarkFailureWarns(t *testing.T) {
	t.Parallel()
	p := &fakePromoter{}
	h := newActionHarness(t, p)
	m := h.post(t, user("ann"), proposal("x"))
	p.during = func() {
		_, err := h.store.SetAction(context.Background(), roomKey(), m.Seq, "accept", channels.ActionPending,
			channels.Action{By: channels.Author{User: "mo"}, At: h.clock.Now()})
		require.NoError(t, err)
	}
	res, err := h.svc.RunAction(as(decider()), room, m.Seq, "accept")
	require.NoError(t, err)
	require.Len(t, res.Warnings, 1)
	require.Equal(t, WarnPromoteUnconfirmed, res.Warnings[0].Code)
}

// TestAction_DoubleAcceptOneEntity pins AC14: two concurrent accepts
// promote once. The loser is refused with a 409: action_taken when it lost
// the compare-and-set, action_pending when it read the message after the
// winner had taken it.
func TestAction_DoubleAcceptOneEntity(t *testing.T) {
	t.Parallel()
	p := &fakePromoter{block: make(chan struct{})}
	h := newActionHarness(t, p)
	m := h.post(t, user("ann"), proposal("x"))
	errs := make(chan error, 2)
	for range 2 {
		go func() {
			_, err := h.svc.RunAction(as(decider()), room, m.Seq, "accept")
			errs <- err
		}()
	}
	loser := <-errs
	close(p.block)
	winner := <-errs
	require.NoError(t, winner)
	d, ok := errors.AsType[*DenialError](loser)
	require.Truef(t, ok, "want a denial, got %v", loser)
	require.Equal(t, http.StatusConflict, d.Status)
	require.Contains(t, []string{TypeActionTaken, TypeActionPending}, d.Type)
	require.Equal(t, int64(1), p.calls.Load())
}

// TestAction_NotOpen pins D7: an action runs only in an open channel.
func TestAction_NotOpen(t *testing.T) {
	t.Parallel()
	h := newActionHarness(t, &fakePromoter{})
	m := h.post(t, user("ann"), proposal("x"))
	_, err := h.store.SetLifecycle(context.Background(), roomKey(), channels.StateOpen,
		channels.Transition{To: channels.StateClosed, By: channels.Author{User: "lena"}, At: h.clock.Now()})
	require.NoError(t, err)
	require.Nil(t, h.offeredOn(t, user("ann"), room, m.Seq))
	_, err = h.svc.RunAction(as(user("ann")), room, m.Seq, "ack")
	requireDenial(t, err, http.StatusConflict, TypeChannelNotOpen)
}

// TestAction_AgentPrincipal pins AC20: an agent that holds the gate runs
// the action and one that does not cannot.
func TestAction_AgentPrincipal(t *testing.T) {
	t.Parallel()
	h := newActionHarness(t, &fakePromoter{})
	m := h.post(t, user("ann"), proposal("x"))
	_, err := h.svc.RunAction(as(agentUser("app")), room, m.Seq, "triage")
	requireDenial(t, err, http.StatusForbidden, TypeActionDenied)
	_, err = h.svc.RunAction(as(agentUser("agent")), room, m.Seq, "triage")
	require.NoError(t, err)
}

// TestAction_NoPromoterNotOffered pins that a process without a promoter
// offers no promoting action and refuses one.
func TestAction_NoPromoterNotOffered(t *testing.T) {
	t.Parallel()
	h := newHarness(t, withYAML(actionsRealtime))
	m := h.post(t, user("ann"), proposal("x"))
	require.Equal(t, []string{"ack"}, h.offeredOn(t, decider(), room, m.Seq))
	_, err := h.svc.RunAction(as(decider()), room, m.Seq, "accept")
	requireDenial(t, err, http.StatusForbidden, TypeActionNotOffered)
}

// TestAction_AffordanceAndGateAgree pins AC15: for every caller, message
// state and action, the action is offered exactly when running it
// succeeds.
func TestAction_AffordanceAndGateAgree(t *testing.T) {
	t.Parallel()
	callers := map[string]func() *fakeAccess{
		"member": func() *fakeAccess { return user("ann") }, "lead": decider, "moderator": moderator,
		"agent": func() *fakeAccess { return agentUser("agent") }, "read-only lead": func() *fakeAccess {
			return readOnlyClient(decider())
		},
	}
	states := []string{"fresh", "done", "pending", "closed"}
	for cname, caller := range callers {
		for _, state := range states {
			for _, action := range []string{"ack", "triage", "accept", "nope"} {
				t.Run(cname+"/"+state+"/"+action, func(t *testing.T) {
					t.Parallel()
					h := newActionHarness(t, &fakePromoter{})
					m := h.post(t, user("ann"), proposal("x"))
					ctx := context.Background()
					switch state {
					case "done":
						_, err := h.svc.RunAction(as(user("ann")), room, m.Seq, "ack")
						require.NoError(t, err)
					case "pending":
						_, err := h.store.SetAction(ctx, roomKey(), m.Seq, "accept", channels.ActionNone,
							channels.Action{Status: channels.ActionPending, By: channels.Author{User: "x"}, At: h.clock.Now()})
						require.NoError(t, err)
					case "closed":
						_, err := h.store.SetLifecycle(ctx, roomKey(), channels.StateOpen,
							channels.Transition{To: channels.StateClosed, By: channels.Author{User: "x"}, At: h.clock.Now()})
						require.NoError(t, err)
					}
					offered := h.offeredOn(t, caller(), room, m.Seq)
					_, err := h.svc.RunAction(as(caller()), room, m.Seq, action)
					require.Equal(t, err == nil, slices.Contains(offered, action), "offered %v, run error %v", offered, err)
				})
			}
		}
	}
}

// TestAction_GatesOncePerPage pins that a page asks each action's gate
// once, however many messages it holds.
func TestAction_GatesOncePerPage(t *testing.T) {
	t.Parallel()
	count := func(n int) int64 {
		h := newActionHarness(t, &fakePromoter{})
		for range n {
			h.post(t, user("ann"), proposal("x"))
		}
		a := decider()
		_, err := h.svc.Messages(as(a), room, channels.Page{})
		require.NoError(t, err)
		return a.permCalls.Load()
	}
	require.Equal(t, count(2), count(20))
}

// TestRelease_GateAndAge pins AC21: a moderator releases a pending action
// after PendingRelease, with a released audit record; earlier is 409, a
// caller without the remove gate is 403, and a message without a pending
// action is 409 action_not_pending. Without message_ops.remove the
// action's own gate releases.
func TestRelease_GateAndAge(t *testing.T) {
	t.Parallel()
	h := newActionHarness(t, &fakePromoter{err: errors.New("crash")})
	m := h.post(t, user("ann"), proposal("x"))
	plain := h.post(t, user("ann"), proposal("y"))
	_, err := h.svc.RunAction(as(decider()), room, m.Seq, "accept")
	require.Error(t, err)

	_, err = h.svc.ReleaseAction(as(decider()), room, m.Seq)
	requireDenial(t, err, http.StatusForbidden, TypeActionDenied)
	_, err = h.svc.ReleaseAction(as(moderator()), room, m.Seq)
	requireDenial(t, err, http.StatusConflict, TypeActionPending)
	_, err = h.svc.ReleaseAction(as(moderator()), room, plain.Seq)
	requireDenial(t, err, http.StatusConflict, TypeActionNotPending)

	h.clock.Add(realtimelimits.PendingRelease + time.Second)
	got, err := h.svc.ReleaseAction(as(moderator()), room, m.Seq)
	require.NoError(t, err)
	require.Nil(t, got.Action)
	trail, err := h.store.Audit(context.Background(), "TKT-1")
	require.NoError(t, err)
	last := trail[len(trail)-1]
	require.Equal(t, channels.AuditReleased, last.Status)
	require.Equal(t, "mo", last.By.User)

	// The free type declares no remove: the action's own gate releases.
	fm, err := h.svc.Post(as(user("ann")), freeRoom,
		PostInput{Type: "proposal", Props: map[string]any{"summary": "z"}})
	require.NoError(t, err)
	_, err = h.svc.RunAction(as(decider()), freeRoom, fm.Message.Seq, "accept")
	require.Error(t, err)
	h.clock.Add(realtimelimits.PendingRelease + time.Second)
	_, err = h.svc.ReleaseAction(as(user("ann")), freeRoom, fm.Message.Seq)
	requireDenial(t, err, http.StatusForbidden, TypeActionDenied)
	_, err = h.svc.ReleaseAction(as(decider()), freeRoom, fm.Message.Seq)
	require.NoError(t, err)
}

// TestPromoteProps_MessagePropsOnly pins AC19: the map copies message
// properties, ref values as ids, and leaves absent ones out.
func TestPromoteProps_MessagePropsOnly(t *testing.T) {
	t.Parallel()
	c := testCompiled(t, actionsRealtime)
	ct, _ := c.ChannelType("room")
	p := *ct.MessageTypes["proposal"].Actions["accept"].PromoteTo
	p.Map = map[string]string{"title": "summary", "ref": "about", "gone": "note"}
	got := promoteProps(&p, channels.Message{Props: map[string]any{"summary": "S", "about": "TKT-2"}})
	require.Equal(t, map[string]any{"title": "S", "ref": "TKT-2"}, got)
}
