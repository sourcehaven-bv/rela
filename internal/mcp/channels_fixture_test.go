package mcp

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	mcpgo "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/acl/acltest"
	"github.com/Sourcehaven-BV/rela/internal/aclstore"
	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/appbuild/appbuildtest"
	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/channels/filechannels"
	"github.com/Sourcehaven-BV/rela/internal/filelock"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/realtime"
	"github.com/Sourcehaven-BV/rela/internal/realtimecfg"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// Channel tool tests run the production channel service, built by
// appbuildtest over a file channel store on an in-memory FS, behind a
// declarative ACL:
//
//   - alice reads and updates requirements;
//   - carol reads requirements;
//   - bob reads and updates requirements and decisions, and holds chat.read.
//
// So a decision is a hidden anchor for alice and carol, and only bob reads
// a secret channel. chat offers the built-in typing indicator. Nobody holds chat.moderate, so nobody posts an announce
// or a cursor.
const channelTestYAML = `
version: 1
channel_types:
  room:
    label: Room
    attach_to: [requirement, decision]
    read: { from: attached }
    roles:
      member: { from: attached }
      mod:    { permission: chat.moderate }
    message_types:
      text:
        properties:
          body:  { type: string, required: true }
          about: { type: ref, to: [requirement, decision] }
        post: [member]
      typing: { persist: ephemeral, ttl: 30s, post: [member] }
      cursor: { persist: ephemeral, ttl: 30s, post: [mod] }
      announce:
        properties: { body: { type: string } }
        post: [mod]
      slow:
        rate: 1/m
        properties: { body: { type: string } }
        post: [member]
    message_ops:
      edit_own:   { roles: [member], within: 15m }
      delete_own: { roles: [member] }
  threads:
    attach_to: [requirement, decision]
    cardinality: many
    read: { from: attached }
    create: { roles: [member] }
    roles:
      member: { from: attached }
    message_types:
      text:
        properties: { body: { type: string } }
        post: [member]
  incident:
    attach_to: [requirement]
    read: { from: attached }
    roles:
      member: { from: attached }
    message_types:
      text:      { properties: { body: { type: string } }, post: [member] }
      contained: { properties: { note: { type: string } }, post: [member] }
    session:
      initial: detected
      states:
        detected:  { label: Detected, on: { contained: { to: contained } } }
        contained: { label: Contained, final: true }
  chat:
    attach_to: [requirement]
    typing: true
    read: { from: attached }
    roles:
      member: { from: attached }
    message_types:
      text: { properties: { body: { type: string } }, post: [member] }
      status: { persist: ephemeral, ttl: 30s, post: [member] }
  secret:
    attach_to: [requirement]
    read: { permission: chat.read }
    roles:
      member: { from: attached }
    message_types:
      text:
        properties: { body: { type: string } }
        post: [member]
`

// outputsYAML is channelTestYAML with outputs on the incident's contained
// transition: it sets the status and creates a decision.
var outputsYAML = strings.Replace(channelTestYAML,
	"detected:  { label: Detected, on: { contained: { to: contained } } }",
	`detected:
          label: Detected
          on:
            contained:
              to: contained
              outputs:
                - set: { properties: { status: contained } }
                - create: { type: decision, properties: { title: Contain, status: proposed }, relate: { type: addresses } }`, 1)

// fakeOutputs is an OutputWriter that allows every output and writes
// nothing. A create answers with DEC-001, which only bob reads; a set on
// REQ-002 fails as invalid on status.
type fakeOutputs struct{}

func (fakeOutputs) Poster(context.Context) (realtime.OutputPoster, error) {
	return realtime.OutputPoster{}, nil
}

func (fakeOutputs) Check(context.Context, realtime.OutputTarget) error { return nil }

func (fakeOutputs) Write(ctx context.Context, r realtime.OutputRequest) (realtime.OutputWritten, error) {
	if r.Target.Kind == realtimecfg.OutputSet && r.Target.AnchorID == "REQ-002" {
		return realtime.OutputWritten{}, &realtime.OutputError{
			Code: realtime.OutputCodeInvalid, Props: []string{"status"}, WroteNothing: true,
		}
	}
	if r.Target.Kind != realtimecfg.OutputCreate {
		return realtime.OutputWritten{}, nil
	}
	if r.Started != nil {
		if err := r.Started(ctx); err != nil {
			return realtime.OutputWritten{}, err
		}
	}
	return realtime.OutputWritten{Entity: &channels.Promoted{ID: "DEC-001", Type: "decision"}}, nil
}

// testChannelConfig is a realtime.ConfigSource over a pointer a test can
// swap; nil is realtime off.
type testChannelConfig struct {
	p *atomic.Pointer[realtimecfg.Compiled]
}

func (c testChannelConfig) Load() realtime.Snapshot { return realtime.Snapshot{Config: c.p.Load()} }

// channelFixture is a seeded store, a channel store and a realtime.yaml
// pointer that every service of the fixture shares.
type channelFixture struct {
	meta *metamodel.Metamodel
	st   *memstore.MemStore
	cs   channels.Store
	cfg  *atomic.Pointer[realtimecfg.Compiled]
	d    *acl.Declarative
	// svc is the first service instance; servers use it.
	svc  *realtime.Service
	bndl *appbuild.Services
}

func newChannelFixture(t *testing.T) *channelFixture {
	t.Helper()
	meta, st := makeTestFixture(t)
	cs, err := filechannels.New(storage.NewMemFS(), "/channels", filelock.ProcessPrivate{})
	if err != nil {
		t.Fatal(err)
	}
	d, err := acl.NewDeclarative(&acl.Policy{
		Roles: map[string]acl.RoleDef{
			"editor": {Read: []string{"requirement"}, Update: []string{"requirement"}},
			"viewer": {Read: []string{"requirement"}},
			"admin": {
				Read: []string{"requirement", "decision"}, Update: []string{"requirement", "decision"},
				Permissions: []string{"chat.read"},
			},
		},
		Assignments: map[string]string{"alice": "editor", "carol": "viewer", "bob": "admin"},
	}, aclstore.MustNewGraph(st), st)
	if err != nil {
		t.Fatal(err)
	}
	f := &channelFixture{meta: meta, st: st, cs: cs, cfg: &atomic.Pointer[realtimecfg.Compiled]{}, d: d}
	compiled, err := realtimecfg.Parse([]byte(channelTestYAML), meta, nil)
	if err != nil {
		t.Fatal(err)
	}
	f.cfg.Store(compiled)
	f.bndl, f.svc = f.instance(t)
	return f
}

// instance builds another service over the fixture's stores, with its own
// process-local state, as another process would have.
func (f *channelFixture) instance(t *testing.T) (*appbuild.Services, *realtime.Service) {
	t.Helper()
	b := appbuildtest.New(f.meta, appbuildtest.WithStore(f.st), appbuildtest.WithDeclarative(f.d),
		appbuildtest.WithChannels(f.cs, testChannelConfig{p: f.cfg}), appbuildtest.WithChannelOutputs(fakeOutputs{}))
	t.Cleanup(func() { _ = b.Close() })
	svc := appbuild.ChannelService(b)
	if svc == nil {
		t.Fatal("appbuildtest built no channel service")
	}
	return b, svc
}

// deps is the dependency bundle of a server over the fixture.
func (f *channelFixture) deps(t *testing.T, refuseSignals bool) Deps {
	t.Helper()
	reads := f.bndl.GatedReads()
	return Deps{
		Store:         reads.Reader,
		Related:       RelatedReads{Traversals: reads.Traversals},
		Meta:          f.meta,
		Tracer:        reads.Tracer,
		Searcher:      reads.Searcher,
		Validator:     reads.Validator,
		EntityManager: f.bndl.EntityManager(),
		Config:        f.bndl.Config(),
		Lua:           LuaDeps{WriteDeps: f.bndl.LuaWriteDeps(), ProjectRoot: t.TempDir()},
		Watcher:       nopWatcher{},
		Watch:         WatchDeps{Changes: f.st, Rebind: reads.Rebind, ChannelPoll: 100 * time.Millisecond},
		Attachments:   testAttachmentDeps(t, f.bndl, f.meta),
		World:         store.TrivialScope(),
		Families:      store.TrivialScope(),
		BindIdentity:  reads.Rebind,
		Channels: ChannelDeps{
			Reader: f.svc, Sessions: f.svc, Writer: f.svc, Creator: f.svc, Waker: f.svc,
			RefuseSignals: refuseSignals,
		},
	}
}

// server builds a server for user over the fixture.
func (f *channelFixture) server(t *testing.T, user string, refuseSignals bool) *Server {
	t.Helper()
	srv, err := NewServer(f.deps(t, refuseSignals), "test",
		WithPrincipal(principal.Principal{User: user, Tool: principal.ToolMCP}))
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	srv.logger = slog.New(slog.DiscardHandler)
	return srv
}

// client connects a session to a server for user.
func (f *channelFixture) client(t *testing.T, user string) *mcpgo.ClientSession {
	t.Helper()
	return connect(t, f.server(t, user, false))
}

// asBob is bob's context bound on the fixture's ACL, for direct service
// calls.
func (f *channelFixture) asBob(t *testing.T) context.Context {
	t.Helper()
	bob := principal.Principal{User: "bob", Tool: principal.ToolMCP}
	return acltest.Bind(principal.With(context.Background(), bob), t, f.d)
}

// postAsBob posts a text message in the room of anchor as bob through svc.
func (f *channelFixture) postAsBob(t *testing.T, svc *realtime.Service, anchor, body string) {
	t.Helper()
	_, err := svc.Post(f.asBob(t), realtime.RawAddress{Anchor: anchor, Type: "room"},
		realtime.PostInput{Type: "text", Props: map[string]any{"body": body}})
	if err != nil {
		t.Fatalf("post in the room of %s as bob: %v", anchor, err)
	}
}

// call calls a tool and returns its text and error flag.
func call(t *testing.T, cs *mcpgo.ClientSession, tool string, args map[string]any) (string, bool) {
	t.Helper()
	raw, err := json.Marshal(args)
	if err != nil {
		t.Fatal(err)
	}
	res, err := cs.CallTool(context.Background(), &mcpgo.CallToolParams{Name: tool, Arguments: json.RawMessage(raw)})
	if err != nil {
		t.Fatalf("%s: %v", tool, err)
	}
	return res.Content[0].(*mcpgo.TextContent).Text, res.IsError
}

// mustCall calls a tool that must succeed and decodes its result into out.
func mustCall(t *testing.T, cs *mcpgo.ClientSession, tool string, args map[string]any, out any) string {
	t.Helper()
	text, isErr := call(t, cs, tool, args)
	if isErr {
		t.Fatalf("%s %v: %s", tool, args, text)
	}
	if out != nil {
		if err := json.Unmarshal([]byte(text), out); err != nil {
			t.Fatalf("%s: decode %s: %v", tool, text, err)
		}
	}
	return text
}

// toolNames lists the tools a session's server registers.
func toolNames(t *testing.T, cs *mcpgo.ClientSession) map[string]*mcpgo.Tool {
	t.Helper()
	res, err := cs.ListTools(context.Background(), &mcpgo.ListToolsParams{})
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]*mcpgo.Tool{}
	for _, tool := range res.Tools {
		out[tool.Name] = tool
	}
	return out
}

// channelToolNames are the tools registered with realtime on.
var channelToolNames = []string{
	"list_channels", toolReadChannel, toolReadSession, "post_message", "edit_message", "remove_message", "create_channel",
}

// noticeFirst reports whether a result text is a JSON object whose first
// key is notice.
func noticeFirst(text string) bool {
	return strings.HasPrefix(text, `{"notice":`+jsonString(realtime.UntrustedNotice)+`,`)
}

func jsonString(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

// typeAsBob starts or refreshes bob's typing entry in the chat of anchor.
func (f *channelFixture) typeAsBob(t *testing.T, anchor string) {
	t.Helper()
	if _, err := f.svc.Typing(f.asBob(t), realtime.RawAddress{Anchor: anchor, Type: "chat"}); err != nil {
		t.Fatalf("typing in the chat of %s as bob: %v", anchor, err)
	}
}

// useOutputs swaps in outputsYAML.
func (f *channelFixture) useOutputs(t *testing.T) {
	t.Helper()
	compiled, err := realtimecfg.Parse([]byte(outputsYAML), f.meta, nil)
	if err != nil {
		t.Fatal(err)
	}
	f.cfg.Store(compiled)
}
