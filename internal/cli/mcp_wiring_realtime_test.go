package cli

import (
	"bytes"
	"context"
	"log/slog"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	mcpgo "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	relamcp "github.com/Sourcehaven-BV/rela/internal/mcp"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/realtime"
	"github.com/Sourcehaven-BV/rela/internal/testutil/logtest"
)

const stdioRealtimeYAML = `version: 1
channel_types:
  room:
    attach_to: [item]
    roles:
      member: { from: attached }
    message_types:
      text:
        properties: { body: { type: string } }
        post: [member]
      typing: { persist: ephemeral, ttl: 5s, post: [member] }
`

const stdioRealtimeWithNotes = stdioRealtimeYAML + `  notes:
    attach_to: [item]
    roles:
      member: { from: attached }
    message_types:
      text:
        properties: { body: { type: string } }
        post: [member]
`

// writeProjectFile writes name under root.
func writeProjectFile(t *testing.T, root, name, body string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(root, name), []byte(body), 0o644))
}

// hasChannelType reports whether the current realtime config of svc
// declares typ.
func hasChannelType(svc *mcpServices, typ string) bool {
	cfg := appbuild.Realtime(svc.current()).Load().Config
	if cfg == nil {
		return false
	}
	_, ok := cfg.ChannelType(typ)
	return ok
}

// TestMCPStdio_RealtimeReload pins AC 16: stdio wires the channel tools with
// signals refused, an edit to realtime.yaml applies from the next call, a
// broken edit keeps the previous config, and nothing reloads after Close.
func TestMCPStdio_RealtimeReload(t *testing.T) {
	root := seedProject(t)
	writeProjectFile(t, root, "realtime.yaml", stdioRealtimeYAML)
	svc, err := mcpServicesForTest(t, root)
	require.NoError(t, err)
	t.Cleanup(func() { _ = svc.Close() })

	deps := svc.Deps()
	require.NotNil(t, deps.Channels.Reader, "stdio wires the channel tools")
	assert.True(t, deps.Channels.RefuseSignals, "stdio refuses signals")
	assert.Equal(t, stdioChannelPoll, deps.Watch.ChannelPoll)
	assert.Same(t, appbuild.ChannelService(svc.current()), deps.Channels.Reader,
		"without acl.yaml the channel tools use the open service")

	srv, err := relamcp.NewServer(deps, "test",
		relamcp.WithPrincipal(principal.Principal{User: "t", Tool: principal.ToolMCP}))
	require.NoError(t, err)
	svc.watchSchema(srv)
	require.NotNil(t, svc.stopRealtimeWatch, "realtime.yaml watcher should have started")

	writeProjectFile(t, root, "realtime.yaml", stdioRealtimeWithNotes)
	assert.Eventually(t, func() bool { return hasChannelType(svc, "notes") },
		5*time.Second, 50*time.Millisecond, "the realtime.yaml edit never applied")

	// The watcher path is pinned above; the broken edit is reloaded directly,
	// so the assertion does not race the watcher.
	writeProjectFile(t, root, "realtime.yaml", "version: 1\nchannel_types: [broken")
	svc.reloadRealtime()
	assert.True(t, hasChannelType(svc, "notes"), "a broken edit replaced the previous config")

	require.NoError(t, svc.Close())
	writeProjectFile(t, root, "realtime.yaml", stdioRealtimeYAML)
	svc.reloadRealtime()
	assert.True(t, hasChannelType(svc, "notes"), "a reload ran after Close")
}

// TestMCPStdio_ChannelToolsApplyACL pins that stdio channel tools apply
// acl.yaml while its entity tools stay open.
func TestMCPStdio_ChannelToolsApplyACL(t *testing.T) {
	root := seedProject(t)
	writeProjectFile(t, root, "realtime.yaml", stdioRealtimeYAML)
	writeProjectFile(t, root, "acl.yaml", `roles:
  editor:
    read: [item]
    create: [item]
    update: [item]
assignments:
  alice: editor
`)
	svc, err := mcpServicesForTest(t, root)
	require.NoError(t, err)
	t.Cleanup(func() { _ = svc.Close() })

	deps := svc.Deps()
	require.NotNil(t, deps.Channels.Reader)
	assert.NotSame(t, appbuild.ChannelService(svc.current()), deps.Channels.Reader,
		"with acl.yaml the channel tools use a gated service")

	as := func(user string) context.Context {
		return principal.With(context.Background(), principal.Principal{User: user, Tool: principal.ToolMCP})
	}
	// asChannels is as, bound for the channel service the way the channel
	// tools bind each call.
	asChannels := func(user string) context.Context {
		ctx, bindErr := deps.Channels.BindIdentity(as(user))
		require.NoError(t, bindErr)
		return ctx
	}
	created, err := deps.EntityManager.CreateEntity(as("alice"),
		&entity.Entity{Type: "item", Properties: map[string]any{"title": "one"}}, entity.CreateOptions{})
	require.NoError(t, err, "entity writes stay open on stdio")

	addr := realtime.RawAddress{Anchor: created.Entity.ID, Type: "room"}
	_, err = deps.Channels.Reader.Read(as("alice"), addr, realtime.ReadRequest{})
	assert.True(t, realtime.IsNoPrincipal(err), "a call not bound for the channel service is refused: %v", err)
	_, err = deps.Channels.Reader.Read(asChannels("alice"), addr, realtime.ReadRequest{})
	require.NoError(t, err, "alice reads items")
	_, err = deps.Channels.Reader.Read(asChannels("mallory"), addr, realtime.ReadRequest{})
	assert.ErrorIs(t, err, realtime.ErrAnchorNotFound, "mallory reads no item under acl.yaml")
}

const (
	stdioACLAlice = `roles:
  editor:
    read: [item]
    create: [item]
    update: [item]
assignments:
  alice: editor
`
	stdioACLBob    = stdioACLAlice + "  bob: editor\n"
	stdioACLBroken = "roles: [broken\n"
	// stdioACLUnknownType parses, but names a user type the schema lacks.
	stdioACLUnknownType = "user_entity_type: nosuch\n" + stdioACLAlice
	aclUnloadable       = "acl.yaml cannot be loaded"
)

// stdioServer builds the stdio MCP server over svc for user and returns a
// client session and the server. The session runs over HTTP because the
// server's in-memory transport is internal to package mcp.
func stdioServer(t *testing.T, svc *mcpServices, user string) (*mcpgo.ClientSession, *relamcp.Server) {
	t.Helper()
	srv, err := relamcp.NewServer(svc.Deps(), "test",
		relamcp.WithPrincipal(principal.Principal{User: user, Tool: principal.ToolMCP}))
	require.NoError(t, err)
	ts := httptest.NewServer(srv.HTTPHandler())
	t.Cleanup(ts.Close)
	client := mcpgo.NewClient(&mcpgo.Implementation{Name: "test", Version: "0"}, nil)
	cs, err := client.Connect(context.Background(), &mcpgo.StreamableClientTransport{
		Endpoint: ts.URL, DisableStandaloneSSE: true,
	}, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = cs.Close() })
	return cs, srv
}

// callText calls tool and returns its text and whether it is an error.
func callText(t *testing.T, cs *mcpgo.ClientSession, tool string, args map[string]any) (string, bool) {
	t.Helper()
	res, err := cs.CallTool(context.Background(), &mcpgo.CallToolParams{Name: tool, Arguments: args})
	require.NoError(t, err)
	var b strings.Builder
	for _, c := range res.Content {
		if tc, ok := c.(*mcpgo.TextContent); ok {
			b.WriteString(tc.Text)
		}
	}
	return b.String(), res.IsError
}

// seedItem creates an item through the stdio entity path and returns its id.
func seedItem(t *testing.T, svc *mcpServices) string {
	t.Helper()
	created, err := svc.Deps().EntityManager.CreateEntity(context.Background(),
		&entity.Entity{Type: "item", Properties: map[string]any{"title": "one"}}, entity.CreateOptions{})
	require.NoError(t, err)
	return created.Entity.ID
}

// TestMCPStdio_BrokenACLAtStartup pins the fail-closed path at startup. An
// acl.yaml that does not parse stops rela mcp when realtime.yaml exists,
// because the realtime config needs the policy for its checks. One that
// parses but does not apply to the schema makes every channel tool answer
// why, and the entity tools keep working.
func TestMCPStdio_BrokenACLAtStartup(t *testing.T) {
	root := seedProject(t)
	writeProjectFile(t, root, "realtime.yaml", stdioRealtimeYAML)
	writeProjectFile(t, root, "acl.yaml", stdioACLBroken)
	_, startErr := mcpServicesForTest(t, root)
	require.ErrorContains(t, startErr, "acl.yaml", "an unparseable acl.yaml must stop startup")

	writeProjectFile(t, root, "acl.yaml", stdioACLUnknownType)
	svc, err := mcpServicesForTest(t, root)
	require.NoError(t, err)
	t.Cleanup(func() { _ = svc.Close() })
	id := seedItem(t, svc)

	cs, _ := stdioServer(t, svc, "alice")
	text, isErr := callText(t, cs, "read_channel", map[string]any{"id": id, "channel_type": "room"})
	assert.True(t, isErr, "read_channel succeeded under a broken acl.yaml: %s", text)
	assert.Contains(t, text, aclUnloadable)
	text, isErr = callText(t, cs, "post_message", map[string]any{
		"id": id, "channel_type": "room", "message_type": "text", "props": map[string]any{"body": "hi"},
	})
	assert.True(t, isErr, "post_message succeeded under a broken acl.yaml: %s", text)
	assert.Contains(t, text, aclUnloadable)

	text, isErr = callText(t, cs, "show_entity", map[string]any{"id": id})
	assert.False(t, isErr, "show_entity failed: %s", text)
}

// TestMCPStdio_BrokenACLOnReload pins that a schema reload with a broken
// acl.yaml publishes the new schema, retires nothing that still serves, and
// leaves the channel tools refusing.
func TestMCPStdio_BrokenACLOnReload(t *testing.T) {
	root := seedProject(t)
	writeProjectFile(t, root, "realtime.yaml", stdioRealtimeYAML)
	writeProjectFile(t, root, "acl.yaml", stdioACLAlice)
	svc, err := mcpServicesForTest(t, root)
	require.NoError(t, err)
	t.Cleanup(func() { _ = svc.Close() })
	id := seedItem(t, svc)
	cs, srv := stdioServer(t, svc, "alice")
	_, isErr := callText(t, cs, "read_channel", map[string]any{"id": id, "channel_type": "room"})
	require.False(t, isErr, "alice reads the channel before the edit")

	writeProjectFile(t, root, "acl.yaml", stdioACLBroken)
	writeSchema(t, root, schemaWithRisk)
	require.NoError(t, svc.reload(srv.ReloadDeps), "a broken acl.yaml must not block a schema reload")
	require.NotSame(t, svc.origin, svc.current(), "the reload did not publish a successor")

	text, isErr := callText(t, cs, "create_entity", map[string]any{
		"type": "risk", "properties": map[string]any{"title": "outage"},
	})
	assert.False(t, isErr, "the server does not serve the new schema: %s", text)
	text, isErr = callText(t, cs, "read_channel", map[string]any{"id": id, "channel_type": "room"})
	assert.True(t, isErr, "read_channel succeeded under a broken acl.yaml: %s", text)
	assert.Contains(t, text, aclUnloadable)
}

// TestMCPStdio_ACLWatch pins that the channel tools follow an acl.yaml edit
// without a restart, in both directions.
func TestMCPStdio_ACLWatch(t *testing.T) {
	root := seedProject(t)
	writeProjectFile(t, root, "realtime.yaml", stdioRealtimeYAML)
	writeProjectFile(t, root, "acl.yaml", stdioACLAlice)
	svc, err := mcpServicesForTest(t, root)
	require.NoError(t, err)
	t.Cleanup(func() { _ = svc.Close() })
	id := seedItem(t, svc)
	cs, srv := stdioServer(t, svc, "bob")
	svc.watchSchema(srv)
	require.NotNil(t, svc.stopACLWatch, "acl.yaml watcher should have started")

	read := map[string]any{"id": id, "channel_type": "room"}
	_, isErr := callText(t, cs, "read_channel", read)
	require.True(t, isErr, "bob has no role and must not read the channel")

	writeProjectFile(t, root, "acl.yaml", stdioACLBob)
	assert.Eventually(t, func() bool {
		_, isErr := callText(t, cs, "read_channel", read)
		return !isErr
	}, 5*time.Second, 50*time.Millisecond, "the acl.yaml edit granting bob never applied")

	writeProjectFile(t, root, "acl.yaml", stdioACLBroken)
	assert.Eventually(t, func() bool {
		text, isErr := callText(t, cs, "read_channel", read)
		return isErr && strings.Contains(text, aclUnloadable)
	}, 5*time.Second, 50*time.Millisecond, "a broken acl.yaml edit did not close the channel tools")
}

// TestMCPStdio_WarnsUnassignedUser pins the startup warning for a server
// principal acl.yaml names no role for: $USER, or the --run-as identity.
func TestMCPStdio_WarnsUnassignedUser(t *testing.T) {
	for _, tc := range []struct {
		name, acl, user string
		want            bool
	}{
		{"unassigned", stdioACLAlice, "bob", true},
		{"assigned", stdioACLAlice, "alice", false},
		{"no acl.yaml", "", "bob", false},
		{"run-as unassigned", stdioACLAlice, "system:ci", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := seedProject(t)
			if tc.acl != "" {
				writeProjectFile(t, root, "acl.yaml", tc.acl)
			}
			var logs bytes.Buffer
			logtest.SetDefault(t, slog.NewTextHandler(&logs, nil))

			svc, err := mcpServicesForTest(t, root)
			require.NoError(t, err)
			t.Cleanup(func() { _ = svc.Close() })
			warnUnassignedUser(svc.current(), tc.user)
			assert.Equal(t, tc.want, strings.Contains(logs.String(), "assigns no role to the local user"),
				"logs: %s", logs.String())
		})
	}
}
