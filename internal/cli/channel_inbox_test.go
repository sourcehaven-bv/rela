package cli

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	relamcp "github.com/Sourcehaven-BV/rela/internal/mcp"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/realtime"
)

const inboxCLIMetamodel = `version: "1.0"
entities:
  item:
    label: Item
    id_type: sequential
    id_prefix: ITEM-
    properties:
      title: {type: string, required: true}
      owner: {type: string}
relations: {}
`

// inboxCLIRealtime: a case waits for the item's owner; a thread waits for
// the one who started it.
const inboxCLIRealtime = `version: 1
channel_types:
  case:
    attach_to: [item]
    roles:
      lead:   { from: entity.owner, label: the lead }
      member: { from: attached }
    message_types:
      chat:   { properties: { body: { type: string } }, post: [member] }
      note:   { properties: { body: { type: markdown } }, post: [member] }
      answer: { properties: { note: { type: string } }, post: [lead] }
    session:
      initial: asked
      states:
        asked:
          label: Asked
          waiting_for: [lead]
          on: { answer: { to: done } }
        done: { final: true }
  threads:
    attach_to: [item]
    cardinality: many
    create: { roles: [member] }
    roles:
      starter: { creator: true, label: the starter }
      member:  { from: attached }
    message_types:
      wrap: { properties: { body: { type: string } }, post: [starter] }
    session:
      initial: open
      states:
        open: { waiting_for: [starter], on: { wrap: { to: wrapped } } }
        wrapped: { final: true }
`

const inboxCLIACL = `roles:
  editor:
    read: [item]
    create: [item]
    update: [item]
assignments:
  alice: editor
  "integration:triage-bot": editor
`

// newInboxProject writes a project whose first item is owned by alice and
// whose second by the job identity integration:triage-bot. It returns the
// services and the two item ids.
func newInboxProject(t *testing.T) (svc *appbuild.Services, items []string) {
	t.Helper()
	setBackendEnv(t)
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".rela"), 0o755))
	writeProjectFile(t, root, "metamodel.yaml", inboxCLIMetamodel)
	writeProjectFile(t, root, "realtime.yaml", inboxCLIRealtime)
	seed := openLeaseServices(t, root)
	for _, owner := range []string{"alice", "integration:triage-bot"} {
		e := entity.New("", "item")
		e.SetString("title", "owned by "+owner)
		e.SetString("owner", owner)
		created, err := seed.EntityManager().CreateEntity(leaseCtx(), e, entity.CreateOptions{})
		require.NoError(t, err)
		items = append(items, created.Entity.ID)
	}
	require.NoError(t, seed.Close())
	writeProjectFile(t, root, "acl.yaml", inboxCLIACL)
	return openLeaseServices(t, root), items
}

// TestChannelInbox_Text pins AC 15 for table output: one line per entry,
// the empty answer, and paging. The first page says when nothing is
// unread; a cursor page computes no unread and says nothing about it.
func TestChannelInbox_Text(t *testing.T) {
	svc, items := newInboxProject(t)

	got, err := runChannel(t, svc, "alice", "", "channel", "inbox")
	require.NoError(t, err)
	assert.Equal(t, "No session waits on you.\n\nNo unread channels.\n\nNo mentions or replies.\n", got)

	for _, id := range items {
		_, err = runChannel(t, svc, "alice", "", "channel", "post", id, "case", "chat", "-P", "body=hi")
		require.NoError(t, err)
	}
	got, err = runChannel(t, svc, "alice", "", "channel", "inbox")
	require.NoError(t, err)
	waiting, unread, ok := strings.Cut(got, "\n\n")
	require.True(t, ok, got)
	assert.Equal(t, "No unread channels.\n\nNo mentions or replies.\n", unread)
	lines := strings.Split(waiting, "\n")
	require.Len(t, lines, 1, got)
	prefix, rest, ok := strings.Cut(lines[0], " since ")
	require.True(t, ok, got)
	assert.Equal(t, items[0]+" case Asked, input_required", prefix)
	stamp, tail, _ := strings.Cut(rest, ", ")
	_, err = time.Parse(time.RFC3339, stamp)
	require.NoError(t, err, got)
	assert.Equal(t, "waiting for the lead (you)", tail)

	_, err = runChannel(t, svc, "alice", "", "channel", "create", items[0], "threads", "--title", "Q")
	require.NoError(t, err)
	got, err = runChannel(t, svc, "alice", "", "channel", "inbox", "--limit", "1")
	require.NoError(t, err)
	assert.Contains(t, got, "1 of 2 shown; next page: --cursor ")
	_, cursor, _ := strings.Cut(got, "--cursor ")
	cursor, _, _ = strings.Cut(cursor, "\n")
	next, err := runChannel(t, svc, "alice", "", "channel", "inbox", "--limit", "1", "--cursor", cursor)
	require.NoError(t, err)
	assert.Contains(t, next, " threads/")
	assert.Contains(t, next, "waiting for the starter (you)")
	assert.NotContains(t, next, "next page")
	assert.NotContains(t, next, "unread")

	_, err = runChannel(t, svc, "alice", "", "channel", "inbox", "--cursor", "forged")
	require.Error(t, err)
	assert.NotEqual(t, channelFailedText, err.Error())
	for _, bad := range []string{"0", "201"} {
		_, err = runChannel(t, svc, "alice", "", "channel", "inbox", "--limit", bad)
		require.ErrorContains(t, err, "limit must be between 1 and 200", bad)
	}
}

// TestChannelInbox_JSON pins AC 15 for JSON output: the service answer,
// notice first.
func TestChannelInbox_JSON(t *testing.T) {
	svc, items := newInboxProject(t)
	_, err := runChannel(t, svc, "alice", "", "channel", "post", items[0], "case", "chat", "-P", "body=hi")
	require.NoError(t, err)
	got, err := runChannel(t, svc, "alice", "", "-o", "json", "channel", "inbox")
	require.NoError(t, err)
	var view realtime.InboxView
	channelJSONOf(t, got, &view)
	require.Len(t, view.Entries, 1, got)
	e := view.Entries[0]
	assert.Equal(t, 1, view.Count)
	assert.Equal(t, items[0], e.Channel.Anchor)
	assert.Equal(t, "case", e.Channel.Type)
	assert.Equal(t, "owned by alice", e.Channel.AnchorTitle)
	assert.Equal(t, []string{"lead"}, e.Roles)
	assert.Equal(t, []string{"answer"}, e.Transitions)
	assert.NotContains(t, got, `"links"`)
}

// TestInbox_JobIdentity pins AC 16: a job identity sees a session whose
// string property holds its name, and a thread it started on a type with a
// creator waiting role, through the CLI and through the stdio MCP wiring.
func TestInbox_JobIdentity(t *testing.T) {
	svc, items := newInboxProject(t)
	const bot = "integration:triage-bot"
	_, err := runChannel(t, svc, "alice", "", "channel", "post", items[1], "case", "chat", "-P", "body=hi")
	require.NoError(t, err)
	_, err = runChannel(t, svc, bot, "", "channel", "create", items[0], "threads", "--title", "Triage")
	require.NoError(t, err)

	want := []string{items[1] + "/case", items[0] + "/threads"}
	got, err := runChannel(t, svc, bot, "", "-o", "json", "channel", "inbox")
	require.NoError(t, err)
	var view realtime.InboxView
	channelJSONOf(t, got, &view)
	assert.ElementsMatch(t, want, inboxKeys(view), got)

	deps := channelDeps(svc)
	require.NotNil(t, deps.Inbox)
	require.NotNil(t, deps.BindIdentity)
	ctx, err := deps.BindIdentity(principal.With(context.Background(),
		principal.Principal{User: bot, Tool: principal.ToolMCP}))
	require.NoError(t, err)
	stdio, err := deps.Inbox.Inbox(ctx, realtime.InboxRequest{})
	require.NoError(t, err)
	assert.ElementsMatch(t, want, inboxKeys(stdio))

	got, err = runChannel(t, svc, "alice", "", "-o", "json", "channel", "inbox")
	require.NoError(t, err)
	channelJSONOf(t, got, &view)
	assert.Empty(t, view.Entries, "alice holds neither role: %s", got)
}

// inboxKeys lists the entries of v as anchor/type.
func inboxKeys(v realtime.InboxView) []string {
	out := make([]string, 0, len(v.Entries))
	for _, e := range v.Entries {
		out = append(out, e.Channel.Anchor+"/"+e.Channel.Type)
	}
	return out
}

// TestChannelInbox_Unread pins TKT-2B2AIA AC 16 for the inbox: an
// "Unread channels" section after the waiting sessions, --kinds, and the
// refusal of an unknown kind.
func TestChannelInbox_Unread(t *testing.T) {
	svc, items := newInboxProject(t)
	const bot = "integration:triage-bot"
	_, err := runChannel(t, svc, "alice", "", "channel", "post", items[0], "case", "chat", "-P", "body=hi")
	require.NoError(t, err)
	_, err = runChannel(t, svc, bot, "", "channel", "post", items[0], "case", "chat", "-P", "body=one")
	require.NoError(t, err)
	_, err = runChannel(t, svc, bot, "", "channel", "post", items[0], "case", "chat", "-P", "body=two")
	require.NoError(t, err)

	got, err := runChannel(t, svc, "alice", "", "channel", "inbox")
	require.NoError(t, err)
	waiting, rest, ok := strings.Cut(got, "\n\n")
	require.True(t, ok, got)
	unread, mentions, ok := strings.Cut(rest, "\n\n")
	require.True(t, ok, got)
	assert.Equal(t, "No mentions or replies.\n", mentions)
	unread += "\n"
	assert.Contains(t, waiting, items[0]+" case Asked")
	assert.Equal(t, "Unread channels:\n"+items[0]+" case: 2 unread (read up to 1 of 3)\n", unread)

	got, err = runChannel(t, svc, "alice", "", "channel", "inbox", "--kinds", "unread")
	require.NoError(t, err)
	assert.Equal(t, unread, got)
	got, err = runChannel(t, svc, "alice", "", "channel", "inbox", "--kinds", "waiting")
	require.NoError(t, err)
	assert.Equal(t, waiting+"\n", got)

	_, err = runChannel(t, svc, "alice", "", "channel", "mark-read", items[0], "case")
	require.NoError(t, err)
	got, err = runChannel(t, svc, "alice", "", "channel", "inbox", "--kinds", "unread")
	require.NoError(t, err)
	assert.Equal(t, "No unread channels.\n", got)

	_, err = runChannel(t, svc, "alice", "", "channel", "inbox", "--kinds", "bogus")
	require.ErrorContains(t, err, "kinds must be waiting, unread, mention or reply")
}

// TestChannelInbox_Mentions pins TKT-6RXAC0 AC 18: the "Mentions and
// replies" section lists a mention and a reply, and mark-read clears both.
func TestChannelInbox_Mentions(t *testing.T) {
	svc, items := newInboxProject(t)
	const bot = "integration:triage-bot"
	_, err := runChannel(t, svc, "alice", "", "channel", "post", items[0], "case", "chat", "-P", "body=question")
	require.NoError(t, err)
	_, err = runChannel(t, svc, bot, "", "channel", "post", items[0], "case", "chat", "-P", "body=answer",
		"--reply-to", "1")
	require.NoError(t, err)
	_, err = runChannel(t, svc, bot, "", "channel", "post", items[0], "case", "note", "-P", "body=`@alice` look")
	require.NoError(t, err)

	got, err := runChannel(t, svc, "alice", "", "channel", "inbox", "--kinds", "mention,reply")
	require.NoError(t, err)
	assert.Regexp(t, "^Mentions and replies:\n"+
		items[0]+` case #3: integration:triage-bot mentioned you \(\S+\)\n`+
		items[0]+` case #2: integration:triage-bot replied to #1 \(\S+\)\n$`, got)

	got, err = runChannel(t, svc, "alice", "", "channel", "inbox", "--kinds", "reply")
	require.NoError(t, err)
	assert.NotContains(t, got, "mentioned you")
	assert.Contains(t, got, "replied to #1")

	got, err = runChannel(t, svc, "alice", "", "-o", "json", "channel", "inbox", "--kinds", "mention,reply")
	require.NoError(t, err)
	var view struct {
		MentionCount *int `json:"mention_count"`
		Mentions     []struct {
			Kind    string `json:"kind"`
			Seq     int64  `json:"seq"`
			ReplyTo int64  `json:"reply_to"`
		} `json:"mentions"`
	}
	require.NoError(t, json.Unmarshal([]byte(got), &view), got)
	require.NotNil(t, view.MentionCount)
	assert.Equal(t, 2, *view.MentionCount)
	require.Len(t, view.Mentions, 2)
	assert.Equal(t, "reply", view.Mentions[1].Kind)
	assert.Equal(t, int64(1), view.Mentions[1].ReplyTo)

	_, err = runChannel(t, svc, "alice", "", "channel", "mark-read", items[0], "case")
	require.NoError(t, err)
	got, err = runChannel(t, svc, "alice", "", "channel", "inbox", "--kinds", "mention,reply")
	require.NoError(t, err)
	assert.Equal(t, "No mentions or replies.\n", got)
}

// TestInbox_JobIdentityMention pins TKT-6RXAC0 AC 17: a mention of a job
// identity shows in its list_inbox mentions through the stdio MCP wiring,
// it reads the message after its marker, and mark_read clears it. A reply
// to its own message shows as a reply entry.
func TestInbox_JobIdentityMention(t *testing.T) {
	svc, items := newInboxProject(t)
	const bot = "integration:triage-bot"
	_, err := runChannel(t, svc, bot, "", "channel", "post", items[1], "case", "chat", "-P", "body=status?")
	require.NoError(t, err)
	_, err = runChannel(t, svc, "alice", "", "channel", "post", items[1], "case", "chat", "-P", "body=fine",
		"--reply-to", "1")
	require.NoError(t, err)
	_, err = runChannel(t, svc, "alice", "", "channel", "post", items[0], "case", "note",
		"-P", "body=`integration:triage-bot` please triage")
	require.NoError(t, err)

	deps := channelDeps(svc)
	ctx := stdioCtx(t, deps, bot)
	mentionOnly := realtime.InboxRequest{Kinds: []string{realtime.InboxKindMention}}
	view, err := deps.Inbox.Inbox(ctx, mentionOnly)
	require.NoError(t, err)
	require.Len(t, view.Mentions, 1)
	m := view.Mentions[0]
	assert.Equal(t, items[0], m.Channel.Anchor)
	assert.Equal(t, realtime.InboxKindMention, m.Kind)
	assert.Equal(t, "alice", m.Author.User)

	replies, err := deps.Inbox.Inbox(ctx, realtime.InboxRequest{Kinds: []string{realtime.InboxKindReply}})
	require.NoError(t, err)
	require.Len(t, replies.Mentions, 1)
	assert.Equal(t, realtime.InboxKindReply, replies.Mentions[0].Kind)
	assert.Equal(t, items[1], replies.Mentions[0].Channel.Anchor)
	assert.Equal(t, int64(1), replies.Mentions[0].ReplyTo)

	page, err := deps.Reader.Read(ctx, realtime.RawAddress{Anchor: items[0], Type: "case"},
		realtime.ReadRequest{AfterSeq: m.Seq - 1})
	require.NoError(t, err)
	require.NotEmpty(t, page.Messages)
	assert.Equal(t, m.Seq, page.Messages[0].Seq)

	_, err = deps.Marker.MarkRead(ctx, realtime.RawAddress{Anchor: items[0], Type: "case"}, nil)
	require.NoError(t, err)
	view, err = deps.Inbox.Inbox(ctx, mentionOnly)
	require.NoError(t, err)
	assert.Empty(t, view.Mentions)
	require.NotNil(t, view.MentionCount)
	assert.Zero(t, *view.MentionCount)
}

// TestInbox_JobIdentityUnread pins TKT-2B2AIA AC 17: a job identity that
// posts gets a marker; a later post by another principal shows in its
// list_inbox unread_channels through the stdio MCP wiring; mark_read
// clears it.
func TestInbox_JobIdentityUnread(t *testing.T) {
	svc, items := newInboxProject(t)
	const bot = "integration:triage-bot"
	_, err := runChannel(t, svc, bot, "", "channel", "post", items[1], "case", "chat", "-P", "body=mine")
	require.NoError(t, err)
	_, err = runChannel(t, svc, "alice", "", "channel", "post", items[1], "case", "chat", "-P", "body=news")
	require.NoError(t, err)

	deps := channelDeps(svc)
	require.NotNil(t, deps.Inbox)
	require.NotNil(t, deps.Marker)
	ctx := stdioCtx(t, deps, bot)
	unreadOnly := realtime.InboxRequest{Kinds: []string{realtime.InboxKindUnread}}
	view, err := deps.Inbox.Inbox(ctx, unreadOnly)
	require.NoError(t, err)
	require.Len(t, view.UnreadChannels, 1)
	u := view.UnreadChannels[0]
	assert.Equal(t, items[1], u.Channel.Anchor)
	assert.Equal(t, realtime.InboxUnread{Channel: u.Channel, Seq: 1, LastSeq: 2, Unread: 1}, u)

	got, err := deps.Marker.MarkRead(ctx, realtime.RawAddress{Anchor: items[1], Type: "case"}, nil)
	require.NoError(t, err)
	assert.Equal(t, realtime.MarkReadResult{Seq: 2, LastSeq: 2}, got)
	view, err = deps.Inbox.Inbox(ctx, unreadOnly)
	require.NoError(t, err)
	assert.Empty(t, view.UnreadChannels)
	require.NotNil(t, view.UnreadCount)
	assert.Zero(t, *view.UnreadCount)

	view, err = deps.Inbox.Inbox(stdioCtx(t, deps, "alice"), unreadOnly)
	require.NoError(t, err)
	assert.Empty(t, view.UnreadChannels, "alice's marker is her own")
}

// stdioCtx is user's MCP context, bound as the stdio wiring binds it.
func stdioCtx(t *testing.T, deps relamcp.ChannelDeps, user string) context.Context {
	t.Helper()
	ctx, err := deps.BindIdentity(principal.With(context.Background(),
		principal.Principal{User: user, Tool: principal.ToolMCP}))
	require.NoError(t, err)
	return ctx
}
