package cli

import (
	"context"
	"os"
	"path/filepath"
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

	writeProjectFile(t, root, "realtime.yaml", "version: 1\nchannel_types: [broken")
	time.Sleep(500 * time.Millisecond)
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
	created, err := deps.EntityManager.CreateEntity(as("alice"),
		&entity.Entity{Type: "item", Properties: map[string]any{"title": "one"}}, entity.CreateOptions{})
	require.NoError(t, err, "entity writes stay open on stdio")

	addr := realtime.RawAddress{Anchor: created.Entity.ID, Type: "room"}
	_, err = deps.Channels.Reader.Read(as("alice"), addr, realtime.ReadRequest{})
	require.NoError(t, err, "alice reads items")
	_, err = deps.Channels.Reader.Read(as("mallory"), addr, realtime.ReadRequest{})
	assert.ErrorIs(t, err, realtime.ErrAnchorNotFound, "mallory reads no item under acl.yaml")
}
