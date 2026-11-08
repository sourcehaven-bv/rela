package appbuild

import (
	"context"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/ai"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/hostconfig"
	"github.com/Sourcehaven-BV/rela/internal/mail"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// seamSchema reads the clock on every path WithClock reaches, and runs one
// Lua automation that probes the clock, HTTP, AI, mail and write_file by
// title. The probe writes what it saw into note.
const seamSchema = `version: "1.0"
entities:
  ticket:
    label: Ticket
    plural: tickets
    id_prefix: "TKT-"
    id_type: sequential
    query_scopes:
      started: "entity.start <= today()"
    properties:
      title: {type: string}
      start: {type: date}
      stamp: {type: string}
      note: {type: string}
      age: {type: integer, computed: "days_between(today(), entity.start)"}
relations: {}
validations:
  - name: no-future-start
    entity_type: ticket
    enforce: write
    when_condition: "entity.start > today()"
    then: ["title=never"]
automations:
  - name: stamp
    on: {entity: [ticket], created: true}
    do:
      - set: stamp
        value: "{{today}}"
  - name: probe
    on: {entity: [ticket], created: true}
    do:
      - lua: |
          local t = entity.properties.title
          local out = "none"
          if t == "clock" then
            out = rela.today
          elseif t == "http" then
            local _, err = http.get("http://127.0.0.1:9/")
            out = err and err.message or "sent"
          elseif t == "ai" then
            local _, err = ai.chat({messages = {{role = "user", content = "hi"}}})
            out = err and err.message or "answered"
          elseif t == "mail-allowed" or t == "mail-blocked" then
            local to = "ok@example.com"
            if t == "mail-blocked" then to = "out@elsewhere.test" end
            local _, err = mail.send{to = to, subject = "s", text = "b"}
            out = err and err.kind or "sent"
          elseif t == "file" then
            rela.write_file("probe.txt", rela.today)
            out = "written"
          end
          rela.update_entity(entity.id, {note = out})
        capabilities: {http: true, ai: true, mail: true, write_file: true}
`

// seamPinned is the fixed time the clock tests run at: far enough from the
// wall clock that a leak to time.Now shows.
var seamPinned = time.Date(2020, 1, 1, 12, 0, 0, 0, time.UTC)

func writeSeamProject(t *testing.T, mailYAML string) string {
	t.Helper()
	root := t.TempDir()
	writeFile(t, root, "schema.yaml", seamSchema)
	writeFile(t, root, "acl.yaml", "roles:\n  editor:\n    read: [ticket]\n    create: [ticket]\n"+
		"    update: [ticket]\nassignments:\n  alice: editor\n")
	require.NoError(t, os.MkdirAll(filepath.Join(root, "relations"), 0o755))
	if mailYAML != "" {
		writeFile(t, root, ".rela/mail.yaml", mailYAML)
	}
	return root
}

// probe creates a ticket with the given title and start, and returns the
// stored row after its automations ran.
func probe(t *testing.T, svc *Services, title, start string) *entity.Entity {
	t.Helper()
	ctx := memWorldAs(t, svc, "alice")
	res, err := svc.EntityManager().CreateEntity(ctx,
		&entity.Entity{Type: "ticket", Properties: map[string]any{"title": title, "start": start}},
		entity.CreateOptions{})
	require.NoError(t, err)
	require.Empty(t, res.AutomationErrors)
	got, err := svc.Store().GetEntity(context.Background(), res.Entity.Ref())
	require.NoError(t, err)
	return got
}

// smtpTrap is a listener that counts connections, standing in for the SMTP
// server mail.yaml names.
func smtpTrap(t *testing.T) (port string, conns *atomic.Int32) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = ln.Close() })
	conns = &atomic.Int32{}
	go func() {
		for {
			c, acceptErr := ln.Accept()
			if acceptErr != nil {
				return
			}
			conns.Add(1)
			_ = c.Close()
		}
	}()
	_, port, err = net.SplitHostPort(ln.Addr().String())
	require.NoError(t, err)
	return port, conns
}

func smtpMailYAML(port string) string {
	return "transport: smtp\nhost: 127.0.0.1\nport: " + port + "\nfrom: rela@example.com\n" +
		"recipients:\n  also_allow: [ok@example.com]\n"
}

func TestWithClock_ReachesEveryWritePathClock(t *testing.T) {
	root := writeSeamProject(t, "")
	svc, err := NewInMemory(root, WithClock(func() time.Time { return seamPinned }))
	require.NoError(t, err)
	t.Cleanup(func() { _ = svc.Close() })

	got := probe(t, svc, "clock", "2019-12-31")
	require.Equal(t, "2020-01-01", got.Properties["stamp"], "{{today}} in an automation")
	require.Equal(t, "2020-01-01", got.Properties["note"], "rela.today in automation Lua")
	require.True(t, got.UpdatedAt.Equal(seamPinned), "memstore UpdatedAt: %v", got.UpdatedAt)

	read, err := svc.LuaReadDeps().VisibleReader.GetAddress(memWorldAs(t, svc, "alice"), got.ID)
	require.NoError(t, err)
	require.EqualValues(t, 1, read.Properties["age"], "computed property")

	// A start after the pinned day is in the future for the write rule, though
	// it is long past on the wall clock.
	_, err = svc.EntityManager().CreateEntity(memWorldAs(t, svc, "alice"),
		&entity.Entity{Type: "ticket", Properties: map[string]any{"title": "x", "start": "2020-06-01"}},
		entity.CreateOptions{})
	require.Error(t, err, "enforce: write rule with today()")

	resolver, problems := svc.base.QueryScopes(nil, svc.Meta())
	require.Empty(t, problems)
	scope, _, ok := resolver.Resolve("ticket", "started")
	require.True(t, ok)
	kept, err := resolver.Filter(context.Background(), scope, "ticket", []store.EntityHeader{
		{ID: "TKT-A", Type: "ticket", Properties: map[string]any{"start": "2019-12-31"}},
		{ID: "TKT-B", Type: "ticket", Properties: map[string]any{"start": "2020-06-01"}},
	}, nil, nil)
	require.NoError(t, err)
	require.Len(t, kept, 1, "query scope with today()")
	require.Equal(t, "TKT-A", kept[0].ID)
}

func TestWithClock_ValidationUsesWorldClock(t *testing.T) {
	root := writeSeamProject(t, "")
	svc, err := NewInMemory(root, WithClock(func() time.Time { return seamPinned }))
	require.NoError(t, err)
	t.Cleanup(func() { _ = svc.Close() })

	// Stored past the write rule by an unchecked store write, as a hand edit
	// would be; the analyze path must judge it on the pinned day.
	e := &entity.Entity{ID: "TKT-9", Type: "ticket", Properties: map[string]any{"title": "x", "start": "2020-06-01"}}
	require.NoError(t, svc.Store().CreateEntity(context.Background(), e))
	findings, err := svc.Validator().CheckAll(memWorldAs(t, svc, "alice"))
	require.NoError(t, err)
	var names []string
	for _, f := range findings {
		names = append(names, f.RuleName)
	}
	require.Contains(t, names, "no-future-start")
}

func TestNewInMemory_DefaultsKeepTheWorldOffTheNetwork(t *testing.T) {
	port, conns := smtpTrap(t)
	root := writeSeamProject(t, smtpMailYAML(port))
	writeFile(t, root, ".rela/ai.yaml", "base_url: http://127.0.0.1:"+port+"\nmodel: m\n")

	svc, err := NewInMemory(root)
	require.NoError(t, err)
	t.Cleanup(func() { _ = svc.Close() })

	require.Equal(t, "sent", probe(t, svc, "mail-allowed", "2019-01-01").Properties["note"])
	require.Equal(t, "recipients_not_allowed", probe(t, svc, "mail-blocked", "2019-01-01").Properties["note"],
		"mail.yaml's recipients policy still applies")
	require.Contains(t, probe(t, svc, "http", "2019-01-01").Properties["note"], ErrRealNetwork.Error())
	require.Contains(t, probe(t, svc, "ai", "2019-01-01").Properties["note"], ErrRealNetwork.Error())
	require.Zero(t, conns.Load(), "no connection may reach the configured SMTP or AI host")
}

// TestNewInMemory_ZeroOptionsKeepTheDefaults pins that a nil or empty option
// is ignored rather than switching a safe default off.
func TestNewInMemory_ZeroOptionsKeepTheDefaults(t *testing.T) {
	port, conns := smtpTrap(t)
	root := writeSeamProject(t, smtpMailYAML(port))
	svc, err := NewInMemory(root, WithMailTransport(nil), WithHTTPClient(nil), WithAIProvider(nil),
		WithOutputDir(""), WithClock(nil))
	require.NoError(t, err)
	t.Cleanup(func() { _ = svc.Close() })

	require.Equal(t, "sent", probe(t, svc, "mail-allowed", "2019-01-01").Properties["note"])
	require.Contains(t, probe(t, svc, "http", "2019-01-01").Properties["note"], ErrRealNetwork.Error())
	require.Contains(t, probe(t, svc, "ai", "2019-01-01").Properties["note"], ErrRealNetwork.Error())
	require.Equal(t, "written", probe(t, svc, "file", "2019-01-01").Properties["note"])
	require.False(t, inside(root, svc.base.opts.seams.outputDir))
	_, err = os.Stat(filepath.Join(svc.base.opts.seams.outputDir, "probe.txt"))
	require.NoError(t, err)
	require.Zero(t, conns.Load())
}

func TestNewInMemory_InjectedSeamsWin(t *testing.T) {
	port, conns := smtpTrap(t)
	root := writeSeamProject(t, smtpMailYAML(port))
	transport := mail.NewMemorySender(0)
	var hits atomic.Int32
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		hits.Add(1)
		return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody, Header: http.Header{}, Request: r}, nil
	})}
	out := t.TempDir()

	svc, err := NewInMemory(root, WithMailTransport(transport), WithHTTPClient(client),
		WithAIProvider(fakeAI{}), WithOutputDir(out))
	require.NoError(t, err)

	require.Equal(t, "sent", probe(t, svc, "mail-allowed", "2019-01-01").Properties["note"])
	require.Equal(t, "recipients_not_allowed", probe(t, svc, "mail-blocked", "2019-01-01").Properties["note"])
	msgs := transport.Messages()
	require.Len(t, msgs, 1, "only the allowed message reaches the injected transport")
	require.Equal(t, "ok@example.com", msgs[0].To[0].Email)

	require.Equal(t, "sent", probe(t, svc, "http", "2019-01-01").Properties["note"])
	require.EqualValues(t, 1, hits.Load())
	require.Equal(t, "answered", probe(t, svc, "ai", "2019-01-01").Properties["note"])
	require.Equal(t, "written", probe(t, svc, "file", "2019-01-01").Properties["note"])
	_, err = os.Stat(filepath.Join(out, "probe.txt"))
	require.NoError(t, err, "write_file writes to the injected output dir")
	require.Zero(t, conns.Load())

	require.NoError(t, svc.Close())
	_, err = os.Stat(out)
	require.NoError(t, err, "a caller's output dir is not removed on Close")
}

func TestNewInMemory_DefaultOutputDirIsTemporary(t *testing.T) {
	root := writeSeamProject(t, "")
	svc, err := NewInMemory(root)
	require.NoError(t, err)
	dir := svc.base.opts.seams.outputDir
	require.NotEmpty(t, dir)
	require.False(t, inside(root, dir), "the default output dir must be outside the project")

	require.Equal(t, "written", probe(t, svc, "file", "2019-01-01").Properties["note"])
	_, err = os.Stat(filepath.Join(dir, "probe.txt"))
	require.NoError(t, err)

	require.NoError(t, svc.Close())
	_, err = os.Stat(dir)
	require.ErrorIs(t, err, os.ErrNotExist, "Close removes the default output dir")
}

func TestInside(t *testing.T) {
	tests := []struct {
		root, path string
		want       bool
	}{
		{"/p", "/p", true},
		{"/p", "/p/out", true},
		{"/p", "/tmp/out", false},
		{"/p", "/pp", false},
		{"/p/a", "/p", false},
	}
	// A symlinked spelling of a directory inside the root is inside it.
	target := t.TempDir()
	link := filepath.Join(t.TempDir(), "link")
	require.NoError(t, os.Symlink(target, link))
	require.NoError(t, os.Mkdir(filepath.Join(target, "out"), 0o755))
	require.True(t, inside(target, filepath.Join(link, "out")))
	for _, tc := range tests {
		t.Run(tc.root+"|"+tc.path, func(t *testing.T) {
			require.Equal(t, tc.want, inside(tc.root, tc.path))
		})
	}
}

func TestMailOnlyHost(t *testing.T) {
	root := writeSeamProject(t, "transport: memory\nfrom: rela@example.com\n")
	writeFile(t, root, ".rela/ai.yaml", "base_url: http://x\nmodel: m\n")
	writeFile(t, root, ".rela/secrets.yaml", "k: v\n")
	h := mailOnlyHost{dir: hostconfig.Dir(filepath.Join(root, ".rela"))}

	body, err := h.File("mail.yaml")
	require.NoError(t, err)
	require.Contains(t, string(body), "transport: memory")
	_, err = h.File("ai.yaml")
	require.ErrorIs(t, err, os.ErrNotExist)
	_, err = h.Secrets("lua")
	require.Error(t, err)
	require.Equal(t, filepath.Join(root, ".rela"), h.Path())
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// fakeAI answers every call.
type fakeAI struct{}

func (fakeAI) Chat(context.Context, ai.ChatRequest) (*ai.ChatResponse, error) {
	return &ai.ChatResponse{Content: "ok"}, nil
}

func (fakeAI) Embed(context.Context, ai.EmbedRequest) (*ai.EmbedResponse, error) {
	return &ai.EmbedResponse{}, nil
}
