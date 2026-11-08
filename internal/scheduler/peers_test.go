package scheduler

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/peersync"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/schedulerstate"
	"github.com/Sourcehaven-BV/rela/internal/secrets"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

const peerSchema = `
version: "1.0"
entities:
  requirement:
    label: Requirement
    id_prefix: "REQ-"
    properties:
      title: {type: string}
      cat: {type: external_ref, system: catalog}
`

func peerMeta(t *testing.T) *metamodel.Metamodel {
	t.Helper()
	m, err := metamodel.Parse([]byte(peerSchema))
	require.NoError(t, err)
	return m
}

func peersYAML(every string) []byte {
	line := ""
	if every != "" {
		line = "    every: " + every + "\n"
	}
	return []byte(`peers:
  cat:
    url: http://127.0.0.1:1
    credential: peer_credential.cat
` + line + `    entities:
      requirement: {ref: cat, fields: {title: title}}
`)
}

func TestLoadConfig_Peers(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name      string
		schedules string
		peers     []byte
		wantTasks []string
		wantErr   string
	}{
		{name: "scheduled", peers: peersYAML("1h"), wantTasks: []string{"peer:cat"}},
		{name: "manual only", peers: peersYAML(""), wantTasks: []string{}},
		{name: "below minimum", peers: peersYAML("1m"), wantTasks: []string{}, wantErr: "shorter than the 5m0s"},
		{name: "bad every", peers: peersYAML("often"), wantTasks: []string{}, wantErr: "every"},
		{
			name: "invalid keeps schedules", schedules: scheduleYAML, peers: []byte("peers: {cat: {url: x}}"),
			wantTasks: []string{"t"}, wantErr: "peers",
		},
		{
			name:      "name taken",
			schedules: "tasks:\n  - name: peer:cat\n    script: t.lua\n    every: 1h\n",
			peers:     peersYAML("1h"), wantTasks: []string{"peer:cat"}, wantErr: "is taken",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ws := newMockWorkspace(t)
			if tc.schedules != "" {
				ws.files["project:"+ConfigFile] = []byte(tc.schedules)
			}
			ws.files["project:"+peersync.FileName] = tc.peers
			cfg, err := LoadConfig(context.Background(), ws.Config(), peerMeta(t))
			if tc.wantErr == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tc.wantErr)
				require.ErrorIs(t, err, ErrPeers)
				require.NotErrorIs(t, err, ErrSubscriptions)
			}
			names := []string{}
			for _, task := range cfg.Tasks {
				names = append(names, task.Name)
				if task.Name == "peer:cat" && task.Peer != nil {
					require.Equal(t, "catalog", task.Peer.System)
				}
			}
			require.Equal(t, tc.wantTasks, names)
		})
	}
}

// TestReload_KeepsPeersOnABrokenFile pins that a broken peers.yaml keeps the
// running peer tasks, as a broken subscriptions.yaml keeps the feeds.
func TestReload_KeepsPeersOnABrokenFile(t *testing.T) {
	t.Parallel()
	ws := newMockWorkspace(t)
	task := peerTask(t, "1h")
	s, _, _ := newTestScheduler(t, ws, t0, task)
	s.reload = func(context.Context) (*Config, error) {
		return &Config{}, wrapPeers(errors.New("broken"))
	}
	s.reloadConfig(context.Background())
	got := s.config.Load().Tasks
	require.Len(t, got, 1)
	require.Equal(t, "peer:cat", got[0].Name)
}

func peerTask(t *testing.T, every string) TaskConfig {
	t.Helper()
	peers, err := peersync.Parse(peersYAML(every), peerMeta(t))
	require.NoError(t, err)
	cfg := &Config{}
	require.NoError(t, AddPeers(cfg, peers))
	require.Len(t, cfg.Tasks, 1)
	return cfg.Tasks[0]
}

// peerWorkspace has what a pull needs: the schema, a raw store, a manager
// and a secrets file holding the peer's credential.
type peerWorkspace struct {
	*mockWorkspace
	meta *metamodel.Metamodel
	st   store.Store
}

func (w peerWorkspace) Meta() *metamodel.Metamodel { return w.meta }
func (w peerWorkspace) Store() store.Store         { return w.st }

func newPeerWorkspace(t *testing.T, credential string) peerWorkspace {
	t.Helper()
	ws := newMockWorkspace(t)
	host := peerHost{}
	if credential != "" {
		host["peer_credential.cat"] = credential + "\n"
	}
	meta := peerMeta(t)
	ws.files["project:"+peersync.FileName] = peersYAML("1h")
	ws.luaDepsProvider = func() lua.WriteDeps {
		return lua.WriteDeps{ReadDeps: lua.ReadDeps{Meta: meta, Host: host}, EntityManager: peerWriter{}}
	}
	return peerWorkspace{mockWorkspace: ws, meta: meta, st: memstore.New()}
}

// peerHost is the host config holding the global secrets.
type peerHost map[string]string

func (h peerHost) Secrets(string) (map[string]string, error) {
	if len(h) == 0 {
		return nil, secrets.ErrNotFound
	}
	return h, nil
}
func (peerHost) File(string) ([]byte, error) { return nil, fs.ErrNotExist }
func (peerHost) Path() string                { return "" }

// peerWriter is a stubMutator that also has the manager's relation update,
// so it satisfies peersync.Writer.
type peerWriter struct{ stubMutator }

func (peerWriter) UpdateRelation(
	context.Context, entity.RelationKey, entity.RelationOptions,
) (*entity.Relation, error) {
	return nil, errors.New("stub")
}

// deniedTransport answers every request with 401 and records what it saw.
type deniedTransport struct{ auth []string }

func (d *deniedTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	d.auth = append(d.auth, req.Header.Get("Authorization"))
	return &http.Response{
		StatusCode: http.StatusUnauthorized, Header: http.Header{}, Body: http.NoBody, Request: req,
	}, nil
}

// TestPeerJob_RunsAsThePeer pins the job path: the task is queued as a peer
// job, the pull is bound as system:peer:<name> with its trigger, the
// credential is sent as a bearer token and never shows in the error, and a
// refused source fails the run and advances the ladder.
func TestPeerJob_RunsAsThePeer(t *testing.T) {
	t.Parallel()
	ws := newPeerWorkspace(t, "rela_pk_secretvalue")
	s, q, _ := newTestScheduler(t, ws, t0, peerTask(t, "1h"))
	tr := &deniedTransport{}
	s.peerTransport = tr
	var gotUser, gotTool, gotTrigger string
	s.bind = func(ctx context.Context) (context.Context, error) {
		p := principal.From(ctx)
		gotUser, gotTool, gotTrigger = p.User, p.Tool, audit.TriggeredByFrom(ctx)
		return ctx, nil
	}

	s.tick(context.Background())
	queued := q.take()
	require.Len(t, queued, 1)
	require.Equal(t, PeerKind, queued[0].Kind)
	err := q.deliver(t, queued[0])
	require.Error(t, err)
	require.NotContains(t, err.Error(), "secretvalue")

	require.Equal(t, "system:peer:cat", gotUser)
	require.Equal(t, principal.ToolPeerPull, gotTool)
	require.Equal(t, "peer:cat", gotTrigger)
	require.NotEmpty(t, tr.auth)
	require.Equal(t, "Bearer rela_pk_secretvalue", tr.auth[0])
	ts, _ := taskState(t, ws, "peer:cat")
	require.Equal(t, 1, ts.Failures)
}

func TestPeerJob_UnknownTask(t *testing.T) {
	t.Parallel()
	s, _, _ := newTestScheduler(t, newMockWorkspace(t), t0)
	err := s.pullPeerTask(context.Background(), "peer:gone", "r")
	require.ErrorContains(t, err, "not configured")
}

func TestPeerDeps_RefusesMissingCapabilities(t *testing.T) {
	t.Parallel()
	m := peerTask(t, "1h").Peer

	_, err := peerDeps(newMockWorkspace(t), m, "r", nil, discardLogger())
	require.ErrorContains(t, err, "no store")

	ws := newPeerWorkspace(t, "")
	_, err = peerDeps(ws, m, "r", nil, discardLogger())
	require.ErrorContains(t, err, `no secret "peer_credential.cat"`)

	ws.luaDepsProvider = func() lua.WriteDeps { return lua.WriteDeps{EntityManager: stubMutator{}} }
	_, err = peerDeps(ws, m, "r", nil, discardLogger())
	require.ErrorContains(t, err, "cannot write peer rows")

	ws.luaDepsProvider = func() lua.WriteDeps { return lua.WriteDeps{EntityManager: peerWriter{}} }
	_, err = peerDeps(ws, m, "r", nil, discardLogger())
	require.ErrorContains(t, err, "no schema")
}

func TestPullPeer(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	bind := func(ctx context.Context) (context.Context, error) { return ctx, nil }
	ws := newPeerWorkspace(t, "rela_pk_secretvalue")

	_, err := PullPeer(ctx, nil, bind, "cat", peersync.Options{}, discardLogger())
	require.Error(t, err)
	_, err = PullPeer(ctx, ws, nil, "cat", peersync.Options{}, discardLogger())
	require.Error(t, err)
	_, err = PullPeer(ctx, ws, bind, "cat", peersync.Options{}, nil)
	require.Error(t, err)
	_, err = PullPeer(ctx, ws, bind, "other", peersync.Options{}, discardLogger())
	require.ErrorContains(t, err, `no peer "other"`)

	// Nothing listens on the source's port, so the pull fails; it records
	// a failed run and frees the lock.
	_, err = PullPeer(ctx, ws, bind, "cat", peersync.Options{}, discardLogger())
	require.Error(t, err)
	ts, _ := taskState(t, ws, "peer:cat")
	require.Nil(t, ts.Active)
	require.Equal(t, 1, ts.Failures)

	// A run already active holds the lock.
	now := time.Now()
	require.NoError(t, ws.SchedulerState().CreateRun(ctx, schedulerstate.Run{
		ID: "held", Task: "peer:cat", CreatedAt: now, LeaseUntil: now.Add(time.Hour),
	}, ts.Version))
	_, err = PullPeer(ctx, ws, bind, "cat", peersync.Options{}, discardLogger())
	require.ErrorIs(t, err, ErrPeerBusy)

	// A dry run takes no lock.
	_, err = PullPeer(ctx, ws, bind, "cat", peersync.Options{DryRun: true}, discardLogger())
	require.NotErrorIs(t, err, ErrPeerBusy)
	require.NotContains(t, err.Error(), "secretvalue")
}

// staleStore answers CreateRun with ErrStale the first stale times.
type staleStore struct {
	schedulerstate.Store
	stale int
}

func (s *staleStore) CreateRun(ctx context.Context, run schedulerstate.Run, version int64) error {
	if s.stale > 0 {
		s.stale--
		return schedulerstate.ErrStale
	}
	return s.Store.CreateRun(ctx, run, version)
}

// TestStartManualRun_RetriesAStaleVersion pins that a tick updating the
// task's state between the read and the create is not reported as a
// running pull.
func TestStartManualRun_RetriesAStaleVersion(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ws := newPeerWorkspace(t, "")
	run, err := startManualRun(ctx, &staleStore{Store: ws.SchedulerState(), stale: 1}, "peer:cat", "r1")
	require.NoError(t, err)
	require.Equal(t, "r1", run.ID)

	_, err = startManualRun(ctx, &staleStore{Store: ws.SchedulerState(), stale: 2}, "peer:dog", "r2")
	require.ErrorIs(t, err, schedulerstate.ErrStale)
	require.NotErrorIs(t, err, ErrPeerBusy)
}

func TestWarnPull(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))
	issues := make([]peersync.Issue, maxWarnedIssues+3)
	for i := range issues {
		issues[i] = peersync.Issue{Kind: peersync.IssueConflict, Address: "E:req:R1", Reason: "both changed"}
	}
	warnPull(logger, "cat", "run-1", peersync.Report{Refused: "too many gone", Skipped: []string{"depends"}, Issues: issues})
	out := buf.String()
	for _, want := range []string{"peer pull refused", "peer pull skipped relation types", "more=3"} {
		if !strings.Contains(out, want) {
			t.Fatalf("log lacks %q:\n%s", want, out)
		}
	}
	if n := strings.Count(out, "peer pull row not applied"); n != maxWarnedIssues {
		t.Fatalf("logged %d issues, want %d", n, maxWarnedIssues)
	}
	buf.Reset()
	warnPull(logger, "cat", "run-2", peersync.Report{})
	if buf.Len() != 0 {
		t.Fatalf("a clean pull logged: %s", buf.String())
	}
}

func TestManualTrigger(t *testing.T) {
	ctx := principal.With(context.Background(), principal.Principal{User: "alice", Tool: principal.ToolCLI})
	require.Equal(t, "peer:cat by alice", manualTrigger(ctx, "cat"))
}
