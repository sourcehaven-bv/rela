package appbuild_test

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/flow"
	"github.com/Sourcehaven-BV/rela/internal/flowrun"
)

// jobFlows adds two job-step flows to the sample: mark runs a script that
// writes the subject's stage as run_as, hook posts a signed webhook to URL.
const jobFlows = `
  mark:
    subject: person
    run_as: system:onboarding-bot
    per_subject: many
    start: [{ on: manual, permission: onboarding.start }]
    initial: send
    states:
      send:
        job:
          script: flows/mark.lua
        on:
          done: { to: sent }
      sent: { final: true }
  hook:
    subject: person
    run_as: system:onboarding-bot
    per_subject: many
    start: [{ on: manual, permission: onboarding.start }]
    initial: send
    states:
      send:
        job:
          kind: webhook
          url: URL
          secret: hook_key
          retry: { attempts: 2, backoff: { initial: 1s } }
        on:
          done: { to: sent }
      sent: { final: true }
  act:
    subject: person
    run_as: system:onboarding-bot
    per_subject: many
    start: [{ on: manual, permission: onboarding.start }]
    initial: send
    states:
      send:
        job:
          action: mark
        on:
          done: { to: sent }
      sent: { final: true }
  guarded:
    subject: person
    run_as: system:onboarding-bot
    per_subject: many
    start: [{ on: manual, permission: onboarding.start }]
    initial: send
    states:
      send:
        job:
          action: guarded-mark
        on:
          done: { to: sent }
      sent: { final: true }
`

// jobActions declares the actions of the act and guarded flows. The bot
// does not hold onboarding.start.
const jobActions = `actions:
  mark: { script: mark.lua }
  guarded-mark: { script: mark.lua, permission: onboarding.start }
`

const jobMarkScript = `rela.update_entity(entity.id, {stage = rela.principal.user .. " " .. rela.params.attempt})`

// hookServer records the webhook requests and answers each with status.
type hookServer struct {
	mu     sync.Mutex
	status int
	got    []*http.Request
	bodies [][]byte
}

func (h *hookServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	h.mu.Lock()
	defer h.mu.Unlock()
	h.got = append(h.got, r)
	h.bodies = append(h.bodies, body)
	w.WriteHeader(h.status)
}

func (h *hookServer) calls() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.got)
}

// jobProject builds a long-running host over the sample plus jobFlows,
// with the webhook pointed at hook.
func jobProject(t *testing.T, hook *hookServer) *appbuild.Services {
	t.Helper()
	svc, err := buildJobProject(t, hook, jobActions)
	require.NoError(t, err)
	return svc
}

// buildJobProject builds the job project with dataEntry as data-entry.yaml.
func buildJobProject(t *testing.T, hook *hookServer, dataEntry string) (*appbuild.Services, error) {
	t.Helper()
	srv := httptest.NewServer(hook)
	t.Cleanup(srv.Close)
	root := writeFlowProject(t)
	flows, err := os.ReadFile(filepath.Join(root, "flows.yaml"))
	require.NoError(t, err)
	extra := strings.Replace(jobFlows, "url: URL", "url: "+srv.URL+"/hook", 1)
	require.NoError(t, os.WriteFile(filepath.Join(root, "flows.yaml"), append(flows, extra...), 0o600))
	require.NoError(t, os.MkdirAll(filepath.Join(root, "scripts", "flows"), 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(root, "scripts", "flows", "mark.lua"), []byte(jobMarkScript), 0o600))
	require.NoError(t, os.MkdirAll(filepath.Join(root, "actions"), 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(root, "actions", "mark.lua"), []byte(jobMarkScript), 0o600))
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".rela"), 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(root, ".rela", "secrets.yaml"), []byte("hook_key: s3cret\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(root, "data-entry.yaml"), []byte(dataEntry), 0o600))
	svc, err := appbuildOnDiskWithOpts(t, root, appbuild.WithBackgroundAutomationJobs(),
		appbuild.WithHTTPClient(srv.Client()))
	if err == nil {
		t.Cleanup(func() { _ = svc.Close() })
	}
	return svc, err
}

func startJob(t *testing.T, svc *appbuild.Services, name string) (id flow.RunID, personID string) {
	t.Helper()
	ctx := context.Background()
	person := entity.New("PER-J", "person")
	person.Properties = map[string]any{"name": "Ada", "status": "new"}
	require.NoError(t, svc.Store().CreateEntity(ctx, person))
	id, err := appbuild.FlowEngine(svc).Start(ctx, name, person.Ref())
	require.NoError(t, err)
	return id, person.ID
}

func waitState(t *testing.T, svc *appbuild.Services, id flow.RunID, state string) {
	t.Helper()
	require.Eventually(t, func() bool { return header(t, svc, id)["state"] == state },
		10*time.Second, 10*time.Millisecond, "run %s never reached %s", id, state)
}

// TestFlowJob_ScriptRunsAsRunAs pins the script target end to end: the
// queue runs the attempt, the script writes as run_as and sees the
// attempt, and the run moves on.
func TestFlowJob_ScriptRunsAsRunAs(t *testing.T) {
	svc := jobProject(t, &hookServer{status: http.StatusOK})
	id, pid := startJob(t, svc, "mark")
	waitState(t, svc, id, "sent")
	p, err := svc.Store().GetEntity(context.Background(), entity.Ref{ID: pid})
	require.NoError(t, err)
	require.Equal(t, "system:onboarding-bot 1", p.Properties["stage"])
}

// TestFlowJob_WebhookSignsIDsOnly pins what a webhook sends: the ids, both
// keys, and a signature over the timestamp and body.
func TestFlowJob_WebhookSignsIDsOnly(t *testing.T) {
	hook := &hookServer{status: http.StatusNoContent}
	svc := jobProject(t, hook)
	id, pid := startJob(t, svc, "hook")
	waitState(t, svc, id, "sent")

	require.Equal(t, 1, hook.calls())
	r, body := hook.got[0], hook.bodies[0]
	var got map[string]any
	require.NoError(t, json.Unmarshal(body, &got))
	require.Equal(t, "hook", got["flow"])
	require.Equal(t, string(id), got["run"])
	require.Equal(t, pid, got["subject"])
	require.NotContains(t, string(body), "Ada", "the body carries ids, not the subject's properties")
	require.Equal(t, got["idempotency_key"], r.Header.Get("Idempotency-Key"))
	require.Equal(t, got["step_key"], r.Header.Get("X-Rela-Step"))
	require.True(t, strings.HasPrefix(r.Header.Get("Idempotency-Key"), r.Header.Get("X-Rela-Step")+"/"))

	mac := hmac.New(sha256.New, []byte("s3cret"))
	mac.Write([]byte(r.Header.Get("X-Rela-Timestamp") + "."))
	mac.Write(body)
	require.Equal(t, "sha256="+hex.EncodeToString(mac.Sum(nil)), r.Header.Get("X-Rela-Signature"))
}

// TestFlowJob_ActionRunsAsRunAs pins the action target: a plain script
// action runs as run_as, and one whose permission run_as lacks fails the
// job at once.
func TestFlowJob_ActionRunsAsRunAs(t *testing.T) {
	svc := jobProject(t, &hookServer{status: http.StatusOK})
	id, pid := startJob(t, svc, "act")
	waitState(t, svc, id, "sent")
	p, err := svc.Store().GetEntity(context.Background(), entity.Ref{ID: pid})
	require.NoError(t, err)
	require.Equal(t, "system:onboarding-bot 1", p.Properties["stage"])

	ctx := context.Background()
	other := entity.New("PER-K", "person")
	other.Properties = map[string]any{"name": "Bob", "status": "new"}
	require.NoError(t, svc.Store().CreateEntity(ctx, other))
	gid, err := appbuild.FlowEngine(svc).Start(ctx, "guarded", other.Ref())
	require.NoError(t, err)
	run := waitJobFailed(t, svc, gid)
	require.Equal(t, flowrun.JobFailedPermanent, run.Pending.Job.FailKind)
	o, err := svc.Store().GetEntity(ctx, other.Ref())
	require.NoError(t, err)
	require.Nil(t, o.Properties["stage"], "the refused action did not run")
}

// TestFlowJob_UnknownActionRefusesTheBuild pins the load check: a job step
// that names an action data-entry.yaml lacks, or one a job cannot run,
// stops the build.
func TestFlowJob_UnknownActionRefusesTheBuild(t *testing.T) {
	for name, dataEntry := range map[string]string{
		"missing": "actions:\n  guarded-mark: { script: mark.lua }\n",
		"set":     "actions:\n  mark: { set: { stage: x } }\n  guarded-mark: { script: mark.lua }\n",
	} {
		t.Run(name, func(t *testing.T) {
			_, err := buildJobProject(t, &hookServer{status: http.StatusOK}, dataEntry)
			require.ErrorContains(t, err, `"mark"`)
		})
	}
}

func waitJobFailed(t *testing.T, svc *appbuild.Services, id flow.RunID) flowrun.Run {
	t.Helper()
	eng := appbuild.FlowEngine(svc)
	var run flowrun.Run
	require.Eventually(t, func() bool {
		var err error
		run, err = eng.Run(context.Background(), id)
		require.NoError(t, err)
		return run.Pending != nil && run.Pending.Job != nil && run.Pending.Job.Phase == flowrun.JobFailed
	}, 10*time.Second, 10*time.Millisecond)
	return run
}

// TestFlowJob_WebhookClientErrorFails pins that a 4xx fails the job at
// once and leaves the run in its state.
func TestFlowJob_WebhookClientErrorFails(t *testing.T) {
	hook := &hookServer{status: http.StatusBadRequest}
	svc := jobProject(t, hook)
	id, _ := startJob(t, svc, "hook")
	run := waitJobFailed(t, svc, id)
	require.Equal(t, flowrun.JobFailedPermanent, run.Pending.Job.FailKind)
	require.Equal(t, 1, hook.calls(), "a permanent error is not tried again")
	require.Equal(t, "send", header(t, svc, id)["state"])
}
