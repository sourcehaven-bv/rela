package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alecthomas/kong"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/appbuild/backendtest"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/output"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/realtime"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

const channelCLIMetamodel = `version: "1.0"
entities:
  item:
    label: Item
    id_type: sequential
    id_prefix: ITEM-
    properties:
      title: {type: string}
  doc:
    label: Doc
    id_type: sequential
    id_prefix: DOC-
    properties:
      title: {type: string}
relations: {}
`

const channelCLIRealtime = `version: 1
channel_types:
  room:
    attach_to: [item, doc]
    roles:
      member: { from: attached }
    message_types:
      text:
        properties:
          body:   { type: string, required: true }
          count:  { type: integer }
          urgent: { type: boolean }
          about:  { type: ref, to: [item, doc] }
        post: [member]
      typing: { persist: ephemeral, ttl: 5s, post: [member] }
    message_ops:
      edit_own:   { roles: [member], within: 15m }
      delete_own: { roles: [member] }
  threads:
    attach_to: [item]
    cardinality: many
    create: { roles: [member] }
    roles:
      member: { from: attached }
    message_types:
      text:
        properties: { body: { type: string } }
        post: [member]
  incident:
    attach_to: [item]
    roles:
      member: { from: attached }
    message_types:
      contained: { properties: { note: { type: string } }, post: [member] }
    session:
      initial: detected
      states:
        detected:
          label: Detected
          on:
            contained:
              to: contained
              outputs:
                - set: { properties: { title: contained } }
                - create: { type: item, properties: { title: Follow-up } }
        contained: { label: Contained, final: true }
`

// channelCLIACL lets alice read and write items, and the job identity
// system:ci read docs too. A doc is a hidden anchor for alice.
const channelCLIACL = `roles:
  editor:
    read: [item]
    create: [item]
    update: [item]
  ci:
    read: [item, doc]
    create: [item]
    update: [item]
assignments:
  alice: editor
  "system:ci": ci
`

// channelProject is a project with realtime.yaml and acl.yaml, one item
// and one doc nobody may read.
type channelProject struct {
	svc  *appbuild.Services
	item string
	doc  string
}

func newChannelProject(t *testing.T) channelProject {
	t.Helper()
	setBackendEnv(t)
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".rela"), 0o755))
	writeProjectFile(t, root, "metamodel.yaml", channelCLIMetamodel)
	writeProjectFile(t, root, "realtime.yaml", channelCLIRealtime)
	// Seed before acl.yaml exists, so the seed needs no grant.
	seed := openLeaseServices(t, root)
	var ids []string
	for _, typ := range []string{"item", "doc"} {
		e := entity.New("", typ)
		e.SetString("title", "a "+typ)
		created, err := seed.EntityManager().CreateEntity(leaseCtx(), e, entity.CreateOptions{})
		require.NoError(t, err)
		ids = append(ids, created.Entity.ID)
	}
	require.NoError(t, seed.Close())
	writeProjectFile(t, root, "acl.yaml", channelCLIACL)
	return channelProject{svc: openLeaseServices(t, root), item: ids[0], doc: ids[1]}
}

// runChannel runs a channel command as user, over the bundles of svc and
// through the CLI's request edge, as runKong does.
//
// It swaps the package globals out and channelStdin, so a test that uses
// it must not call t.Parallel.
func runChannel(t *testing.T, svc *appbuild.Services, user, stdin string, args ...string) (string, error) {
	t.Helper()
	var cli CLI
	parser, err := kong.New(&cli, kong.Name("rela"), kong.Vars{"version": Version})
	require.NoError(t, err)
	ktx, err := parser.Parse(args)
	if err != nil {
		return "", err
	}
	b, err := newCLIBundles(svc)
	require.NoError(t, err)
	bind, err := appbuild.IdentityBinder(svc)
	require.NoError(t, err)
	ctx := appbuild.BindOrWarn(principal.With(context.Background(),
		principal.Principal{User: user, Tool: principal.ToolCLI}), bind)

	buf := new(bytes.Buffer)
	prevOut, prevStdin := out, channelStdin
	out = output.NewWithWriter(buf, output.Format(cli.Output))
	channelStdin = strings.NewReader(stdin)
	t.Cleanup(func() { out, channelStdin = prevOut, prevStdin })
	ktx.BindTo(ctx, (*context.Context)(nil))
	ktx.Bind(out, b.channels)
	err = ktx.Run()
	return buf.String(), err
}

// channelJSONOf decodes a JSON document and checks notice is its first key.
func channelJSONOf(t *testing.T, got string, v any) {
	t.Helper()
	require.True(t, strings.HasPrefix(strings.TrimSpace(got), `{
  "notice": `), "notice leads: %s", got)
	require.NoError(t, json.Unmarshal([]byte(got), v), got)
}

func TestChannelRequiresProject(t *testing.T) {
	assert.True(t, requiresProject("channel list <id>"))
}

// messagesJSON is the part of a read document the tests decode.
type messagesJSON struct {
	Messages []realtime.MessageView `json:"messages"`
}

// TestChannelCLI_Loop runs every command in table and JSON output.
func TestChannelCLI_Loop(t *testing.T) {
	p := newChannelProject(t)

	got, err := runChannel(t, p.svc, "alice", "", "channel", "post", p.item, "room", "text",
		"-P", "body=hello, world", "-P", "count=3", "-P", "urgent=true", "-P", "about="+p.item)
	require.NoError(t, err)
	assert.Equal(t, "#1 posted\n", got)

	got, err = runChannel(t, p.svc, "alice", `{"body":"from\nstdin"}`,
		"-o", "json", "channel", "post", p.item, "room", "text", "--props-json", "-", "--reply-to", "1")
	require.NoError(t, err)
	var posted messageJSON
	channelJSONOf(t, got, &posted)
	assert.Equal(t, int64(2), posted.Message.Seq)
	assert.Equal(t, int64(1), posted.Message.ReplyTo)
	assert.Equal(t, "from\nstdin", posted.Message.Props["body"])

	got, err = runChannel(t, p.svc, "alice", "", "-o", "json", "channel", "read", p.item, "room")
	require.NoError(t, err)
	var read struct {
		Notice string `json:"notice"`
		realtime.ReadView
	}
	channelJSONOf(t, got, &read)
	require.NotEmpty(t, read.Messages)
	first := read.Messages[0]
	assert.Equal(t, "hello, world", first.Props["body"], "-P keeps commas")
	assert.InDelta(t, 3, first.Props["count"], 0, "-P coerces an integer property")
	assert.Equal(t, true, first.Props["urgent"], "-P coerces a boolean property")
	assert.Equal(t, "alice", first.Author.User)
	require.NotNil(t, read.Actions)
	assert.Equal(t, []string{"text"}, read.Actions.Post, "the CLI cannot post a signal")

	got, err = runChannel(t, p.svc, "alice", "", "channel", "read", p.item, "room", "--schemas")
	require.NoError(t, err)
	assert.Contains(t, got, "You may post: text\n")
	assert.Contains(t, got, "  count: integer\n")
	assert.Contains(t, got, "#1  ")
	assert.Contains(t, got, "  alice (cli)  text\n")
	assert.Contains(t, got, propIndent+"body: hello, world\n")
	assert.Contains(t, got, propIndent+"about: "+p.item+"\n")
	assert.Contains(t, got, "  text  1 reply\n")
	assert.Contains(t, got, propIndent+"body: from\n"+propIndent+"stdin\n")

	got, err = runChannel(t, p.svc, "alice", "", "channel", "edit", p.item, "room", "1",
		"-P", "body=changed\nline2", "-P", "count=4")
	require.NoError(t, err)
	assert.Equal(t, "#1 edited\n", got)

	got, err = runChannel(t, p.svc, "alice", "", "-o", "json", "channel", "remove", p.item, "room", "2")
	require.NoError(t, err)
	var removed messageJSON
	channelJSONOf(t, got, &removed)
	require.NotNil(t, removed.Message.Removed)

	got, err = runChannel(t, p.svc, "alice", "", "channel", "read", p.item, "room")
	require.NoError(t, err)
	assert.Contains(t, got, propIndent+"body: changed\n"+propIndent+"line2\n", "-P keeps a newline")
	assert.Contains(t, got, propIndent+"count: 4\n")
	assert.Contains(t, got, "#2 (reply to #1)  [removed by author]\n")

	got, err = runChannel(t, p.svc, "alice", "", "-o", "json", "channel", "read", p.item, "room")
	require.NoError(t, err)
	var reread messagesJSON
	channelJSONOf(t, got, &reread)
	require.NotEmpty(t, reread.Messages)
	assert.Equal(t, "changed\nline2", reread.Messages[0].Props["body"])
	assert.InDelta(t, 4, reread.Messages[0].Props["count"], 0, "edit coerces -P from one read")

	got, err = runChannel(t, p.svc, "alice", "", "channel", "create", p.item, "threads", "--title", "Design")
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(got, "created "+p.item+" threads "), got)
	assert.Contains(t, got, `"Design"`)

	got, err = runChannel(t, p.svc, "alice", "", "channel", "list", p.item)
	require.NoError(t, err)
	assert.Contains(t, got, "TYPE")
	assert.Contains(t, got, "Design")
	assert.Contains(t, got, "You may create: threads\n")

	got, err = runChannel(t, p.svc, "alice", "", "-o", "json", "channel", "list", p.item, "--state", "open")
	require.NoError(t, err)
	var list struct {
		Notice string `json:"notice"`
		realtime.AnchorChannels
	}
	channelJSONOf(t, got, &list)
	assert.Equal(t, p.item, list.Anchor)
}

// TestChannelCLI_HiddenAnchorTextEqualsShow pins that a hidden and a missing
// anchor answer exactly what `rela show` answers for the same id.
func TestChannelCLI_HiddenAnchorTextEqualsShow(t *testing.T) {
	p := newChannelProject(t)
	for _, id := range []string{p.doc, "ITEM-999"} {
		want := classifyReadError(id, store.ErrNotFound).Error()
		for _, args := range [][]string{
			{"channel", "list", id},
			{"channel", "read", id, "room"},
			{"channel", "post", id, "room", "text", "-P", "body=x"},
			{"channel", "edit", id, "room", "1", "-P", "body=x"},
			{"channel", "remove", id, "room", "1"},
			{"channel", "create", id, "threads"},
			{"channel", "watch", id, "room", "--timeout", "1s"},
		} {
			_, err := runChannel(t, p.svc, "alice", "", args...)
			require.Error(t, err, args)
			assert.Equal(t, want, err.Error(), args)
		}
	}
}

// TestChannelCLI_Errors pins the refusals the CLI answers itself and the
// service texts it passes on.
func TestChannelCLI_Errors(t *testing.T) {
	p := newChannelProject(t)
	tests := []struct {
		name string
		user string
		args []string
		want string
	}{
		{"signal", "alice", []string{"channel", "post", p.item, "room", "typing"}, "signals_not_kept"},
		{"unknown type", "alice", []string{"channel", "read", p.item, "nosuch"}, "unknown_channel_type"},
		{"bad integer", "alice", []string{"channel", "post", p.item, "room", "text", "-P", "body=x", "-P", "count=many"},
			"-P count: want a whole number"},
		{"bad boolean", "alice", []string{"channel", "post", p.item, "room", "text", "-P", "body=x", "-P", "urgent=maybe"},
			"-P urgent: want true or false"},
		{"no equals", "alice", []string{"channel", "post", p.item, "room", "text", "-P", "body"}, "want key=value"},
		{"twice", "alice", []string{"channel", "post", p.item, "room", "text", "-P", "body=a", "-P", "body=b"},
			"given twice"},
		{"clash", "alice", []string{"channel", "post", p.item, "room", "text", "-P", "body=a", "--props-json", "{}"},
			"use -P or --props-json, not both"},
		{"not an object", "alice", []string{"channel", "post", p.item, "room", "text", "--props-json", "[1]"},
			"want a JSON object"},
		{"edit without props", "alice", []string{"channel", "edit", p.item, "room", "1"}, "give -P or --props-json"},
		{"two cursors", "alice", []string{"channel", "read", p.item, "room", "--after", "1", "--before", "3"},
			"at most one of"},
		{"limit", "alice", []string{"channel", "read", p.item, "room", "--limit", "500"}, "--limit: 1 to 200"},
		{"no principal", principal.Unknown, []string{"channel", "post", p.item, "room", "text", "-P", "body=x"},
			realtime.NoPrincipalHint},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := runChannel(t, p.svc, tc.user, "", tc.args...)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.want)
		})
	}
}

// TestChannelCLI_RestrictedRef pins that a ref the reader cannot read shows
// no id or title, and that posting a hidden and a missing ref answer alike.
func TestChannelCLI_RestrictedRef(t *testing.T) {
	p := newChannelProject(t)
	_, errHidden := runChannel(t, p.svc, "alice", "", "channel", "post", p.item, "room", "text",
		"-P", "body=x", "-P", "about="+p.doc)
	_, errMissing := runChannel(t, p.svc, "alice", "", "channel", "post", p.item, "room", "text",
		"-P", "body=x", "-P", "about=DOC-999")
	require.Error(t, errHidden)
	require.Error(t, errMissing)
	assert.Equal(t, errMissing.Error(), strings.ReplaceAll(errHidden.Error(), p.doc, "DOC-999"))

	// The job identity may read docs and posts the ref; alice cannot see it.
	_, err := runChannel(t, p.svc, "system:ci", "", "channel", "post", p.item, "room", "text",
		"-P", "body=see", "-P", "about="+p.doc)
	require.NoError(t, err)
	got, err := runChannel(t, p.svc, "alice", "", "channel", "read", p.item, "room")
	require.NoError(t, err)
	assert.Contains(t, got, propIndent+"about: (restricted)\n")
	assert.NotContains(t, got, p.doc)

	got, err = runChannel(t, p.svc, "alice", "", "-o", "json", "channel", "read", p.item, "room")
	require.NoError(t, err)
	assert.Contains(t, got, `"restricted": true`)
	assert.NotContains(t, got, p.doc)
	assert.NotContains(t, got, "a doc")
}

// TestChannelCLI_Disabled pins the answer of a project without
// realtime.yaml.
func TestChannelCLI_Disabled(t *testing.T) {
	setBackendEnv(t)
	root, _ := newLeaseProject(t)
	svc := openLeaseServices(t, root)
	_, err := runChannel(t, svc, "ops", "", "channel", "list", "DOC-1")
	require.Error(t, err)
	assert.Equal(t, realtime.OffText, err.Error())
}

// appboundChannels returns the project's channel service and a ctx bound
// for alice through the CLI's request edge, for tests that drive a
// command's loop directly.
func appboundChannels(t *testing.T, p channelProject) (context.Context, channelService) {
	t.Helper()
	bind, err := appbuild.IdentityBinder(p.svc)
	require.NoError(t, err)
	ctx := appbuild.BindOrWarn(principal.With(context.Background(),
		principal.Principal{User: "alice", Tool: principal.ToolCLI}), bind)
	svc := appbuild.ChannelService(p.svc)
	require.NotNil(t, svc)
	return ctx, svc
}

// swapOut points the package output at a buffer in format until the test
// ends.
func swapOut(t *testing.T, format string) *bytes.Buffer {
	t.Helper()
	buf := new(bytes.Buffer)
	prev := out
	out = output.NewWithWriter(buf, output.Format(format))
	t.Cleanup(func() { out = prev })
	return buf
}

// setBackendEnv points appbuild.Discover at a store the current build can
// open (see mcpServicesForTest); on fs and memory it sets nothing.
func setBackendEnv(t *testing.T) {
	t.Helper()
	for k, v := range backendtest.Env(t) {
		t.Setenv(k, v)
	}
}

// TestChannelCLI_PostSession pins that a post in a session channel prints
// the session after the message, and the outputs of its move, in table and
// JSON output.
func TestChannelCLI_PostSession(t *testing.T) {
	p := newChannelProject(t)
	got, err := runChannel(t, p.svc, "alice", "", "channel", "post", p.item, "incident", "contained")
	require.NoError(t, err)
	assert.Equal(t, "#1 posted\nsession: contained (Contained), generation 1, final, completed\n"+
		"outputs done: set "+p.item+", created ITEM-002\n", got)

	got, err = runChannel(t, p.svc, "alice", "", "channel", "read", p.item, "incident")
	require.NoError(t, err)
	assert.Contains(t, got, "session: contained (Contained), generation 1, final, completed\n")
	assert.Contains(t, got, "  moved from detected to contained; outputs done: set "+p.item+
		", created ITEM-002\n")

	got, err = runChannel(t, p.svc, "alice", "", "-o", "json", "channel", "post", p.item, "incident", "contained")
	require.NoError(t, err)
	var posted messageJSON
	channelJSONOf(t, got, &posted)
	require.NotNil(t, posted.Session)
	assert.Equal(t, "contained", posted.Session.State)
	assert.Nil(t, posted.Outputs, "a post in a final state moves nothing")

	got, err = runChannel(t, p.svc, "alice", "", "channel", "post", p.item, "room", "text", "-P", "body=x")
	require.NoError(t, err)
	assert.Equal(t, "#1 posted\n", got, "a type without a session prints no session")
}
