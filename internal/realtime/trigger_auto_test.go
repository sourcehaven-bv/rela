package realtime

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/entity"
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
		Rows: th.src, Seen: th.kv, DueWake: NewDueWake(),
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

// statusRealtime adds to autoRealtime a trigger that posts on every status
// change, so it is in its cooldown when the auto rule's edge comes.
const statusRealtime = autoRealtime + `
  status-note:
    entity_type: ticket
    on: change
    when: "entity.status ~= ''"
    channel: room
    post: { type: text, props: { body: "Status" } }
`

func TestAuto_WaitsForTriggerInCooldown(t *testing.T) {
	th := newTriggerHarness(t, statusRealtime)
	th.ticket(1, map[string]any{"status": "doing"})
	th.openRoom(t, "TKT-1")
	th.reconcile(t)
	th.ticket(1, map[string]any{"status": "review"})
	th.round(t)
	require.Equal(t, []string{"hi", "Status"}, th.bodies(t, "TKT-1"))

	th.clock.Add(realtimelimits.TriggerCooldown / 2)
	th.ticket(1, map[string]any{"status": "done"})
	require.Zero(t, th.round(t).Moved, "the status note in its cooldown goes first")
	require.Equal(t, channels.StateOpen, th.state(t, roomOf("TKT-1")))

	th.clock.Add(realtimelimits.TriggerCooldown / 2)
	require.Equal(t, 1, th.round(t).Moved)
	require.Equal(t, []string{"hi", "Status", "Done", "Status"}, th.bodies(t, "TKT-1"))
	require.Equal(t, channels.StateClosed, th.state(t, roomOf("TKT-1")))
}

// failOnce fails the first Append, or the second SetLifecycle, once.
type failOnce struct {
	channels.Store
	appends, sets atomic.Int64
	append, set   bool
}

func (s *failOnce) Append(ctx context.Context, k channels.Key, nm channels.NewMessage) (channels.Message, error) {
	if s.append && s.appends.Add(1) == 1 {
		return channels.Message{}, errors.New("store down")
	}
	return s.Store.Append(ctx, k, nm)
}

func (s *failOnce) SetLifecycle(
	ctx context.Context, k channels.Key, expect channels.State, tr channels.Transition,
) (channels.Channel, error) {
	if s.set && s.sets.Add(1) == 2 {
		return channels.Channel{}, errors.New("store down")
	}
	return s.Store.SetLifecycle(ctx, k, expect, tr)
}

// useStore points the harness's engine at st.
func (th *triggerHarness) useStore(t *testing.T, st channels.Store) {
	t.Helper()
	trg, err := NewTriggers(TriggerDeps{
		Store: st, Config: th.cfg, Local: th.local, Meta: testMeta(t), Rows: th.src, Seen: th.kv, DueWake: NewDueWake(),
	})
	require.NoError(t, err)
	th.trg = trg
}

func TestAuto_WaitsForTriggerRetry(t *testing.T) {
	th := autoHarness(t)
	th.useStore(t, &failOnce{Store: th.store, append: true})
	th.ticket(1, map[string]any{"status": "done"})
	_, err := th.trg.RunRound(context.Background())
	require.NoError(t, err)
	_, err = th.trg.RunRound(context.Background())
	require.NoError(t, err)
	require.Equal(t, channels.StateOpen, th.state(t, roomOf("TKT-1")), "the trigger post waiting for a retry goes first")

	th.clock.Add(time.Second)
	require.Equal(t, 1, th.round(t).Moved)
	require.Equal(t, []string{"hi", "Done"}, th.bodies(t, "TKT-1"))
	require.Equal(t, channels.StateClosed, th.state(t, roomOf("TKT-1")))
}

// closedThreads creates n closed threads on TKT-1.
func (th *triggerHarness) closedThreads(t *testing.T, n int) []channels.Key {
	t.Helper()
	keys := make([]channels.Key, n)
	for i := range n {
		c, err := th.store.CreateMany(context.Background(), channels.Anchor{ID: "TKT-1"}, channels.NewChannel{
			Type: "threads", Title: fmt.Sprintf("t%d", i), By: channels.Author{User: "alice"}, At: th.clock.Now(),
		})
		require.NoError(t, err)
		th.move(t, c.Key, channels.StateOpen, channels.StateClosed)
		keys[i] = c.Key
	}
	return keys
}

func noTriggerAuto(t *testing.T) *triggerHarness {
	t.Helper()
	noTrigger, _, _ := strings.Cut(autoRealtime, "triggers:")
	th := newTriggerHarness(t, noTrigger)
	th.ticket(1, map[string]any{"status": "doing"})
	th.reconcile(t)
	return th
}

func TestAuto_FailedMoveResumes(t *testing.T) {
	th := noTriggerAuto(t)
	keys := th.closedThreads(t, 3)
	th.useStore(t, &failOnce{Store: th.store, set: true})
	th.ticket(1, map[string]any{"status": "done"})
	rep, err := th.trg.RunRound(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, rep.Moved, "the second move fails")

	th.clock.Add(time.Second)
	require.Equal(t, 2, th.round(t).Moved, "the retry moves the rest")
	for _, k := range keys {
		require.Equal(t, channels.StateArchived, th.state(t, k))
	}
}

func TestAuto_MovesCountAgainstTheCap(t *testing.T) {
	th := noTriggerAuto(t)
	n := realtimelimits.MaxSystemPostsPerRound + 6
	keys := th.closedThreads(t, n)
	th.ticket(1, map[string]any{"status": "done"})
	// The room rule finds no room, which still costs one attempt; each
	// thread moved costs one more.
	require.Equal(t, realtimelimits.MaxSystemPostsPerRound-1, th.round(t).Moved)
	require.True(t, th.trg.Pending(), "the rest wait at the head of the queue")
	require.Equal(t, 7, th.round(t).Moved)
	for _, k := range keys {
		require.Equal(t, channels.StateArchived, th.state(t, k))
	}
}

const faceAutoRealtime = `
version: 1
channel_types:
  facechat:
    attach_to: [page]
    anchor: face
    roles:
      member: { from: attached }
    message_types:
      text:
        properties: { body: { type: string } }
        post: [member]
    lifecycle:
      close: { roles: [member], auto: { when: "entity.title == 'done'" } }
`

func TestAuto_FaceAnchorClosesTheEnteredFace(t *testing.T) {
	th := newTriggerHarness(t, faceAutoRealtime)
	ctx := context.Background()
	chat := func(face entity.Face) channels.Key {
		return channels.Key{Anchor: channels.Anchor{ID: "PAGE-1", Face: face}, Type: "facechat"}
	}
	for _, face := range []entity.Face{"published", "draft"} {
		th.src.set("page", "PAGE-1", face, map[string]any{"title": "wip"})
		_, err := th.store.Store.Append(ctx, chat(face), channels.NewMessage{
			Type: "text", Author: channels.Author{User: "alice"}, At: th.clock.Now(), Props: map[string]any{"body": "hi"},
		})
		require.NoError(t, err)
	}
	th.reconcile(t)
	th.src.set("page", "PAGE-1", "draft", map[string]any{"title": "done"})
	th.trg.Note(Change{EntityType: "page", ID: "PAGE-1"})
	require.Equal(t, 1, th.round(t).Moved)
	require.Equal(t, channels.StateClosed, th.state(t, chat("draft")))
	require.Equal(t, channels.StateOpen, th.state(t, chat("published")))
}
