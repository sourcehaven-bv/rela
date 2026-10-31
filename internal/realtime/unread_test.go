package realtime

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/channels"
)

// unreadRealtime is testRealtime with a log type progress in room that has
// `unread: false` (TKT-O6RC8Z).
var unreadRealtime = strings.Replace(testRealtime,
	"      typing: { persist: ephemeral, ttl: 5s, post: [member] }\n",
	"      progress: { properties: { body: { type: string } }, post: [member], unread: false }\n"+
		"      typing: { persist: ephemeral, ttl: 5s, post: [member] }\n", 1)

// TestPost_NoUnreadType pins that a person's post of a type with
// `unread: false` reaches the store with NewMessage.NoUnread, and a post of
// a counting type without it (AC 2). The flag comes only from the type:
// PostInput has no field for it.
func TestPost_NoUnreadType(t *testing.T) {
	t.Parallel()
	h := newHarness(t, withYAML(unreadRealtime))
	alice := user("alice")
	for _, tc := range []struct {
		typ      string
		noUnread bool
	}{{"progress", true}, {"text", false}} {
		_, err := h.svc.Post(as(alice), room, PostInput{Type: tc.typ, Props: props("body", "x")})
		require.NoError(t, err)
		require.Equal(t, tc.noUnread, h.store.lastAppend.Load().NoUnread, tc.typ)
	}
	msgs, err := h.store.Messages(context.Background(), roomKey(), channels.Page{Limit: 10})
	require.NoError(t, err)
	require.True(t, msgs[0].NoUnread, "the store keeps the flag")
	require.False(t, msgs[1].NoUnread)
}

// TestTriggerPost_NoUnreadType pins that a trigger's post of a type with
// `unread: false` is stored as non-counting (AC 2).
func TestTriggerPost_NoUnreadType(t *testing.T) {
	t.Parallel()
	yaml := unreadRealtime + `
triggers:
  opened:
    entity_type: ticket
    on: enter
    when: "entity.status == 'open'"
    channel: room
    post: { type: progress, props: { body: "Opened" } }
`
	th := newTriggerHarness(t, yaml)
	th.src.set("ticket", "TKT-1", "", map[string]any{"status": "new"})
	th.reconcile(t)
	th.ticket(1, map[string]any{"status": "open"})
	require.Equal(t, 1, th.round(t).Posted)
	require.True(t, th.store.lastAppend.Load().NoUnread)
	msgs, err := th.store.Messages(context.Background(), roomKey(), channels.Page{Limit: 10})
	require.NoError(t, err)
	require.Len(t, msgs, 1)
	require.True(t, msgs[0].NoUnread)
	require.False(t, channels.Counts(msgs[0]))
}

// TestReminderStep_NoUnreadType pins that the reminder step's Remind
// returns a post with NoUnread for a post type with `unread: false`, and
// without it for a counting one (AC 2).
func TestReminderStep_NoUnreadType(t *testing.T) {
	t.Parallel()
	quiet := strings.Replace(remindRealtime, "        post: [member]\n      escalate-stalled",
		"        post: [member]\n        unread: false\n      escalate-stalled", 1)
	require.NotEqual(t, remindRealtime, quiet)
	for _, tc := range []struct {
		name, yaml string
		noUnread   bool
	}{{"counting", remindRealtime, false}, {"non-counting", quiet, true}} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			s, r := remindSession(t, tc.yaml)
			ch := inState("contained", 1, t0, rec(-1, 0))
			ch.State = channels.StateOpen
			step := reminderStep(testChannelType(t, tc.yaml), s, r, ch.Session.Reminders["status-update"], nil,
				t0.Add(time.Hour))
			post, ok := step.Remind(ch)
			require.True(t, ok, "the reminder is due")
			require.Equal(t, "reminder", post.Type)
			require.Equal(t, tc.noUnread, post.NoUnread)
		})
	}
}

// TestRender_NoUnreadNotOnWire pins that the flag never reaches a client:
// the rendered message has no no_unread (AC 2, Q6).
func TestRender_NoUnreadNotOnWire(t *testing.T) {
	t.Parallel()
	h := newHarness(t, withYAML(unreadRealtime))
	posted := h.post(t, user("alice"), PostInput{Type: "progress", Props: props("body", "x")})
	page := h.readAll(t, user("bob"), room)
	require.Len(t, page.Messages, 1)
	for _, v := range []MessageView{posted, page.Messages[0]} {
		raw, err := json.Marshal(v)
		require.NoError(t, err)
		require.NotContains(t, string(raw), "no_unread")
	}
}
