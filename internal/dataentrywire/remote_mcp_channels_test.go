//go:build !postgres

package dataentrywire

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/appbuild"
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
	if cs == nil || deps.Channels.Reader != cs || deps.Channels.Writer != cs {
		t.Fatal("the channel tools must use the bundle's channel service")
	}
	if deps.Channels.RefuseSignals {
		t.Error("rela-server keeps signals; the remote tools must not refuse them")
	}
	if deps.Watch.ChannelPoll != remoteChannelPoll {
		t.Errorf("ChannelPoll = %v, want %v", deps.Watch.ChannelPoll, remoteChannelPoll)
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
// realtime.yaml wires no channel tools.
func TestRemoteMCPDeps_NoChannelsWithoutRealtime(t *testing.T) {
	_, svc := channelsServices(t, "")
	deps, err := RemoteMCPDeps(svc, testHost(t, svc))
	if err != nil {
		t.Fatalf("RemoteMCPDeps: %v", err)
	}
	if appbuild.ChannelService(svc) == nil && deps.Channels.Reader != nil {
		t.Error("channel tools wired without a channel service")
	}
	if deps.Channels.Reader != nil && deps.Channels.Reader.Enabled() {
		t.Error("channel tools enabled without realtime.yaml")
	}
}
