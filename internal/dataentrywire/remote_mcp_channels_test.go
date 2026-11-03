//go:build !postgres

package dataentrywire

import (
	"context"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	mcpgo "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/attachment"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/script"
	"github.com/Sourcehaven-BV/rela/internal/storage"
)

const channelsRealtime = `version: 1
channel_types:
  room:
    attach_to: [ticket]
    roles:
      member: { from: attached }
    message_types:
      text:
        properties: { body: { type: string } }
        post: [member]
`

const channelsRealtimeWithNotes = channelsRealtime + `  notes:
    attach_to: [ticket]
    roles:
      member: { from: attached }
    message_types:
      text:
        properties: { body: { type: string } }
        post: [member]
`

// channelsServices builds a project with realtime.yaml, if set, and returns
// its root and services.
func channelsServices(t *testing.T, realtimeYAML string) (string, *appbuild.Services) {
	t.Helper()
	root := t.TempDir()
	for _, d := range []string{"entities", "relations", filepath.Join(".rela", "audit")} {
		if err := os.MkdirAll(filepath.Join(root, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	files := map[string]string{"metamodel.yaml": depsMetamodel, "acl.yaml": depsPolicy}
	if realtimeYAML != "" {
		files["realtime.yaml"] = realtimeYAML
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	fs := storage.NewSafeFS(storage.NewOsFS())
	paths, err := project.Discover(root, fs)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	svc, err := appbuild.New(appbuild.Config{
		FS: fs, Paths: paths, ScriptEngine: script.NewEngine(), Audit: audit.Nop{},
	})
	if err != nil {
		t.Fatalf("appbuild.New: %v", err)
	}
	t.Cleanup(func() { _ = svc.Close() })
	return root, svc
}

// TestRemoteMCPDeps_Channels pins the remote channel wiring: the tools use
// the HTTP routes' channel service with signals kept, poll every 5s, and
// follow a realtime.yaml reload without a new server.
func TestRemoteMCPDeps_Channels(t *testing.T) {
	root, svc := channelsServices(t, channelsRealtime)
	deps, err := RemoteMCPDeps(svc, testHost(t, svc))
	if err != nil {
		t.Fatalf("RemoteMCPDeps: %v", err)
	}
	cs := appbuild.ChannelService(svc)
	ch := deps.Channels
	if cs == nil || ch.Reader != cs || ch.Writer != cs || ch.Creator != cs || ch.Waker != cs {
		t.Fatal("the channel tools must use the bundle's channel service")
	}
	if ch.RefuseSignals {
		t.Error("rela-server keeps signals; the remote tools must not refuse them")
	}
	if ch.LocalIdentity {
		t.Error("a remote caller's identity is not $USER; the hint must stay off")
	}
	if deps.Watch.ChannelPoll != remoteChannelPoll {
		t.Errorf("ChannelPoll = %v, want %v", deps.Watch.ChannelPoll, remoteChannelPoll)
	}
	if names := remoteToolNames(t, svc); !slices.Contains(names, "read_channel") {
		t.Errorf("remote MCP does not list read_channel: %v", names)
	}

	if err := os.WriteFile(filepath.Join(root, "realtime.yaml"), []byte(channelsRealtimeWithNotes), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := appbuild.Realtime(svc).Reload(context.Background()); err != nil {
		t.Fatalf("Reload: %v", err)
	}
	if _, ok := cs.Config().ChannelType("notes"); !ok {
		t.Error("the channel service did not follow the realtime.yaml reload")
	}
}

// TestRemoteMCPDeps_NoChannelsWithoutRealtime pins that a project without
// realtime.yaml registers no channel tool on the remote server. The service
// exists, so a realtime.yaml added later needs only a reload, but it reports
// itself disabled.
func TestRemoteMCPDeps_NoChannelsWithoutRealtime(t *testing.T) {
	_, svc := channelsServices(t, "")
	cs := appbuild.ChannelService(svc)
	if cs == nil {
		t.Fatal("the bundle has no channel service")
	}
	deps, err := RemoteMCPDeps(svc, testHost(t, svc))
	if err != nil {
		t.Fatalf("RemoteMCPDeps: %v", err)
	}
	if deps.Channels.Reader != cs {
		t.Fatal("the channel tools must use the bundle's channel service")
	}
	if cs.Enabled() {
		t.Fatal("the channel service is enabled without realtime.yaml")
	}
	for _, name := range remoteToolNames(t, svc) {
		if strings.Contains(name, "channel") || strings.Contains(name, "message") {
			t.Errorf("remote MCP lists %s without realtime.yaml", name)
		}
	}
}

// remoteToolNames lists the tools of the remote server over svc.
func remoteToolNames(t *testing.T, svc *appbuild.Services) []string {
	t.Helper()
	ctx := context.Background()
	host := testHost(t, svc)
	host.AttachmentUploads = attachment.NewLimiter(attachment.DefaultMaxUploads)
	srv, err := RemoteMCPServer(svc, host, "test")
	if err != nil {
		t.Fatalf("RemoteMCPServer: %v", err)
	}
	ts := httptest.NewServer(srv.HTTPHandler())
	t.Cleanup(ts.Close)
	client := mcpgo.NewClient(&mcpgo.Implementation{Name: "test", Version: "0"}, nil)
	session, err := client.Connect(ctx, &mcpgo.StreamableClientTransport{
		Endpoint: ts.URL, DisableStandaloneSSE: true,
	}, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })
	tools, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("tools/list: %v", err)
	}
	names := make([]string, 0, len(tools.Tools))
	for _, tool := range tools.Tools {
		names = append(names, tool.Name)
	}
	return names
}
