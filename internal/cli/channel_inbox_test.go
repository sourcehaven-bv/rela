package cli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/entity"
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
// the empty answer, and paging.
func TestChannelInbox_Text(t *testing.T) {
	svc, items := newInboxProject(t)

	got, err := runChannel(t, svc, "alice", "", "channel", "inbox")
	require.NoError(t, err)
	assert.Equal(t, "No session waits on you.\n", got)

	for _, id := range items {
		_, err = runChannel(t, svc, "alice", "", "channel", "post", id, "case", "chat", "-P", "body=hi")
		require.NoError(t, err)
	}
	got, err = runChannel(t, svc, "alice", "", "channel", "inbox")
	require.NoError(t, err)
	lines := strings.Split(strings.TrimSuffix(got, "\n"), "\n")
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
	cursor := got[strings.LastIndex(got, " ")+1 : len(got)-1]
	next, err := runChannel(t, svc, "alice", "", "channel", "inbox", "--limit", "1", "--cursor", cursor)
	require.NoError(t, err)
	assert.Contains(t, next, " threads/")
	assert.Contains(t, next, "waiting for the starter (you)")
	assert.NotContains(t, next, "next page")

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
