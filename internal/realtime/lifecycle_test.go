package realtime

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/realtimecfg"
)

// lifecycleRealtime declares every lifecycle operation on room, only a
// calendar-retention delete on calls, none on plain, and on timed a close
// whose condition reads no entity property.
const lifecycleRealtime = `
version: 1
channel_types:
  room:
    attach_to: [ticket]
    cardinality: one
    read: { from: attached }
    roles:
      member: { from: attached }
      lead:   { permission: chat.lead, label: lead }
    message_types:
      text: { properties: { body: { type: string } }, post: [member] }
    lifecycle:
      close:   { roles: [lead], condition: "entity.status == 'done'", denied: "Finish the ticket first" }
      reopen:  { roles: [lead] }
      archive: { roles: [lead], from: closed }
      delete:  { permission: records.delete, retention: 7d }
  calls:
    attach_to: [ticket]
    cardinality: many
    read: { from: attached }
    create: { roles: [member] }
    roles:
      member: { from: attached }
    message_types:
      text: { properties: { body: { type: string } }, post: [member] }
    lifecycle:
      delete: { roles: [member], retention: 1mo }
  plain:
    attach_to: [ticket]
    cardinality: one
    read: { from: attached }
    roles:
      member: { from: attached }
    message_types:
      text: { properties: { body: { type: string } }, post: [member] }
  timed:
    attach_to: [ticket]
    cardinality: one
    read: { from: attached }
    roles:
      member: { from: attached }
    message_types:
      text: { properties: { body: { type: string } }, post: [member] }
    lifecycle:
      close: { roles: [member], condition: "days_between(today(), today()) == 0" }
`

// lead may run every room operation; member may only post.
func lead() *fakeAccess   { return user("lead", "chat.lead", "records.delete") }
func member() *fakeAccess { return user("bob") }

// newLifecycleHarness is a harness over lifecycleRealtime whose room holds
// one message and whose TKT-1 has status done.
func newLifecycleHarness(t *testing.T) *harness {
	t.Helper()
	h := newHarness(t, withYAML(lifecycleRealtime))
	h.post(t, member(), PostInput{Type: "text", Props: map[string]any{"body": "hi"}})
	h.anchors.setStatus("done")
	return h
}

// moveRoom puts the room in state at the harness's current time, through
// the store.
func (h *harness) moveRoom(t *testing.T, to channels.State) {
	t.Helper()
	cur, err := h.store.Head(context.Background(), roomKey())
	require.NoError(t, err)
	if cur.State == to {
		return
	}
	_, err = h.store.SetLifecycle(context.Background(), roomKey(), cur.State,
		channels.Transition{To: to, By: channels.Author{User: "setup"}, At: h.clock.Now()})
	require.NoError(t, err)
}

func (h *harness) offered(t *testing.T, a *fakeAccess, addr RawAddress) []string {
	t.Helper()
	head, err := h.svc.Head(as(a), addr)
	require.NoError(t, err)
	return head.Actions.Lifecycle
}

// TestLifecycle_NotDeclared pins AC1: an undeclared operation is not
// offered and answers 403 lifecycle_not_offered naming the rule.
func TestLifecycle_NotDeclared(t *testing.T) {
	t.Parallel()
	h := newLifecycleHarness(t)
	plain := RawAddress{EntityType: "ticket", Anchor: "TKT-1", Type: "plain"}
	h.post(t, member(), PostInput{Type: "text"})
	_, err := h.svc.Post(as(member()), plain, PostInput{Type: "text"})
	require.NoError(t, err)

	require.Empty(t, h.offered(t, lead(), plain))
	_, err = h.svc.Lifecycle(as(lead()), plain, realtimecfg.LifecycleClose)
	d := requireDenial(t, err, http.StatusForbidden, TypeLifecycleNotOffered)
	require.Equal(t, "channel_types.plain.lifecycle.close", d.Rule)
	require.Equal(t, "close is not declared for plain channels", d.Title)

	_, err = h.svc.Lifecycle(as(lead()), room, "explode")
	requireDenial(t, err, http.StatusForbidden, TypeLifecycleNotOffered)
}

// TestLifecycle_NeverWrittenChannel pins D6: a cardinality-one channel
// that holds no message has nothing to act on.
func TestLifecycle_NeverWrittenChannel(t *testing.T) {
	t.Parallel()
	h := newHarness(t, withYAML(lifecycleRealtime))
	require.Empty(t, h.offered(t, lead(), room))
	_, err := h.svc.Lifecycle(as(lead()), room, realtimecfg.LifecycleClose)
	requireDenial(t, err, http.StatusNotFound, TypeChannelNotFound)
}

// TestLifecycle_GateDeniedUsesDeniedText pins AC2: a failed gate is 403
// lifecycle_denied, titled with `denied:` when set and else with who the
// gate admits. A read-only client fails every gate.
func TestLifecycle_GateDeniedUsesDeniedText(t *testing.T) {
	t.Parallel()
	h := newLifecycleHarness(t)
	_, err := h.svc.Lifecycle(as(member()), room, realtimecfg.LifecycleClose)
	d := requireDenial(t, err, http.StatusForbidden, TypeLifecycleDenied)
	require.Equal(t, "Finish the ticket first", d.Title)
	require.Equal(t, "channel_types.room.lifecycle.close", d.Rule)

	h.moveRoom(t, channels.StateClosed)
	_, err = h.svc.Lifecycle(as(member()), room, realtimecfg.LifecycleReopen)
	d = requireDenial(t, err, http.StatusForbidden, TypeLifecycleDenied)
	require.Equal(t, "reopen needs the lead role", d.Title)

	_, err = h.svc.Lifecycle(as(readOnlyClient(lead())), room, realtimecfg.LifecycleReopen)
	requireDenial(t, err, http.StatusForbidden, TypeLifecycleDenied)
	require.Empty(t, h.offered(t, readOnlyClient(lead()), room))
	require.Empty(t, h.offered(t, member(), room))
}

// TestLifecycle_FromWrongState pins AC3: an operation from a state it does
// not start from is 409 lifecycle_wrong_state and is not offered.
func TestLifecycle_FromWrongState(t *testing.T) {
	t.Parallel()
	h := newLifecycleHarness(t)
	require.NotContains(t, h.offered(t, lead(), room), realtimecfg.LifecycleArchive)
	_, err := h.svc.Lifecycle(as(lead()), room, realtimecfg.LifecycleArchive)
	d := requireDenial(t, err, http.StatusConflict, TypeLifecycleWrongState)
	require.Equal(t, "the channel is open; archive needs closed", d.Title)

	_, err = h.svc.Lifecycle(as(lead()), room, realtimecfg.LifecycleReopen)
	d = requireDenial(t, err, http.StatusConflict, TypeLifecycleWrongState)
	require.Equal(t, "the channel is open; reopen needs closed or archived", d.Title)
}

// TestLifecycle_Condition_TrueFalseRedactedDegraded pins AC4: the
// condition decides on the anchor's redacted record. A property the
// caller cannot see is absent from the record and fails the condition;
// a degraded config refuses every condition.
func TestLifecycle_Condition_TrueFalseRedactedDegraded(t *testing.T) {
	t.Parallel()
	h := newLifecycleHarness(t)
	require.Contains(t, h.anchors.lastReq, "status", "the anchor read asks for the condition's property")

	h.anchors.setStatus("open")
	require.NotContains(t, h.offered(t, lead(), room), realtimecfg.LifecycleClose)
	_, err := h.svc.Lifecycle(as(lead()), room, realtimecfg.LifecycleClose)
	d := requireDenial(t, err, http.StatusForbidden, TypeLifecycleDenied)
	require.Equal(t, "Finish the ticket first", d.Title)
	require.Equal(t, "channel_types.room.lifecycle.close.condition", d.Rule)

	// Redaction drops a hidden property from the view; it reads as absent.
	h.anchors.setStatus(nil)
	_, err = h.svc.Lifecycle(as(lead()), room, realtimecfg.LifecycleClose)
	requireDenial(t, err, http.StatusForbidden, TypeLifecycleDenied)

	h.anchors.setStatus("done")
	cfg := h.cfg.Load()
	h.cfg.set(Snapshot{Config: cfg.Config, Degraded: true})
	require.NotContains(t, h.offered(t, lead(), room), realtimecfg.LifecycleClose)
	_, err = h.svc.Lifecycle(as(lead()), room, realtimecfg.LifecycleClose)
	d = requireDenial(t, err, http.StatusForbidden, TypeLifecycleDenied)
	require.Contains(t, d.Title, "degraded")

	h.cfg.set(cfg)
	require.Contains(t, h.offered(t, lead(), room), realtimecfg.LifecycleClose)
	got, err := h.svc.Lifecycle(as(lead()), room, realtimecfg.LifecycleClose)
	require.NoError(t, err)
	require.Equal(t, channels.StateClosed, got.State)
}

// TestLifecycle_ConditionReadingNoProperty pins that a condition that reads
// no entity property is judged on the record all the same: the anchor read
// binds the record whether or not it was asked for properties.
func TestLifecycle_ConditionReadingNoProperty(t *testing.T) {
	t.Parallel()
	h := newHarness(t, withYAML(lifecycleRealtime))
	timed := RawAddress{EntityType: "ticket", Anchor: "TKT-1", Type: "timed"}
	_, err := h.svc.Post(as(member()), timed, PostInput{Type: "text"})
	require.NoError(t, err)
	require.Equal(t, []string{realtimecfg.LifecycleClose}, h.offered(t, member(), timed))
	got, err := h.svc.Lifecycle(as(member()), timed, realtimecfg.LifecycleClose)
	require.NoError(t, err)
	require.Equal(t, channels.StateClosed, got.State)
}

// TestLifecycle_DeleteRetention pins AC5: a delete under retention is
// refused, with the date it ends, until retention has passed since the
// close; a channel that never closed is refused too. Calendar retention
// uses calendar arithmetic. A delete answers nothing, keeps a deletion
// record in the anchor's audit trail, and the next post to a
// cardinality-one channel starts it over.
func TestLifecycle_DeleteRetention(t *testing.T) {
	t.Parallel()
	t.Run("fixed", func(t *testing.T) {
		t.Parallel()
		h := newLifecycleHarness(t)
		_, err := h.svc.Lifecycle(as(lead()), room, realtimecfg.LifecycleDelete)
		d := requireDenial(t, err, http.StatusConflict, TypeRetentionActive)
		require.Contains(t, d.Title, "close it first")
		require.NotContains(t, h.offered(t, lead(), room), realtimecfg.LifecycleDelete)

		closedAt := h.clock.Now()
		h.moveRoom(t, channels.StateClosed)
		h.clock.Add(7*24*time.Hour - time.Second)
		_, err = h.svc.Lifecycle(as(lead()), room, realtimecfg.LifecycleDelete)
		d = requireDenial(t, err, http.StatusConflict, TypeRetentionActive)
		require.Equal(t, "retention keeps this channel until "+
			closedAt.Add(7*24*time.Hour).Format(time.RFC3339), d.Title)
		require.NotContains(t, h.offered(t, lead(), room), realtimecfg.LifecycleDelete)

		h.clock.Add(time.Second)
		require.Contains(t, h.offered(t, lead(), room), realtimecfg.LifecycleDelete)
		got, err := h.svc.Lifecycle(as(lead()), room, realtimecfg.LifecycleDelete)
		require.NoError(t, err)
		require.Equal(t, ChannelView{}, got)
		_, err = h.store.Head(context.Background(), roomKey())
		require.ErrorIs(t, err, channels.ErrNotFound)
		trail, err := h.store.Audit(context.Background(), "TKT-1")
		require.NoError(t, err)
		last := trail[len(trail)-1]
		require.Equal(t, channels.AuditDelete, last.Op)
		require.Equal(t, "lead", last.By.User)

		m := h.post(t, member(), PostInput{Type: "text"})
		require.Equal(t, int64(1), m.Seq, "a deleted cardinality-one channel starts over")
	})
	t.Run("fractional close time", func(t *testing.T) {
		t.Parallel()
		h := newLifecycleHarness(t)
		h.clock.Add(500 * time.Millisecond)
		until := h.clock.Now().Add(7 * 24 * time.Hour)
		h.moveRoom(t, channels.StateClosed)
		h.clock.now = until.Add(-time.Millisecond)
		_, err := h.svc.Lifecycle(as(lead()), room, realtimecfg.LifecycleDelete)
		d := requireDenial(t, err, http.StatusConflict, TypeRetentionActive)
		named := until.Truncate(time.Second).Add(time.Second)
		require.Equal(t, "retention keeps this channel until "+named.Format(time.RFC3339), d.Title,
			"the title rounds up to the next whole second")
		h.clock.now = named
		_, err = h.svc.Lifecycle(as(lead()), room, realtimecfg.LifecycleDelete)
		require.NoError(t, err, "a retry at the time the title names succeeds")
	})
	t.Run("calendar", func(t *testing.T) {
		t.Parallel()
		h := newLifecycleHarness(t)
		h.clock.now = time.Date(2027, 1, 31, 12, 0, 0, 0, time.UTC)
		ch, err := h.svc.Create(as(member()), "ticket", "TKT-1", "calls", CreateInput{})
		require.NoError(t, err)
		addr := RawAddress{EntityType: "ticket", Anchor: "TKT-1", Type: "calls", Name: ch.Name}
		_, err = h.store.SetLifecycle(context.Background(), channels.Key{
			Anchor: channels.Anchor{ID: "TKT-1"}, Type: "calls", Name: ch.Name,
		}, channels.StateOpen, channels.Transition{To: channels.StateClosed, By: channels.Author{User: "x"},
			At: h.clock.Now()})
		require.NoError(t, err)
		// One month after 31 January is 3 March in Go's calendar arithmetic.
		h.clock.now = time.Date(2027, 3, 3, 11, 59, 59, 0, time.UTC)
		_, err = h.svc.Lifecycle(as(member()), addr, realtimecfg.LifecycleDelete)
		d := requireDenial(t, err, http.StatusConflict, TypeRetentionActive)
		require.Contains(t, d.Title, "2027-03-03T12:00:00Z")
		h.clock.now = time.Date(2027, 3, 3, 12, 0, 0, 0, time.UTC)
		_, err = h.svc.Lifecycle(as(member()), addr, realtimecfg.LifecycleDelete)
		require.NoError(t, err)
		_, err = h.svc.Head(as(member()), addr)
		requireDenial(t, err, http.StatusNotFound, TypeChannelNotFound)
	})
}

// TestLifecycle_TransitionsAuditAndMark pins AC6: close, archive and
// reopen move the state, bump the rev, write a lifecycle audit record and
// wake subscribers of the channel.
func TestLifecycle_TransitionsAuditAndMark(t *testing.T) {
	t.Parallel()
	h := newLifecycleHarness(t)
	sub := h.svc.Subscribe()
	defer sub.Close()
	takeKeys(t, sub)
	steps := []struct {
		op   string
		want channels.State
	}{
		{realtimecfg.LifecycleClose, channels.StateClosed},
		{realtimecfg.LifecycleArchive, channels.StateArchived},
		{realtimecfg.LifecycleReopen, channels.StateOpen},
	}
	before, err := h.svc.Head(as(lead()), room)
	require.NoError(t, err)
	rev := before.Channel.Rev
	for _, st := range steps {
		got, err := h.svc.Lifecycle(as(lead()), room, st.op)
		require.NoError(t, err, st.op)
		require.Equal(t, st.want, got.State)
		require.Greater(t, got.Rev, rev)
		rev = got.Rev
		require.Equal(t, []channels.Key{roomKey()}, takeKeys(t, sub), st.op)
		trail, err := h.store.Audit(context.Background(), "TKT-1")
		require.NoError(t, err)
		last := trail[len(trail)-1]
		require.Equal(t, channels.AuditLifecycle, last.Op)
		require.Equal(t, string(st.want), last.To)
		require.Equal(t, "lead", last.By.User)
	}
}

// TestLifecycle_ConcurrentCAS pins AC6: two concurrent identical
// operations give one success and one 409.
func TestLifecycle_ConcurrentCAS(t *testing.T) {
	t.Parallel()
	h := newLifecycleHarness(t)
	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i := range errs {
		wg.Go(func() {
			_, errs[i] = h.svc.Lifecycle(as(lead()), room, realtimecfg.LifecycleClose)
		})
	}
	wg.Wait()
	ok := 0
	for _, err := range errs {
		if err == nil {
			ok++
			continue
		}
		d, isDenial := errors.AsType[*DenialError](err)
		require.True(t, isDenial, err)
		require.Equal(t, http.StatusConflict, d.Status)
		require.Contains(t, []string{TypeLifecycleWrongState, TypeConflict}, d.Type)
	}
	require.Equal(t, 1, ok)
}

// TestLifecycle_DeleteRacesChange pins that a delete refuses a channel
// that was reopened and closed again after its checks read it: the close
// time its retention counts from moved, so the delete is a 409 conflict.
func TestLifecycle_DeleteRacesChange(t *testing.T) {
	t.Parallel()
	h := newLifecycleHarness(t)
	h.moveRoom(t, channels.StateClosed)
	h.clock.Add(8 * 24 * time.Hour)
	h.store.onDelete = func() {
		h.store.onDelete = nil
		h.moveRoom(t, channels.StateOpen)
		h.moveRoom(t, channels.StateClosed)
	}
	_, err := h.svc.Lifecycle(as(lead()), room, realtimecfg.LifecycleDelete)
	requireDenial(t, err, http.StatusConflict, TypeConflict)
	_, err = h.store.Head(context.Background(), roomKey())
	require.NoError(t, err, "the channel was not deleted")
}

// TestLifecycle_RateLimited pins that lifecycle operations share the
// per-channel operation rate.
func TestLifecycle_RateLimited(t *testing.T) {
	t.Parallel()
	h := newLifecycleHarness(t)
	var err error
	for range 100 {
		if _, err = h.svc.Lifecycle(as(lead()), room, realtimecfg.LifecycleClose); err != nil {
			break
		}
		if _, err = h.svc.Lifecycle(as(lead()), room, realtimecfg.LifecycleReopen); err != nil {
			break
		}
	}
	requireDenial(t, err, http.StatusTooManyRequests, TypeRateLimited)
}

// agreeTarget is one channel the agree-test runs every operation on.
type agreeTarget struct {
	name string
	// setup prepares a fresh harness and returns the channel's address.
	setup func(t *testing.T, h *harness) RawAddress
}

// agreeTargets lists the room in each state with its condition true or
// false, the room never written, and a calls channel open or closed.
func agreeTargets() []agreeTarget {
	var out []agreeTarget
	for _, st := range []channels.State{channels.StateOpen, channels.StateClosed, channels.StateArchived} {
		for _, status := range []string{"done", "open"} {
			out = append(out, agreeTarget{
				name: fmt.Sprintf("room %s status %s", st, status),
				setup: func(t *testing.T, h *harness) RawAddress {
					t.Helper()
					h.post(t, member(), PostInput{Type: "text"})
					if st == channels.StateArchived {
						h.moveRoom(t, channels.StateClosed)
					}
					h.moveRoom(t, st)
					h.anchors.setStatus(status)
					return room
				},
			})
		}
	}
	out = append(out, agreeTarget{
		name:  "room never written",
		setup: func(*testing.T, *harness) RawAddress { return room },
	})
	for _, st := range []channels.State{channels.StateOpen, channels.StateClosed} {
		out = append(out, agreeTarget{
			name: fmt.Sprintf("calls %s", st),
			setup: func(t *testing.T, h *harness) RawAddress {
				t.Helper()
				ch, err := h.svc.Create(as(member()), "ticket", "TKT-1", "calls", CreateInput{})
				require.NoError(t, err)
				if st != channels.StateOpen {
					_, err = h.store.SetLifecycle(context.Background(), channels.Key{
						Anchor: channels.Anchor{ID: "TKT-1"}, Type: "calls", Name: ch.Name,
					}, channels.StateOpen,
						channels.Transition{To: st, By: channels.Author{User: "setup"}, At: h.clock.Now()})
					require.NoError(t, err)
				}
				return RawAddress{EntityType: "ticket", Anchor: "TKT-1", Type: "calls", Name: ch.Name}
			},
		})
	}
	return out
}

// TestLifecycle_AffordanceAndGateAgree pins AC7: for every caller, channel,
// state, condition outcome, retention age and degraded flag, the head
// offers an operation exactly when running it succeeds.
func TestLifecycle_AffordanceAndGateAgree(t *testing.T) {
	t.Parallel()
	callers := []struct {
		name string
		get  func() *fakeAccess
	}{
		{"lead", lead},
		{"member", member},
		{"readonly", func() *fakeAccess { return readOnlyClient(lead()) }},
	}
	ages := []time.Duration{time.Hour, 60 * 24 * time.Hour}
	for _, target := range agreeTargets() {
		for _, caller := range callers {
			for _, age := range ages {
				for _, degraded := range []bool{false, true} {
					name := fmt.Sprintf("%s/%s/age %s/degraded %v", target.name, caller.name, age, degraded)
					t.Run(name, func(t *testing.T) {
						t.Parallel()
						setup := func(t *testing.T) (*harness, RawAddress) {
							t.Helper()
							h := newHarness(t, withYAML(lifecycleRealtime))
							addr := target.setup(t, h)
							h.clock.Add(age)
							if degraded {
								h.cfg.set(Snapshot{Config: h.cfg.Load().Config, Degraded: true})
							}
							return h, addr
						}
						h, addr := setup(t)
						offered := h.offered(t, caller.get(), addr)
						for _, op := range realtimecfg.LifecycleOpNames() {
							h, addr := setup(t)
							_, err := h.svc.Lifecycle(as(caller.get()), addr, op)
							require.Equalf(t, slices.Contains(offered, op), err == nil,
								"%s: offered %v, got %v", op, offered, err)
							if err != nil {
								d, ok := errors.AsType[*DenialError](err)
								require.True(t, ok, err)
								require.GreaterOrEqual(t, d.Status, 400)
								require.Less(t, d.Status, 500)
							}
						}
					})
				}
			}
		}
	}
}
