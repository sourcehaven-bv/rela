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
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/remotesync"
	"github.com/Sourcehaven-BV/rela/internal/schedulerstate"
	"github.com/Sourcehaven-BV/rela/internal/secrets"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

const remoteSchema = `
version: "1.0"
entities:
  requirement:
    label: Requirement
    id_prefix: "REQ-"
    properties:
      title: {type: string}
      cat: {type: external_ref, system: catalog}
`

func remoteMeta(t *testing.T) *metamodel.Metamodel {
	t.Helper()
	m, err := metamodel.Parse([]byte(remoteSchema))
	require.NoError(t, err)
	return m
}

func remotesYAML(every string, extra ...string) []byte {
	var b strings.Builder
	if every != "" {
		b.WriteString("    every: " + every + "\n")
	}
	for _, e := range extra {
		b.WriteString("    " + e + "\n")
	}
	line := b.String()
	return []byte(`remotes:
  cat:
    url: http://127.0.0.1:1
    credential: {secret: remote_credential.cat}
` + line + `    entities:
      requirement: {ref: cat, fields: {title: title}}
`)
}

func TestLoadConfig_Remotes(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name      string
		schedules string
		remotes   []byte
		wantTasks []string
		wantErr   string
	}{
		{name: "scheduled", remotes: remotesYAML("1h"), wantTasks: []string{"remote:cat"}},
		{name: "manual only", remotes: remotesYAML(""), wantTasks: []string{}},
		{name: "below minimum", remotes: remotesYAML("1m"), wantTasks: []string{}, wantErr: "shorter than the 5m0s"},
		{name: "bad every", remotes: remotesYAML("often"), wantTasks: []string{}, wantErr: "every"},
		{
			name: "invalid keeps schedules", schedules: scheduleYAML, remotes: []byte("remotes: {cat: {url: x}}"),
			wantTasks: []string{"t"}, wantErr: "remotes",
		},
		{
			name:      "name taken",
			schedules: "tasks:\n  - name: remote:cat\n    script: t.lua\n    every: 1h\n",
			remotes:   remotesYAML("1h"), wantTasks: []string{"remote:cat"}, wantErr: "is taken",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ws := newMockWorkspace(t)
			if tc.schedules != "" {
				ws.files["project:"+ConfigFile] = []byte(tc.schedules)
			}
			ws.files["project:"+remotesync.FileName] = tc.remotes
			cfg, err := LoadConfig(context.Background(), ws.Config(), remoteMeta(t))
			if tc.wantErr == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tc.wantErr)
				require.ErrorIs(t, err, ErrRemotes)
				require.NotErrorIs(t, err, ErrSubscriptions)
			}
			names := []string{}
			for _, task := range cfg.Tasks {
				names = append(names, task.Name)
				if task.Name == "remote:cat" && task.Remote != nil {
					require.Equal(t, "catalog", task.Remote.System)
				}
			}
			require.Equal(t, tc.wantTasks, names)
		})
	}
}

// TestReload_KeepsRemotesOnABrokenFile pins that a broken remotes.yaml keeps the
// running remote tasks, as a broken subscriptions.yaml keeps the feeds.
func TestReload_KeepsRemotesOnABrokenFile(t *testing.T) {
	t.Parallel()
	ws := newMockWorkspace(t)
	task := remoteTask(t)
	s, _, _ := newTestScheduler(t, ws, t0, task)
	s.reload = func(context.Context) (*Config, error) {
		return &Config{}, wrapRemotes(errors.New("broken"))
	}
	s.reloadConfig(context.Background())
	got := s.config.Load().Tasks
	require.Len(t, got, 1)
	require.Equal(t, "remote:cat", got[0].Name)
}

func remoteTask(t *testing.T, extra ...string) TaskConfig {
	t.Helper()
	remotes, err := remotesync.Parse(remotesYAML("1h", extra...), remoteMeta(t))
	require.NoError(t, err)
	cfg := &Config{}
	require.NoError(t, AddRemotes(cfg, remotes))
	require.Len(t, cfg.Tasks, 1)
	return cfg.Tasks[0]
}

// remoteWorkspace has what a pull needs: the schema, a raw store, a manager
// and a secrets file holding the remote's credential.
type remoteWorkspace struct {
	*mockWorkspace
	meta *metamodel.Metamodel
	st   store.Store
}

func (w remoteWorkspace) Meta() *metamodel.Metamodel { return w.meta }
func (w remoteWorkspace) Store() store.Store         { return w.st }

func newRemoteWorkspace(t *testing.T, credential string) remoteWorkspace {
	t.Helper()
	ws := newMockWorkspace(t)
	host := remoteHost{}
	if credential != "" {
		host["remote_credential.cat"] = credential + "\n"
	}
	meta := remoteMeta(t)
	ws.files["project:"+remotesync.FileName] = remotesYAML("1h")
	ws.luaDepsProvider = func() lua.WriteDeps {
		return lua.WriteDeps{ReadDeps: lua.ReadDeps{Meta: meta, Host: host}, EntityManager: remoteWriter{}}
	}
	return remoteWorkspace{mockWorkspace: ws, meta: meta, st: memstore.New()}
}

// remoteHost is the host config holding the global secrets.
type remoteHost map[string]string

func (h remoteHost) Secrets(string) (map[string]string, error) {
	if len(h) == 0 {
		return nil, secrets.ErrNotFound
	}
	return h, nil
}
func (remoteHost) File(string) ([]byte, error) { return nil, fs.ErrNotExist }
func (remoteHost) Path() string                { return "" }

// remoteWriter is a stubMutator that also has the manager's relation update,
// so it satisfies remotesync.Writer.
type remoteWriter struct{ stubMutator }

func (remoteWriter) UpdateRelation(
	context.Context, entity.RelationKey, entity.RelationOptions,
) (*entity.Relation, error) {
	return nil, errors.New("stub")
}

// deniedTransport answers every request with 401 and records what it saw.
type deniedTransport struct{ auth, key []string }

func (d *deniedTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	d.auth = append(d.auth, req.Header.Get("Authorization"))
	d.key = append(d.key, req.Header.Get("X-Api-Key"))
	return &http.Response{
		StatusCode: http.StatusUnauthorized, Header: http.Header{}, Body: http.NoBody, Request: req,
	}, nil
}

// TestRemoteJob_RunsAsTheTaskIdentity pins the job path: the task is queued
// as a remote job, the pull is bound as the scheduler's identity or the
// remote's run_as with its trigger, the credential is sent as a bearer token
// and never shows in the error, and a refused source fails the run and
// advances the ladder.
func TestRemoteJob_RunsAsTheTaskIdentity(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, runAs, want string
	}{
		{"default", "", principal.UserScheduler},
		{"run_as", "integration:catalog-mirror", "integration:catalog-mirror"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ws := newRemoteWorkspace(t, "secretvalue")
			var task TaskConfig
			if tc.runAs == "" {
				task = remoteTask(t)
			} else {
				task = remoteTask(t, "run_as: "+tc.runAs)
			}
			s, q, _ := newTestScheduler(t, ws, t0, task)
			tr := &deniedTransport{}
			s.remoteTransport = tr
			var gotUser, gotTool, gotTrigger string
			s.bind = func(ctx context.Context) (context.Context, error) {
				p := principal.From(ctx)
				gotUser, gotTool, gotTrigger = p.User, p.Tool, audit.TriggeredByFrom(ctx)
				return ctx, nil
			}

			s.tick(context.Background())
			queued := q.take()
			require.Len(t, queued, 1)
			require.Equal(t, RemoteKind, queued[0].Kind)
			err := q.deliver(t, queued[0])
			require.Error(t, err)
			require.NotContains(t, err.Error(), "secretvalue")

			require.Equal(t, tc.want, gotUser)
			require.Equal(t, principal.ToolScheduler, gotTool)
			require.Equal(t, "remote:cat", gotTrigger)
			require.NotEmpty(t, tr.auth)
			require.Equal(t, "Bearer secretvalue", tr.auth[0])
			ts, _ := taskState(t, ws, "remote:cat")
			require.Equal(t, 1, ts.Failures)
		})
	}
}

func TestRemoteJob_UnknownTask(t *testing.T) {
	t.Parallel()
	s, _, _ := newTestScheduler(t, newMockWorkspace(t), t0)
	err := s.pullRemoteTask(context.Background(), "remote:gone", "r")
	require.ErrorContains(t, err, "not configured")
}

func TestRemoteDeps_RefusesMissingCapabilities(t *testing.T) {
	t.Parallel()
	m := remoteTask(t).Remote

	noEnv := func(string) string { return "" }
	_, err := remoteDeps(newMockWorkspace(t), m, "r", nil, discardLogger(), noEnv)
	require.ErrorContains(t, err, "no store")

	ws := newRemoteWorkspace(t, "")
	_, err = remoteDeps(ws, m, "r", nil, discardLogger(), noEnv)
	require.ErrorContains(t, err, "credential secret remote_credential.cat: no credential")

	ws.luaDepsProvider = func() lua.WriteDeps { return lua.WriteDeps{EntityManager: stubMutator{}} }
	_, err = remoteDeps(ws, m, "r", nil, discardLogger(), noEnv)
	require.ErrorContains(t, err, "cannot write remote rows")

	ws.luaDepsProvider = func() lua.WriteDeps { return lua.WriteDeps{EntityManager: remoteWriter{}} }
	_, err = remoteDeps(ws, m, "r", nil, discardLogger(), noEnv)
	require.ErrorContains(t, err, "no schema")

	ws.luaDepsProvider = func() lua.WriteDeps {
		return lua.WriteDeps{ReadDeps: lua.ReadDeps{Meta: ws.meta}, EntityManager: remoteWriter{}}
	}
	_, err = remoteDeps(ws, m, "r", nil, discardLogger(), noEnv)
	require.ErrorContains(t, err, "no secrets")
}

// TestRemoteDeps_EnvCredentialAndHeader pins a credential read from the
// environment and sent in the auth_header the remote names.
func TestRemoteDeps_EnvCredentialAndHeader(t *testing.T) {
	t.Parallel()
	remotes, err := remotesync.Parse([]byte(`remotes:
  cat:
    url: http://127.0.0.1:1
    credential: {env: REMOTE_CREDENTIAL_CAT}
    auth_header: X-Api-Key
    entities:
      requirement: {ref: cat, fields: {title: title}}
`), remoteMeta(t))
	require.NoError(t, err)
	m := remotes.Mappings["cat"]
	ws := newRemoteWorkspace(t, "")
	getenv := func(k string) string {
		if k == "REMOTE_CREDENTIAL_CAT" {
			return " envvalue\n"
		}
		return ""
	}
	tr := &deniedTransport{}
	deps, err := remoteDeps(ws, m, "r", tr, discardLogger(), getenv)
	require.NoError(t, err)
	_, _ = deps.Source.Get(context.Background(), "requirement", 1<<10)
	require.Equal(t, []string{"envvalue"}, tr.key)

	_, err = remoteDeps(ws, m, "r", tr, discardLogger(), func(string) string { return "" })
	require.ErrorContains(t, err, "credential env $REMOTE_CREDENTIAL_CAT: no credential")
}

func TestPullRemote(t *testing.T) {
	t.Parallel()
	ctx := principal.With(context.Background(), principal.Principal{User: "alice", Tool: principal.ToolCLI})
	var gotUser, gotTrigger string
	bind := func(ctx context.Context) (context.Context, error) {
		gotUser, gotTrigger = principal.From(ctx).User, audit.TriggeredByFrom(ctx)
		return ctx, nil
	}
	ws := newRemoteWorkspace(t, "secretvalue")

	_, err := PullRemote(ctx, nil, bind, "cat", remotesync.Options{}, discardLogger())
	require.Error(t, err)
	_, err = PullRemote(ctx, ws, nil, "cat", remotesync.Options{}, discardLogger())
	require.Error(t, err)
	_, err = PullRemote(ctx, ws, bind, "cat", remotesync.Options{}, nil)
	require.Error(t, err)
	_, err = PullRemote(ctx, ws, bind, "other", remotesync.Options{}, discardLogger())
	require.ErrorContains(t, err, `no remote "other"`)

	// Nothing listens on the source's port, so the pull fails; it records
	// a failed run and frees the lock.
	_, err = PullRemote(ctx, ws, bind, "cat", remotesync.Options{}, discardLogger())
	require.Error(t, err)
	// A pull by hand runs as the person who starts it.
	require.Equal(t, "alice", gotUser)
	require.Equal(t, "remote:cat", gotTrigger)
	ts, _ := taskState(t, ws, "remote:cat")
	require.Nil(t, ts.Active)
	require.Equal(t, 1, ts.Failures)

	// A run already active holds the lock.
	now := time.Now()
	require.NoError(t, ws.SchedulerState().CreateRun(ctx, schedulerstate.Run{
		ID: "held", Task: "remote:cat", CreatedAt: now, LeaseUntil: now.Add(time.Hour),
	}, ts.Version))
	_, err = PullRemote(ctx, ws, bind, "cat", remotesync.Options{}, discardLogger())
	require.ErrorIs(t, err, ErrRemoteBusy)

	// A dry run takes no lock.
	_, err = PullRemote(ctx, ws, bind, "cat", remotesync.Options{DryRun: true}, discardLogger())
	require.NotErrorIs(t, err, ErrRemoteBusy)
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
	ws := newRemoteWorkspace(t, "")
	run, err := startManualRun(ctx, &staleStore{Store: ws.SchedulerState(), stale: 1}, "remote:cat", "r1")
	require.NoError(t, err)
	require.Equal(t, "r1", run.ID)

	_, err = startManualRun(ctx, &staleStore{Store: ws.SchedulerState(), stale: 2}, "remote:dog", "r2")
	require.ErrorIs(t, err, schedulerstate.ErrStale)
	require.NotErrorIs(t, err, ErrRemoteBusy)
}

func TestWarnPull(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))
	issues := make([]remotesync.Issue, maxWarnedIssues+3)
	for i := range issues {
		issues[i] = remotesync.Issue{Kind: remotesync.IssueConflict, Address: "E:req:R1", Reason: "both changed"}
	}
	warnPull(logger, "cat", "run-1", remotesync.Report{Refused: "too many gone", Skipped: []string{"depends"}, Issues: issues})
	out := buf.String()
	for _, want := range []string{"remote pull refused", "remote pull skipped relation types", "more=3"} {
		if !strings.Contains(out, want) {
			t.Fatalf("log lacks %q:\n%s", want, out)
		}
	}
	if n := strings.Count(out, "remote pull row not applied"); n != maxWarnedIssues {
		t.Fatalf("logged %d issues, want %d", n, maxWarnedIssues)
	}
	buf.Reset()
	warnPull(logger, "cat", "run-2", remotesync.Report{})
	if buf.Len() != 0 {
		t.Fatalf("a clean pull logged: %s", buf.String())
	}
}
