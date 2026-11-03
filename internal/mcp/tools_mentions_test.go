package mcp

import (
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/realtime"
	"github.com/Sourcehaven-BV/rela/internal/realtimecfg"
)

// mentionTestYAML adds to inboxTestYAML a huddle on a requirement whose
// note has a markdown body, so a post can mention someone. No acl.yaml
// maps logins to entities, so a mention is `@login`.
var mentionTestYAML = inboxTestYAML + `  huddle:
    attach_to: [requirement]
    read: { from: attached }
    roles:
      member: { from: attached }
    message_types:
      note: { properties: { body: { type: markdown } }, post: [member] }
`

// useMentions swaps in mentionTestYAML.
func (f *channelFixture) useMentions(t *testing.T) {
	t.Helper()
	compiled, err := realtimecfg.Parse([]byte(mentionTestYAML), f.meta, nil)
	if err != nil {
		t.Fatal(err)
	}
	f.cfg.Store(compiled)
}

// noteAsBob posts body as a huddle note on anchor as bob.
func (f *channelFixture) noteAsBob(t *testing.T, anchor, body string) {
	t.Helper()
	_, err := f.svc.Post(f.asBob(t), realtime.RawAddress{Anchor: anchor, Type: "huddle"},
		realtime.PostInput{Type: "note", Props: map[string]any{"body": body}})
	if err != nil {
		t.Fatalf("note in the huddle of %s as bob: %v", anchor, err)
	}
}

// TestListInbox_Mentions pins TKT-6RXAC0 AC 16: list_inbox lists a
// mention of the caller with mention_count, and mark_read clears it.
func TestListInbox_Mentions(t *testing.T) {
	t.Parallel()
	f := newChannelFixture(t)
	f.useMentions(t)
	f.noteAsBob(t, "REQ-002", "`@alice` please look")
	cs := f.client(t, "alice")
	var got struct {
		MentionCount *int                    `json:"mention_count"`
		Mentions     []realtime.InboxMention `json:"mentions"`
	}
	mustCall(t, cs, toolListInbox, map[string]any{"kinds": []string{"mention"}}, &got)
	if got.MentionCount == nil || *got.MentionCount != 1 || len(got.Mentions) != 1 {
		t.Fatalf("list_inbox mentions = %+v", got)
	}
	m := got.Mentions[0]
	if m.Kind != realtime.InboxKindMention || m.Channel.Anchor != "REQ-002" || m.Seq != 1 || m.Author.User != "bob" {
		t.Errorf("mention entry = %+v", m)
	}
	var read realtime.MarkReadResult
	mustCall(t, cs, toolMarkRead, map[string]any{"id": "REQ-002", chanTypeArg: "huddle"}, &read)
	got.Mentions, got.MentionCount = nil, nil
	mustCall(t, cs, toolListInbox, map[string]any{"kinds": []string{"mention"}}, &got)
	if got.MentionCount == nil || *got.MentionCount != 0 {
		t.Errorf("list_inbox after mark_read = %+v, want no mentions", got)
	}
}

// TestWatch_ListInboxWakesOnMention pins TKT-6RXAC0 AC 16: a post that
// mentions the caller wakes a list_inbox watch, though its channel is in
// no window the watch knows.
func TestWatch_ListInboxWakesOnMention(t *testing.T) {
	t.Parallel()
	f := newChannelFixture(t)
	f.useMentions(t)
	cs := connect(t, inboxWatchClient(t, f, "alice", time.Hour, time.Hour, false))
	base := mustWatch(t, cs, watchArgs("", 0, inboxTarget))
	done := waitChanged(cs, watchArgs(base.Cursor, 20, inboxTarget))
	time.Sleep(200 * time.Millisecond)
	f.noteAsBob(t, "REQ-002", "`@alice` please look")
	waitInbox(t, done, "a mention of alice")
}

// TestWatch_ListInboxNoWakeOnOthersMention pins TKT-6RXAC0 AC 16: a
// mention of someone else does not wake the caller's watch.
func TestWatch_ListInboxNoWakeOnOthersMention(t *testing.T) {
	t.Parallel()
	f := newChannelFixture(t)
	f.useMentions(t)
	cs := connect(t, inboxWatchClient(t, f, "alice", time.Hour, time.Hour, false))
	base := mustWatch(t, cs, watchArgs("", 0, inboxTarget))
	done := waitChanged(cs, watchArgs(base.Cursor, 3, inboxTarget))
	time.Sleep(200 * time.Millisecond)
	f.noteAsBob(t, "REQ-002", "`@carol` please look")
	if got := <-done; !got.TimedOut || len(got.Changed) != 0 {
		t.Errorf("watch after a mention of carol = %+v, want a timeout", got)
	}
}

// TestWatch_ListInboxKindsIgnoreMention pins TKT-6RXAC0 AC 16: a watch
// whose kinds hold neither mention nor reply does not wake on a mention.
func TestWatch_ListInboxKindsIgnoreMention(t *testing.T) {
	t.Parallel()
	f := newChannelFixture(t)
	f.useMentions(t)
	target := `{"tool":"list_inbox","args":{"kinds":["waiting","unread"]}}`
	cs := connect(t, inboxWatchClient(t, f, "alice", time.Hour, time.Hour, false))
	base := mustWatch(t, cs, watchArgs("", 0, target))
	done := waitChanged(cs, watchArgs(base.Cursor, 3, target))
	time.Sleep(200 * time.Millisecond)
	f.noteAsBob(t, "REQ-002", "`@alice` please look")
	if got := <-done; !got.TimedOut || len(got.Changed) != 0 {
		t.Errorf("waiting and unread watch after a mention = %+v, want a timeout", got)
	}
}

func TestAsksMentions(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		args map[string]any
		want bool
	}{
		{map[string]any{}, true},
		{map[string]any{"kinds": nil}, true},
		{map[string]any{"kinds": "mention"}, true},
		{map[string]any{"kinds": []any{"waiting"}}, false},
		{map[string]any{"kinds": []any{"waiting", "reply"}}, true},
		{map[string]any{"kinds": []any{"mention"}}, true},
	} {
		if got := asksMentions(tc.args); got != tc.want {
			t.Errorf("asksMentions(%v) = %v, want %v", tc.args, got, tc.want)
		}
	}
}
