package realtime

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/realtimelimits"
)

// autoRealtime closes a ticket's room, and archives its closed threads,
// when the ticket is done. The finished trigger posts into the room on the
// same edge.
const autoRealtime = `
version: 1
channel_types:
  room:
    attach_to: [ticket]
    roles:
      member: { from: attached }
    message_types:
      text:
        properties: { body: { type: string } }
        post: [member]
    lifecycle:
      close:  { roles: [member], auto: { when: "entity.status == 'done'" } }
      reopen: { roles: [member] }
  threads:
    attach_to: [ticket]
    cardinality: many
    create: { roles: [member] }
    roles:
      member: { from: attached }
    message_types:
      text:
        properties: { body: { type: string } }
        post: [member]
    lifecycle:
      archive: { roles: [member], from: closed, auto: { when: "entity.status == 'done'" } }
triggers:
  finished:
    entity_type: ticket
    on: enter
    when: "entity.status == 'done'"
    channel: room
    post: { type: text, props: { body: "Done" } }
`

func roomOf(id string) channels.Key {
	return channels.Key{Anchor: channels.Anchor{ID: id}, Type: "room"}
}

// openRoom writes the first message of id's room, which opens it.
func (th *triggerHarness) openRoom(t *testing.T, id string) {
	t.Helper()
	_, err := th.store.Store.Append(context.Background(), roomOf(id), channels.NewMessage{
		Type: "text", Author: channels.Author{User: "alice"}, At: th.clock.Now(), Props: map[string]any{"body": "hi"},
	})
	require.NoError(t, err)
}

func (th *triggerHarness) state(t *testing.T, k channels.Key) channels.State {
	t.Helper()
	c, err := th.store.Head(context.Background(), k)
	require.NoError(t, err)
	return c.State
}

func (th *triggerHarness) move(t *testing.T, k channels.Key, from, to channels.State) {
	t.Helper()
	_, err := th.store.SetLifecycle(context.Background(), k, from, channels.Transition{
		To: to, By: channels.Author{User: "alice"}, At: th.clock.Now(),
	})
	require.NoError(t, err)
}

// autoHarness is a trigger harness over autoRealtime with TKT-1 in
// progress, its room open and the rules baselined.
func autoHarness(t *testing.T) *triggerHarness {
	t.Helper()
	th := newTriggerHarness(t, autoRealtime)
	th.ticket(1, map[string]any{"status": "doing"})
	th.openRoom(t, "TKT-1")
	require.Equal(t, 3, th.reconcile(t).Baselined, "one trigger and two auto rules")
	return th
}

func TestAuto_CloseOnEnter(t *testing.T) {
	th := autoHarness(t)
	th.ticket(1, map[string]any{"status": "done"})
	rep := th.round(t)
	require.Equal(t, 1, rep.Moved)
	require.Equal(t, channels.StateClosed, th.state(t, roomOf("TKT-1")))
	require.Equal(t, []string{"hi", "Done"}, th.bodies(t, "TKT-1"),
		"the trigger on the same edge posts before the room closes")

	th.ticket(1, map[string]any{"status": "done", "title": "renamed"})
	require.Zero(t, th.round(t).Moved, "a change while the rule holds is no new edge")
}

func TestAuto_AuditTriggeredBy(t *testing.T) {
	th := autoHarness(t)
	th.ticket(1, map[string]any{"status": "done"})
	th.round(t)
	trail, err := th.store.Audit(context.Background(), "TKT-1")
	require.NoError(t, err)
	last := trail[len(trail)-1]
	require.Equal(t, channels.AuditLifecycle, last.Op)
	require.Equal(t, string(channels.StateClosed), last.To)
	require.Equal(t, principal.UserRealtime, last.By.User)
	require.Equal(t, "system", last.By.PrincipalType)
	require.Equal(t, "auto:room/close", last.TriggeredBy)
}

func TestAuto_HonoursFrom(t *testing.T) {
	th := autoHarness(t)
	ctx := context.Background()
	var keys []channels.Key
	for _, title := range []string{"open", "closed"} {
		c, err := th.store.CreateMany(ctx, channels.Anchor{ID: "TKT-1"}, channels.NewChannel{
			Type: "threads", Title: title, By: channels.Author{User: "alice"}, At: th.clock.Now(),
		})
		require.NoError(t, err)
		keys = append(keys, c.Key)
	}
	th.move(t, keys[1], channels.StateOpen, channels.StateClosed)

	th.ticket(1, map[string]any{"status": "done"})
	require.Equal(t, 2, th.round(t).Moved, "the room closes and the closed thread is archived")
	require.Equal(t, channels.StateOpen, th.state(t, keys[0]), "archive starts only from closed")
	require.Equal(t, channels.StateArchived, th.state(t, keys[1]))
}

func TestAuto_ReopenStands(t *testing.T) {
	th := autoHarness(t)
	th.ticket(1, map[string]any{"status": "done"})
	th.round(t)
	th.move(t, roomOf("TKT-1"), channels.StateClosed, channels.StateOpen)

	th.ticket(1, map[string]any{"status": "done", "title": "still done"})
	th.round(t)
	th.reconcile(t)
	require.Equal(t, channels.StateOpen, th.state(t, roomOf("TKT-1")), "a person's reopen stands while the rule holds")

	th.ticket(1, map[string]any{"status": "doing"})
	th.round(t)
	th.clock.Add(realtimelimits.TriggerCooldown)
	th.ticket(1, map[string]any{"status": "done"})
	require.Equal(t, 1, th.round(t).Moved, "the next enter closes it again")
	require.Equal(t, channels.StateClosed, th.state(t, roomOf("TKT-1")))
}

func TestAuto_NewRuleStartsFromNow(t *testing.T) {
	th := newTriggerHarness(t, autoRealtime)
	th.ticket(1, map[string]any{"status": "done"})
	th.openRoom(t, "TKT-1")
	rep := th.reconcile(t)
	require.Equal(t, 3, rep.Baselined)
	require.Zero(t, rep.Moved)
	require.Equal(t, channels.StateOpen, th.state(t, roomOf("TKT-1")),
		"an entity that already matches when the rule arrives keeps its channels")
}

func TestAuto_NeverWrittenChannelIsNotCreated(t *testing.T) {
	noTrigger, _, _ := strings.Cut(autoRealtime, "triggers:")
	th := newTriggerHarness(t, noTrigger)
	th.ticket(2, map[string]any{"status": "doing"})
	require.Equal(t, 2, th.reconcile(t).Baselined)
	th.ticket(2, map[string]any{"status": "done"})
	require.Zero(t, th.round(t).Moved)
	chs, err := th.store.ListEntity(context.Background(), "TKT-2")
	require.NoError(t, err)
	require.Empty(t, chs, "an auto rule moves channels; it creates none")
}

// staleList answers List with the channels as they were before a person
// moved them, as a list read just before a concurrent manual close would.
type staleList struct {
	channels.Store
	stale []channels.Channel
}

func (s *staleList) List(context.Context, channels.Anchor, string, channels.State) ([]channels.Channel, error) {
	return s.stale, nil
}

func TestAuto_CASConflictSkipped(t *testing.T) {
	th := autoHarness(t)
	ctx := context.Background()
	stale, err := th.store.List(ctx, channels.Anchor{ID: "TKT-1"}, "room", "")
	require.NoError(t, err)
	th.move(t, roomOf("TKT-1"), channels.StateOpen, channels.StateClosed)

	trg, err := NewTriggers(TriggerDeps{
		Store: &staleList{Store: th.store, stale: stale}, Config: th.cfg, Local: th.local, Meta: testMeta(t),
		Rows: th.src, Seen: th.kv,
	})
	require.NoError(t, err)
	th.trg = trg
	th.ticket(1, map[string]any{"status": "done"})
	rep := th.round(t)
	require.Zero(t, rep.Moved)
	require.Zero(t, rep.Failed, "a channel a person closed first is no failure")
	require.False(t, th.trg.Pending(), "nothing is retried")
	trail, err := th.store.Audit(ctx, "TKT-1")
	require.NoError(t, err)
	require.Empty(t, trail[len(trail)-1].TriggeredBy, "the person's close is the last record")
}

func TestAuto_WaitsForTriggerIntoSameType(t *testing.T) {
	th := newTriggerHarness(t, autoRealtime)
	rules := rulesFor(th.cfg.Load(), testMeta(t))
	require.Len(t, rules, 3)
	require.NotNil(t, rules[0].trigger, "triggers come first")
	byName := map[string]*rule{}
	for i := range rules {
		byName[rules[i].name] = &rules[i]
	}
	r := th.trg.newRound(context.Background(), th.cfg.Load(), th.clock.Now(), rules)
	th.trg.enqueue(byName["finished"].id, "TKT-1", queued{typ: "ticket", noted: th.clock.Now()})
	require.True(t, r.waitsForTrigger(byName["room.close.auto"], "TKT-1"))
	require.False(t, r.waitsForTrigger(byName["room.close.auto"], "TKT-2"))
	require.False(t, r.waitsForTrigger(byName["threads.archive.auto"], "TKT-1"),
		"a trigger into another channel type does not hold the rule back")
}
