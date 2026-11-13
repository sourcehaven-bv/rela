package appbuild

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/config"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/flowcfg"
	"github.com/Sourcehaven-BV/rela/internal/flowengine"
	"github.com/Sourcehaven-BV/rela/internal/jobs"
	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/script"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// Flow job steps (TKT-PY2G11W): the queue kind that runs one attempt, and
// the runner that runs its target as the flow's run_as.

// flowJobKind runs one attempt of a flow job step.
var flowJobKind = jobs.NewKind("flow", "job-attempt")

// maxWebhookResponse bounds what a webhook response is read for.
const maxWebhookResponse = 64 << 10

// flowJobQueue puts attempts on the job queue. The engine owns the retries,
// so each attempt is one job that never retries.
type flowJobQueue struct{ q jobs.Enqueuer }

// EnqueueAttempt implements [flowengine.JobQueue]. The key collapses a
// second enqueue of the same attempt; the deadline drops an attempt the
// queue cannot start in time.
func (f flowJobQueue) EnqueueAttempt(ctx context.Context, ref flowengine.AttemptRef, key string,
	deadline time.Time) error {
	payload, err := attemptPayload(ref)
	if err != nil {
		return err
	}
	return f.q.Enqueue(ctx, jobs.Job{
		Kind: flowJobKind, Payload: payload, Retry: jobs.RetryNever, IdempotencyKey: key, Deadline: deadline,
	})
}

// attemptPayload is ref as a JSON object, the shape every queue keeps.
func attemptPayload(ref flowengine.AttemptRef) (map[string]any, error) {
	data, err := json.Marshal(ref)
	if err != nil {
		return nil, err
	}
	var m map[string]any
	return m, json.Unmarshal(data, &m)
}

// attemptRef reads the payload back. It only names an attempt: the engine
// acts only when the stored attempt still matches it.
func attemptRef(payload map[string]any) (flowengine.AttemptRef, error) {
	var ref flowengine.AttemptRef
	data, err := json.Marshal(payload)
	if err != nil {
		return ref, err
	}
	if err := json.Unmarshal(data, &ref); err != nil {
		return ref, fmt.Errorf("flows: job payload: %w", err)
	}
	return ref, nil
}

// registerFlowJobs registers the attempt handler on q.
func registerFlowJobs(q jobs.Registrar, eng *flowengine.Engine) error {
	h, err := flowengine.NewAttemptHandler(eng)
	if err != nil {
		return err
	}
	err = q.Register(flowJobKind, func(ctx context.Context, job jobs.Job) error {
		ref, rerr := attemptRef(job.Payload)
		if rerr != nil {
			return rerr
		}
		return h.HandleAttempt(ctx, ref)
	})
	if err != nil {
		return fmt.Errorf("flows: register job steps: %w", err)
	}
	return nil
}

// flowJobServices is what the runner needs from the assembled services.
type flowJobServices interface {
	ScheduledLuaWriteDeps() lua.WriteDeps
}

// flowJobRunner runs the target of one attempt as the flow's run_as.
type flowJobRunner struct {
	svc     flowJobServices
	scripts *script.Engine
	cfg     config.Loader
	host    lua.HostConfig
	acl     *acl.Declarative
	as      asIdentity
	client  *http.Client
}

// newFlowJobRunner returns the runner over s. Webhooks post through the
// services' HTTP seam when one is set.
func newFlowJobRunner(s *Services, as asIdentity) *flowJobRunner {
	client := &http.Client{}
	if c := s.base.opts.seams.http; c != nil {
		cp := *c
		client = &cp
	}
	// A redirect would carry the signed body to a URL flows.yaml does not
	// name.
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &flowJobRunner{
		svc: s, scripts: s.scriptEngine, cfg: s.cfgLoader, host: s.host, acl: s.aclDeclarative, as: as,
		client: client,
	}
}

// RunJob implements [flowengine.JobRunner].
func (r *flowJobRunner) RunJob(ctx context.Context, call flowengine.JobCall) error {
	if call.RunAs == "" || principal.IsRelaIdentity(call.RunAs) {
		return flowengine.Permanent(fmt.Errorf("flows: run_as %q may not run a job", call.RunAs))
	}
	ctx, err := r.as(ctx, call.RunAs)
	if err != nil {
		return fmt.Errorf("flows: identity %q: %w", call.RunAs, err)
	}
	switch call.Job.Target {
	case flowcfg.JobScript:
		return r.runScript(ctx, call)
	case flowcfg.JobAction:
		return r.runAction(ctx, call)
	case flowcfg.JobWebhook:
		return r.post(ctx, call)
	default:
		return flowengine.Permanent(fmt.Errorf("flows: unknown job target %q", call.Job.Target))
	}
}

// jobParams are the attempt's keys as rela.params.
func jobParams(call flowengine.JobCall) map[string]string {
	return map[string]string{
		"flow": call.Flow, "flow_run": string(call.Run), "flow_state": call.State,
		"attempt": strconv.Itoa(call.Attempt), "idempotency_key": call.Key, "step_key": call.StepKey,
	}
}

// luaTimeout is the script timeout: past the attempt's deadline, so the
// deadline ends the script and the engine reads it as a timeout.
func luaTimeout(ctx context.Context) time.Duration {
	if d, ok := ctx.Deadline(); ok {
		return time.Until(d) + time.Second
	}
	return lua.DefaultTimeout
}

// subject reads the run's subject as run_as. A subject run_as cannot read
// fails the job: another attempt reads the same.
func subject(ctx context.Context, reader lua.EntityReader, ref entity.Ref) (*entity.Entity, error) {
	e, err := reader.GetAddress(ctx, ref.String())
	if errors.Is(err, store.ErrNotFound) || errors.Is(err, entitymanager.ErrEntityNotFound) {
		return nil, flowengine.Permanent(fmt.Errorf("flows: the subject %s is not readable as run_as", ref))
	}
	return e, err
}

// scriptOutcome marks a script error raised with `retry = false` permanent.
func scriptOutcome(err error) error {
	var se *lua.ScriptError
	if errors.As(err, &se) && se.Permanent {
		return flowengine.Permanent(err)
	}
	return err
}

func (r *flowJobRunner) runScript(ctx context.Context, call flowengine.JobCall) error {
	deps := r.svc.ScheduledLuaWriteDeps()
	caps := luaCaps(call.Job.Capabilities)
	deps.Capabilities = caps
	e, err := subject(ctx, deps.VisibleReader, call.Subject)
	if err != nil {
		return err
	}
	return scriptOutcome(r.scripts.ExecuteJob(ctx, call.Job.Script, deps, script.JobInvocation{
		Subject: e, Params: jobParams(call), Capabilities: caps, Timeout: luaTimeout(ctx),
	}))
}

// runAction runs the action as data-entry.yaml declares it now: a reload
// that removed or changed the action applies to the next attempt.
func (r *flowJobRunner) runAction(ctx context.Context, call flowengine.JobCall) error {
	actions, err := loadActions(ctx, r.cfg)
	if err != nil {
		return err
	}
	a, ok := actions[call.Job.Action]
	if !ok {
		return flowengine.Permanent(fmt.Errorf("flows: data-entry.yaml declares no action %q", call.Job.Action))
	}
	if uerr := flowActionUsable(call.Job.Action, a); uerr != nil {
		return flowengine.Permanent(uerr)
	}
	if !r.holdsPermission(ctx, a.Permission) {
		return flowengine.Permanent(fmt.Errorf("flows: run_as %q lacks the %q permission of action %q",
			call.RunAs, a.Permission, call.Job.Action))
	}
	deps := r.svc.ScheduledLuaWriteDeps()
	deps.Capabilities = luaCaps(a.Capabilities)
	e, err := subject(ctx, deps.VisibleReader, call.Subject)
	if err != nil {
		return err
	}
	params := maps.Clone(a.Params)
	if params == nil {
		params = map[string]string{}
	}
	maps.Copy(params, jobParams(call))
	_, err = r.scripts.ExecuteActionRequest(ctx, a.Script, deps, script.ActionInvocation{
		TriggerEntity: e, Params: params, Timeout: luaTimeout(ctx),
	})
	return scriptOutcome(err)
}

// holdsPermission answers an action's `permission:` as the action route
// does: no policy holds every permission.
func (r *flowJobRunner) holdsPermission(ctx context.Context, perm string) bool {
	if perm == "" || r.acl == nil {
		return true
	}
	req, err := r.acl.RequestFrom(ctx)
	return err == nil && req.HoldsPermission(ctx, perm)
}

// webhookBody is what a webhook posts: ids only. The receiver reads the
// subject through the API with its own credentials.
type webhookBody struct {
	Flow           string `json:"flow"`
	Run            string `json:"run"`
	State          string `json:"state"`
	Subject        string `json:"subject"`
	Attempt        int    `json:"attempt"`
	IdempotencyKey string `json:"idempotency_key"`
	StepKey        string `json:"step_key"`
}

// post sends the webhook. 2xx succeeds; 429, 5xx and a network error may
// succeed on another attempt; any other status is permanent.
func (r *flowJobRunner) post(ctx context.Context, call flowengine.JobCall) error {
	body, err := json.Marshal(webhookBody{
		Flow: call.Flow, Run: string(call.Run), State: call.State, Subject: call.Subject.String(),
		Attempt: call.Attempt, IdempotencyKey: call.Key, StepKey: call.StepKey,
	})
	if err != nil {
		return flowengine.Permanent(err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, call.Job.URL, bytes.NewReader(body))
	if err != nil {
		return flowengine.Permanent(fmt.Errorf("flows: webhook request: %w", err))
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", call.Key)
	req.Header.Set("X-Rela-Step", call.StepKey)
	if call.Job.Secret != "" {
		key, serr := r.secret(call.Job.Secret)
		if serr != nil {
			return flowengine.Permanent(serr)
		}
		ts := strconv.FormatInt(time.Now().Unix(), 10)
		req.Header.Set("X-Rela-Timestamp", ts)
		req.Header.Set("X-Rela-Signature", "sha256="+webhookSignature(key, ts, body))
	}
	resp, err := r.client.Do(req)
	if err != nil {
		// The URL error quotes the whole URL, query included, and a query
		// may carry a token: name the host only.
		var ue *url.Error
		if errors.As(err, &ue) {
			err = ue.Err
		}
		return fmt.Errorf("flows: webhook to %s: %w", req.URL.Host, err)
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, maxWebhookResponse))
	_ = resp.Body.Close()
	return webhookOutcome(resp.StatusCode)
}

// webhookOutcome classifies a webhook response status.
func webhookOutcome(code int) error {
	switch {
	case code >= 200 && code < 300:
		return nil
	case code == http.StatusTooManyRequests || code >= 500:
		return fmt.Errorf("flows: webhook answered %d", code)
	default:
		return flowengine.Permanent(fmt.Errorf("flows: webhook answered %d", code))
	}
}

// webhookSignature is the hex HMAC-SHA256 of "<ts>.<body>" under key. The
// timestamp is signed so a receiver can refuse a replay.
func webhookSignature(key, ts string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(key))
	mac.Write([]byte(ts))
	mac.Write([]byte("."))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

// secret reads the global secret name.
func (r *flowJobRunner) secret(name string) (string, error) {
	if r.host == nil {
		return "", fmt.Errorf("flows: no secrets for webhook secret %q", name)
	}
	all, err := r.host.Secrets("")
	if err != nil {
		return "", fmt.Errorf("flows: webhook secret %q: %w", name, err)
	}
	v, ok := all[name]
	if !ok || v == "" {
		return "", fmt.Errorf("flows: webhook secret %q is not set", name)
	}
	return v, nil
}

// luaCaps is a `capabilities:` block as the runtime's grant. AllSecrets is
// never set: it is the operator shell's grant.
func luaCaps(c metamodel.Capabilities) lua.Capabilities {
	http, ai, mail, writeFile, secrets, tokens := c.Fields()
	return lua.Capabilities{HTTP: http, AI: ai, Mail: mail, WriteFile: writeFile, Secrets: secrets, Tokens: tokens}
}

// loadActions reads the actions of data-entry.yaml. An absent file
// declares none.
func loadActions(ctx context.Context, cfg config.Loader) (map[string]dataentryconfig.Action, error) {
	data, err := cfg.Load(ctx, dataentryconfig.ConfigFile)
	if errors.Is(err, fs.ErrNotExist) || os.IsNotExist(err) {
		return map[string]dataentryconfig.Action{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("flows: read %s: %w", dataentryconfig.ConfigFile, err)
	}
	var doc struct {
		Actions map[string]dataentryconfig.Action `yaml:"actions"`
	}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("flows: parse %s: %w", dataentryconfig.ConfigFile, err)
	}
	return doc.Actions, nil
}

// flowActionUsable refuses an action a job step cannot run: one that is not
// a plain script, or one whose meaning depends on an HTTP request or a
// detail page.
func flowActionUsable(id string, a dataentryconfig.Action) error {
	switch {
	case a.Script == "":
		return fmt.Errorf("flows: action %q has no script", id)
	case len(a.Set) > 0:
		return fmt.Errorf("flows: action %q sets properties; a job step runs only script actions", id)
	case a.Request != nil:
		return fmt.Errorf("flows: action %q reads an HTTP request, which a job step does not have", id)
	case a.AvailableOn != nil:
		return fmt.Errorf("flows: action %q is bound to a detail page", id)
	case a.AllowACLBypass.Enabled() || len(a.ElevatedUpdates) > 0:
		return fmt.Errorf("flows: action %q bypasses the ACL, which a job step may not", id)
	}
	return nil
}

// checkFlowActions refuses flows whose job steps name an action
// data-entry.yaml does not declare, or one a job step cannot run.
func checkFlowActions(ctx context.Context, flows *flowcfg.Compiled, cfg config.Loader) error {
	var actions map[string]dataentryconfig.Action
	loaded := false
	for _, f := range flows.Flows() {
		for _, st := range f.States() {
			job, ok := st.Job()
			if !ok || job.Target != flowcfg.JobAction {
				continue
			}
			if !loaded {
				var err error
				if actions, err = loadActions(ctx, cfg); err != nil {
					return err
				}
				loaded = true
			}
			a, ok := actions[job.Action]
			if !ok {
				return fmt.Errorf("flow %q state %q: data-entry.yaml declares no action %q", f.Name(), st.Name,
					job.Action)
			}
			if err := flowActionUsable(job.Action, a); err != nil {
				return fmt.Errorf("flow %q state %q: %w", f.Name(), st.Name, err)
			}
		}
	}
	return nil
}
