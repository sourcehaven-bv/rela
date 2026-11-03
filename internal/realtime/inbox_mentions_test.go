package realtime

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/channels"
)

func roomAt(id string) RawAddress { return RawAddress{EntityType: "ticket", Anchor: id, Type: "room"} }

// mentionAlice has bob mention alice by login in the room of id.
func (h *harness) mentionAlice(t *testing.T, id string) {
	t.Helper()
	h.postTo(t, user("bob"), roomAt(id), "text", props("body", "`@alice` look"))
}

// TestInbox_Mentions pins the mention and reply entries (TKT-6RXAC0 AC
// 10): newest first, one per message with the mention kind preferred, the
// count, the wake keys, and none on a cursor page.
func TestInbox_Mentions(t *testing.T) {
	t.Parallel()
	h := newMentionHarness(t, false)
	alice := user("alice")
	parent := h.postTo(t, alice, roomAt("TKT-1"), "text", props("body", "question"))
	h.clock.Add(time.Second)
	h.ticket("TKT-2", nil)
	h.mentionAlice(t, "TKT-2")
	h.clock.Add(time.Second)
	reply := PostInput{Type: "text", Props: props("body", "`@alice` and a reply"), ReplyTo: parent.Message.Seq}
	both, err := h.svc.Post(as(user("bob")), roomAt("TKT-1"), reply)
	require.NoError(t, err)
	h.clock.Add(time.Second)
	onlyReply := PostInput{Type: "text", Props: props("body", "just a reply"), ReplyTo: parent.Message.Seq}
	last, err := h.svc.Post(as(user("carol")), roomAt("TKT-1"), onlyReply)
	require.NoError(t, err)

	h.store.addressedLists.Store(0)
	got := h.inbox(t, alice, InboxRequest{})
	require.Equal(t, int64(1), h.store.addressedLists.Load())
	require.NotNil(t, got.MentionCount)
	require.Equal(t, 3, *got.MentionCount)
	require.Len(t, got.Mentions, 3)
	require.Equal(t, InboxKindReply, got.Mentions[0].Kind)
	require.Equal(t, last.Message.Seq, got.Mentions[0].Seq)
	require.Equal(t, "carol", got.Mentions[0].Author.User)
	require.Equal(t, parent.Message.Seq, got.Mentions[0].ReplyTo)
	require.Equal(t, InboxKindMention, got.Mentions[1].Kind, "a mention row wins over the reply row")
	require.Equal(t, both.Message.Seq, got.Mentions[1].Seq)
	require.Equal(t, InboxMention{
		Kind: InboxKindMention, Seq: 1, At: got.Mentions[2].At, MessageType: "text",
		Author: AuthorView{User: "bob", Tool: "data-entry"},
		Channel: InboxChannel{Anchor: "TKT-2", EntityType: "ticket", AnchorTitle: "Title of TKT-2", Type: "room",
			State: channels.StateOpen},
	}, got.Mentions[2])
	require.Contains(t, got.WakeKeys, channels.Key{Anchor: channels.Anchor{ID: "TKT-2"}, Type: "room"})

	replies := h.inbox(t, alice, InboxRequest{Kinds: []string{InboxKindReply}})
	require.Equal(t, 2, *replies.MentionCount)
	for _, m := range replies.Mentions {
		require.Equal(t, InboxKindReply, m.Kind)
	}

	_, err = h.svc.MarkRead(as(alice), roomAt("TKT-2"), nil)
	require.NoError(t, err)
	require.Equal(t, 2, *h.inbox(t, alice, InboxRequest{}).MentionCount, "a marker at the seq clears the entry")

	_, err = h.svc.Inbox(as(alice), InboxRequest{Kinds: []string{InboxKindMention},
		Cursor: cursorOf(t, inboxCursor{V: 1, Since: h.clock.Now(), ID: "TKT-1", Type: "room"})})
	requireDenial(t, err, http.StatusBadRequest, TypeInvalidCursor)

	empty := h.inbox(t, user("dave"), InboxRequest{Kinds: []string{InboxKindMention}})
	require.Contains(t, inboxJSON(t, empty), `"mention_count":0`)
	require.Contains(t, inboxJSON(t, empty), `"mentions":[]`)
}

// TestInbox_MentionKindsCalls pins that kinds=mention makes no holder
// query, no ListWaiting and no ListReads (AC 10).
func TestInbox_MentionKindsCalls(t *testing.T) {
	t.Parallel()
	h := newMentionHarness(t, false)
	h.mentionAlice(t, "TKT-1")
	h.store.listReads.Store(0)
	h.store.waitingLists.Store(0)
	h.holders.calls.Store(0)
	got := h.inbox(t, user("alice"), InboxRequest{Kinds: []string{InboxKindMention}})
	require.Equal(t, 1, *got.MentionCount)
	require.Zero(t, h.store.listReads.Load())
	require.Zero(t, h.store.waitingLists.Load())
	require.Zero(t, h.holders.calls.Load())
	require.Nil(t, got.UnreadCount)
	require.False(t, got.HasWaiting())
}

// TestInbox_MentionHiddenChangesNothing pins gate before count (AC 11):
// rows on an anchor the caller cannot read, in a channel whose read role
// the caller fails, and of a channel type the configuration no longer
// declares change no byte of the answer.
func TestInbox_MentionHiddenChangesNothing(t *testing.T) {
	t.Parallel()
	seed := func(t *testing.T, h *harness) {
		t.Helper()
		for i := range 5 {
			id := fmt.Sprintf("TKT-%d", 100+i)
			h.ticket(id, nil)
			h.mentionAlice(t, id)
			h.clock.Add(time.Second)
		}
	}
	plain := newMentionHarness(t, false)
	seed(t, plain)

	// The hidden rows are posted first, at the same clock time, so the
	// visible rows keep their times.
	mixed := newMentionHarness(t, false)
	mixed.ticket("TKT-900", nil)
	mixed.mentionAlice(t, "TKT-900")
	mixed.postTo(t, user("bob", "chat.secret"), secretRoom, "text", props("body", "`@alice` secret"))
	seed(t, mixed)
	mixed.anchors.mu.Lock()
	delete(mixed.anchors.views, "TKT-900")
	mixed.anchors.mu.Unlock()

	alice := user("alice")
	require.Equal(t, inboxJSON(t, plain.inbox(t, alice, InboxRequest{})),
		inboxJSON(t, mixed.inbox(t, alice, InboxRequest{})))

	gone := newMentionHarness(t, false)
	gone.postTo(t, user("bob", "chat.secret"), secretRoom, "text", props("body", "`@alice` secret"))
	seed(t, gone)
	gone.cfg.set(Snapshot{Config: testCompiled(t, mentionRealtimeWithoutSecret())})
	require.Equal(t, inboxJSON(t, plain.inbox(t, alice, InboxRequest{})),
		inboxJSON(t, gone.inbox(t, user("alice", "chat.secret"), InboxRequest{})),
		"a channel type the configuration no longer declares yields no entry")
}

// storeMentions writes n rows that mention alice, by bob, straight into
// the store: per rows in the room of each ticket from TKT-<first> on, at
// times after from. It bypasses the post path, so the test is fast and no
// rate limit applies.
func (h *harness) storeMentions(t *testing.T, first, n, per int, from time.Time) {
	t.Helper()
	for i := range n {
		id := fmt.Sprintf("TKT-%d", first+i/per)
		_, err := h.store.Append(context.Background(), channels.Key{Anchor: channels.Anchor{ID: id}, Type: "room"},
			channels.NewMessage{
				Type: "text", Author: channels.Author{User: "bob", Tool: "data-entry"},
				At: from.Add(time.Duration(i+1) * time.Second), Props: props("body", "`@alice` look"),
				Mentions: []channels.Mention{{Target: "alice", Form: channels.FormLogin}},
			})
		require.NoError(t, err)
	}
}

// TestInbox_MentionsGateBeforeCut pins that rows the caller may not read
// never push a visible row out of the inbox (review finding 4): more
// unread rows than the old 500-row window, all newer and all in channels
// alice cannot read, leave her one visible mention listed and counted.
// The only cut before the gate is the documented scan of
// [channels.MaxAddressedScan] rows.
func TestInbox_MentionsGateBeforeCut(t *testing.T) {
	t.Parallel()
	h := newMentionHarness(t, false)
	h.mentionAlice(t, "TKT-1")
	const hidden, per = 600, 100
	h.storeMentions(t, 300, hidden, per, h.clock.Now())
	h.anchors.mu.Lock()
	for i := range hidden / per {
		delete(h.anchors.views, fmt.Sprintf("TKT-%d", 300+i))
	}
	h.anchors.mu.Unlock()

	got := h.inbox(t, user("alice"), InboxRequest{Kinds: []string{InboxKindMention}})
	require.Equal(t, 1, *got.MentionCount)
	require.Len(t, got.Mentions, 1)
	require.Equal(t, "TKT-1", got.Mentions[0].Channel.Anchor)
}

// TestInbox_ReplyKindPastMentions pins that kinds=reply filters inside the
// store scan (review finding 2): an unread reply older than more unread
// mentions than the old 500-row window is still listed.
func TestInbox_ReplyKindPastMentions(t *testing.T) {
	t.Parallel()
	h := newMentionHarness(t, false)
	alice := user("alice")
	parent := h.postTo(t, alice, roomAt("TKT-1"), "text", props("body", "question"))
	reply := PostInput{Type: "text", Props: props("body", "an answer"), ReplyTo: parent.Message.Seq}
	answer, err := h.svc.Post(as(user("carol")), roomAt("TKT-1"), reply)
	require.NoError(t, err)
	const mentions, per = 600, 100
	for i := range mentions / per {
		h.ticket(fmt.Sprintf("TKT-%d", 400+i), nil)
	}
	h.storeMentions(t, 400, mentions, per, h.clock.Now())

	got := h.inbox(t, alice, InboxRequest{Kinds: []string{InboxKindReply}})
	require.Equal(t, 1, *got.MentionCount)
	require.Len(t, got.Mentions, 1)
	require.Equal(t, InboxKindReply, got.Mentions[0].Kind)
	require.Equal(t, answer.Message.Seq, got.Mentions[0].Seq)

	all := h.inbox(t, alice, InboxRequest{Kinds: []string{InboxKindMention, InboxKindReply}})
	require.Equal(t, mentions+1, *all.MentionCount, "every unread row in the scan is counted")
	require.Len(t, all.Mentions, MaxInboxMentions)
}

// mentionRealtimeWithoutSecret is mentionRealtime without the secret type.
func mentionRealtimeWithoutSecret() string {
	before, _, _ := strings.Cut(mentionRealtime, "  secret:")
	return before
}

// noUnreadMentionRealtime is mentionRealtime with two room types that have
// `unread: false`: progress, with a markdown body, and silent, whose ref
// property says notify: false (TKT-O6RC8Z).
var noUnreadMentionRealtime = strings.Replace(mentionRealtime,
	"      about:   { properties: { on: { type: ref, to: [ticket] } }, post: [member] }\n",
	"      about:   { properties: { on: { type: ref, to: [ticket] } }, post: [member] }\n"+
		"      progress: { properties: { body: { type: markdown } }, post: [member], unread: false }\n"+
		"      silent: { properties: { to: { type: ref, to: [person], notify: false } }, post: [member], "+
		"unread: false }\n", 1)

// newNoUnreadMentionHarness is newMentionHarness over
// noUnreadMentionRealtime; mapping maps logins to persons.
func newNoUnreadMentionHarness(t *testing.T, mapping bool) *harness {
	t.Helper()
	h := newMentionHarness(t, mapping)
	var policy *acl.Policy
	if mapping {
		policy = personMappingPolicy()
	}
	h.cfg.set(Snapshot{Config: testCompiledWith(t, noUnreadMentionRealtime, policy)})
	return h
}

// TestInbox_MentionInNoUnreadType pins that a mention in a message of a
// type with `unread: false` still reaches the inbox with its seq, while
// the channel's unread count stays 0; notify: false on a property still
// keeps the mention out (TKT-O6RC8Z AC 6).
func TestInbox_MentionInNoUnreadType(t *testing.T) {
	t.Parallel()
	h := newNoUnreadMentionHarness(t, false)
	bob, alice := user("bob"), user("alice")
	markRead(t, h, alice, room, nil)
	v := h.post(t, bob, PostInput{Type: "progress", Props: props("body", "`@alice` look")})
	require.NotEmpty(t, storedMentions(t, h, v.Seq))
	got := h.inbox(t, alice, InboxRequest{})
	require.Equal(t, 1, *got.MentionCount)
	require.Equal(t, InboxKindMention, got.Mentions[0].Kind)
	require.Equal(t, v.Seq, got.Mentions[0].Seq)
	require.Equal(t, "progress", got.Mentions[0].MessageType)
	require.Zero(t, *got.UnreadCount, "the message does not count as unread")

	mapped := newNoUnreadMentionHarness(t, true)
	poster := user("USR-3")
	ho := mapped.post(t, poster, PostInput{Type: "handoff", Props: map[string]any{"to": "USR-7"}})
	require.NotEmpty(t, storedMentions(t, mapped, ho.Seq), "a ref to the user type mentions")
	s := mapped.post(t, poster, PostInput{Type: "silent", Props: map[string]any{"to": "USR-7"}})
	require.Nil(t, storedMentions(t, mapped, s.Seq), "notify: false still keeps the mention out")
}

// TestInbox_ReplyInNoUnreadType pins that a reply of a type with `unread:
// false` still writes the parent author's reply row (TKT-O6RC8Z AC 6).
func TestInbox_ReplyInNoUnreadType(t *testing.T) {
	t.Parallel()
	h := newNoUnreadMentionHarness(t, false)
	alice := user("alice")
	parent := h.post(t, alice, PostInput{Type: "text", Props: props("body", "question")})
	h.clock.Add(time.Second)
	reply, err := h.svc.Post(as(user("bob")), room,
		PostInput{Type: "progress", Props: props("body", "working on it"), ReplyTo: parent.Seq})
	require.NoError(t, err)
	got := h.inbox(t, alice, InboxRequest{})
	require.Equal(t, 1, *got.MentionCount)
	require.Equal(t, InboxKindReply, got.Mentions[0].Kind)
	require.Equal(t, reply.Message.Seq, got.Mentions[0].Seq)
	require.Equal(t, parent.Seq, got.Mentions[0].ReplyTo)
	require.Zero(t, *got.UnreadCount, "the reply does not count as unread")
}
