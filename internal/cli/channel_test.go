package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
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
          options: { type: string, list: true, max_items: 3 }
          picks:
            type: object
            list: true
            properties:
              label: { type: string, required: true }
              note:  { type: string }
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
    attach_to: [item, doc]
    roles:
      member: { from: attached }
    message_types:
      contained: { properties: { note: { type: string } }, post: [member] }
      closing:   { properties: { note: { type: string } }, post: [member], in: [contained] }
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
// system:ci read docs too and hold chat.lead. A doc is a hidden anchor for alice.
const channelCLIACL = `roles:
  editor:
    read: [item]
    create: [item]
    update: [item]
  ci:
    read: [item, doc]
    create: [item]
    update: [item]
    permissions: [chat.lead]
assignments:
  alice: editor
  "system:ci": ci
`

// channelProject is a project with realtime.yaml and acl.yaml, one item
// and one doc nobody may read.
type channelProject struct {
	root string
	svc  *appbuild.Services
	item string
	doc  string
}

func newChannelProject(t *testing.T) channelProject {
	t.Helper()
	return newChannelProjectWith(t, channelCLIRealtime)
}

// newChannelProjectWith is newChannelProject with its own realtime.yaml.
func newChannelProjectWith(t *testing.T, realtimeYAML string) channelProject {
	t.Helper()
	setBackendEnv(t)
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".rela"), 0o755))
	writeProjectFile(t, root, "metamodel.yaml", channelCLIMetamodel)
	writeProjectFile(t, root, "realtime.yaml", realtimeYAML)
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
	return channelProject{root: root, svc: openLeaseServices(t, root), item: ids[0], doc: ids[1]}
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
	assert.Regexp(t, `  text  1 reply, last \d{4}-\d\d-\d\dT[^\n]*Z\n`, got)
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

	got, err = runChannel(t, p.svc, "alice", "", "channel", "read", p.item, "room", "--thread", "1")
	require.NoError(t, err)
	assert.Contains(t, got, "#2 (reply to #1)")
	got, err = runChannel(t, p.svc, "alice", "", "channel", "read", p.item, "room", "--top")
	require.NoError(t, err)
	assert.Contains(t, got, "#1 ")
	assert.NotContains(t, got, "#2 ", "--top leaves out a reply that is not inline")

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
			{"channel", "mark-read", id, "room"},
			{"channel", "action", id, "room", "1", "x"},
			{"channel", "action", id, "room", "1", "--release"},
			{"channel", "session", id, "incident"},
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
		{"thread and top", "alice", []string{"channel", "read", p.item, "room", "--thread", "1", "--top"},
			"at most one of --thread and --top"},
		{"thread zero", "alice", []string{"channel", "read", p.item, "room", "--thread", "0"}, "--thread"},
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

// TestChannelCLI_SessionWrongState pins TKT-QYFLRH AC 4 for the CLI: a
// type limited by in: is not in "You may post" outside its states, and
// `rela channel post` prints the refusal with the allowed states and fails.
func TestChannelCLI_SessionWrongState(t *testing.T) {
	p := newChannelProject(t)
	got, err := runChannel(t, p.svc, "alice", "", "channel", "read", p.item, "incident")
	require.NoError(t, err)
	assert.Contains(t, got, "You may post: contained\n")

	_, err = runChannel(t, p.svc, "alice", "", "channel", "post", p.item, "incident", "closing")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "session_wrong_state: closing may be posted only while the session is in "+
		"contained; it is in detected (rule: channel_types.incident.message_types.closing.in)")

	_, err = runChannel(t, p.svc, "alice", "", "channel", "post", p.item, "incident", "contained")
	require.NoError(t, err)
	got, err = runChannel(t, p.svc, "alice", "", "channel", "read", p.item, "incident")
	require.NoError(t, err)
	assert.Contains(t, got, "You may post: closing, contained\n")
	_, err = runChannel(t, p.svc, "alice", "", "channel", "post", p.item, "incident", "closing")
	require.NoError(t, err)
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

// TestChannelCLI_ExpectGeneration pins TKT-T9G1AE AC 2 for the CLI:
// --expect-generation reaches the service when set, 0 included, and is
// not sent when absent; a negative value is refused before posting.
func TestChannelCLI_ExpectGeneration(t *testing.T) {
	p := newChannelProject(t)
	_, err := runChannel(t, p.svc, "alice", "", "channel", "post", p.item, "incident", "contained",
		"--expect-generation", "1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "session_moved: The session moved on: it is at generation 0 (detected); "+
		"this post expected generation 1.")

	_, err = runChannel(t, p.svc, "alice", "", "channel", "post", p.item, "room", "text", "-P", "body=x",
		"--expect-generation", "0")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid_message: expect_generation applies only")

	_, err = runChannel(t, p.svc, "alice", "", "channel", "post", p.item, "room", "text", "-P", "body=x")
	require.NoError(t, err, "absent, the flag is not sent")

	_, err = runChannel(t, p.svc, "alice", "", "channel", "post", p.item, "incident", "contained",
		"--expect-generation=-1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--expect-generation: a generation of at least 0")

	got, err := runChannel(t, p.svc, "alice", "", "channel", "post", p.item, "incident", "contained",
		"--expect-generation", "0")
	require.NoError(t, err)
	assert.Contains(t, got, "generation 1")
}

// TestChannelCLI_ListProp pins AC 3 and AC 7 of TKT-03FQPD for the CLI: a
// repeated -P adds an item to a list property, a comma stays inside one
// item, the read describes the property and prints the list one item per
// line, and a scalar given twice is still refused.
func TestChannelCLI_ListProp(t *testing.T) {
	p := newChannelProject(t)
	_, err := runChannel(t, p.svc, "alice", "", "channel", "post", p.item, "room", "text",
		"-P", "body=Which", "-P", "options=A", "-P", "options=b,c")
	require.NoError(t, err)

	got, err := runChannel(t, p.svc, "alice", "", "-o", "json", "channel", "read", p.item, "room")
	require.NoError(t, err)
	var read struct {
		Messages []struct {
			Props map[string]any `json:"props"`
		} `json:"messages"`
	}
	channelJSONOf(t, got, &read)
	require.Len(t, read.Messages, 1)
	assert.Equal(t, []any{"A", "b,c"}, read.Messages[0].Props["options"])

	got, err = runChannel(t, p.svc, "alice", "", "channel", "read", p.item, "room", "--schemas")
	require.NoError(t, err)
	assert.Contains(t, got, "  options: list of string, at most 3\n")
	assert.Contains(t, got, propIndent+"options:\n"+propIndent+"  - A\n"+propIndent+"  - b,c\n")

	_, err = runChannel(t, p.svc, "alice", "", "channel", "post", p.item, "room", "text",
		"-P", "body=a", "-P", "body=b")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "-P body: given twice")

	_, err = runChannel(t, p.svc, "alice", "", "channel", "post", p.item, "room", "text",
		"--props-json", `{"body":"x","options":["A","A"]}`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), `property "options": item 2 repeats an earlier item`)
}

// channelCLIFileRealtime adds a channel type with a file property.
const channelCLIFileRealtime = `version: 1
channel_types:
  shots:
    attach_to: [item]
    roles:
      member: { from: attached }
    message_types:
      photo:
        properties:
          caption: { type: string }
          image:   { type: file, max: 2 }
        post: [member]
`

func TestChannelCLI_Files(t *testing.T) {
	p := newChannelProjectWith(t, channelCLIFileRealtime)
	dir := t.TempDir()
	src := filepath.Join(dir, "note.txt")
	require.NoError(t, os.WriteFile(src, []byte("hello file"), 0o600))

	_, err := runChannel(t, p.svc, "alice", "", "channel", "post", p.item, "shots", "photo",
		"-P", "caption=hi", "--file", "image="+src)
	require.NoError(t, err)
	text, err := runChannel(t, p.svc, "alice", "", "channel", "read", p.item, "shots")
	require.NoError(t, err)
	assert.Contains(t, text, "  file image: note.txt (text/plain")
	schemas, err := runChannel(t, p.svc, "alice", "", "channel", "read", p.item, "shots", "--schemas")
	require.NoError(t, err)
	assert.Contains(t, schemas, "  image: file, at most 2 of 10485760 bytes\n")

	read, err := runChannel(t, p.svc, "alice", "", "-o", "json", "channel", "read", p.item, "shots")
	require.NoError(t, err)
	var view struct {
		Messages []struct {
			Seq   int64 `json:"seq"`
			Files []struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"files"`
		} `json:"messages"`
	}
	channelJSONOf(t, read, &view)
	require.Len(t, view.Messages, 1)
	require.Len(t, view.Messages[0].Files, 1)
	f := view.Messages[0].Files[0]

	dst := filepath.Join(dir, "out.txt")
	_, err = runChannel(t, p.svc, "alice", "", "channel", "file", p.item, "shots",
		strconv.FormatInt(view.Messages[0].Seq, 10), f.ID, "--to", dst)
	require.NoError(t, err)
	data, err := os.ReadFile(dst)
	require.NoError(t, err)
	assert.Equal(t, "hello file", string(data))

	_, err = runChannel(t, p.svc, "alice", "", "channel", "file", p.item, "shots",
		strconv.FormatInt(view.Messages[0].Seq, 10), "nope", "--to", dst)
	require.Error(t, err)

	for _, tc := range []struct{ name, flag, want string }{
		{"no equals", "image", "want PROP=PATH"},
		{"missing file", "image=" + filepath.Join(dir, "absent"), "--file image"},
		{"undeclared property", "caption=" + src, "no file property"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := runChannel(t, p.svc, "alice", "", "channel", "post", p.item, "shots", "photo", "--file", tc.flag)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.want)
		})
	}
}

// TestChannelCLI_ObjectListProp pins that each -P on an object list is one
// item given as a JSON object, that a non-object value is refused, and
// that the read prints each field of an item on its own line.
func TestChannelCLI_ObjectListProp(t *testing.T) {
	p := newChannelProject(t)
	_, err := runChannel(t, p.svc, "alice", "", "channel", "post", p.item, "room", "text",
		"-P", "body=Which", "-P", `picks={"label":"A","note":"fast"}`, "-P", `picks={"label":"B"}`)
	require.NoError(t, err)

	got, err := runChannel(t, p.svc, "alice", "", "-o", "json", "channel", "read", p.item, "room")
	require.NoError(t, err)
	var read struct {
		Messages []struct {
			Props map[string]any `json:"props"`
		} `json:"messages"`
	}
	channelJSONOf(t, got, &read)
	require.Len(t, read.Messages, 1)
	assert.Equal(t, []any{
		map[string]any{"label": "A", "note": "fast"},
		map[string]any{"label": "B"},
	}, read.Messages[0].Props["picks"])

	got, err = runChannel(t, p.svc, "alice", "", "channel", "read", p.item, "room", "--schemas")
	require.NoError(t, err)
	assert.Contains(t, got, "  picks: list of object with label, note")
	assert.Contains(t, got, propIndent+"picks:\n"+propIndent+"  - label: A\n"+propIndent+"    note: fast\n"+propIndent+"  - label: B\n")

	_, err = runChannel(t, p.svc, "alice", "", "channel", "post", p.item, "room", "text",
		"-P", "body=x", "-P", "picks=A")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "want one item as a JSON object")
}

// TestChannelCLI_PostBatch pins post-batch: one line per message, the
// shared batch id in JSON, the array read from stdin, and a malformed
// document refused before posting.
func TestChannelCLI_PostBatch(t *testing.T) {
	p := newChannelProject(t)
	batch := `[{"type":"text","props":{"body":"a"}},{"type":"text","props":{"body":"b"}}]`
	got, err := runChannel(t, p.svc, "alice", "", "channel", "post-batch", p.item, "room", "--json", batch)
	require.NoError(t, err)
	assert.Equal(t, "#1 posted\n#2 posted\n", got)

	got, err = runChannel(t, p.svc, "alice", batch, "-o", "json", "channel", "post-batch", p.item, "room",
		"--json", "-")
	require.NoError(t, err)
	var posted batchJSON
	channelJSONOf(t, got, &posted)
	require.Len(t, posted.Messages, 2)
	assert.Equal(t, []int64{3, 3}, []int64{posted.Messages[0].Batch, posted.Messages[1].Batch})

	_, err = runChannel(t, p.svc, "alice", "", "channel", "post-batch", p.item, "room",
		"--json", `[{"type":"text","body":"a"}]`)
	require.ErrorContains(t, err, `--json: an array of {"type": ..., "props": {...}}`)
}
