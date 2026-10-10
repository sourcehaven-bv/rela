package appbuild

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"iter"
	"log/slog"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/autocascade"
	"github.com/Sourcehaven-BV/rela/internal/automation"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/jobs"
	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/state"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// automationJobKind runs one `background: true` automation action
// (TKT-2Q4UFI).
var automationJobKind = jobs.NewKind("automation", "run-lua")

const (
	// maxAutomationJobHops bounds a chain of background jobs whose writes
	// trigger further background jobs (A → B → A). Every job starts a new
	// cascade, so the cascade's own MaxDepth does not bound it.
	maxAutomationJobHops = 8

	// maxAutomationJobRuns bounds the reruns one job makes for saves that
	// arrive while it runs.
	maxAutomationJobRuns = 5
)

// automationJobs schedules and runs background automation actions.
//
// # Delivery
//
// With a queue (rela-server, desktop) a trigger enqueues a job, and the
// queue's idempotency key coalesces the saves made while it is pending.
// Without one (every other assembly, such as a CLI command) the action runs
// in the foreground right after the cascade, once per save: a one-shot
// process would exit before its queue ran the job.
//
// # Saves during a run
//
// The idempotency key also collapses against a RUNNING job, and against one
// that has returned but whose completion the queue has not recorded yet. So
// each trigger writes a fresh token to state.KV before it enqueues, a run
// repeats while the token it started with has changed, and a run records the
// token it handled. A trigger whose enqueue collapsed retries the enqueue a
// few times until a run has handled its token or a new job is queued. On
// postgres the KV is shared, so a token written by another node is seen too.
//
// # Authority
//
// The payload only selects a configured action. Identity, capabilities and
// the script come from the live metamodel, so a stale or redelivered job
// cannot run anything the configuration no longer declares. Every run starts
// from a fresh context: nothing of the saver's request reaches the job except
// the saver's name, which the audit label records as data.
type automationJobs struct {
	meta  *metamodel.Metamodel
	kv    state.KV
	raw   rawEntityLister
	queue jobs.Client // nil: foreground delivery

	locksMu sync.Mutex
	locks   map[string]*keyLock

	// tokenMu serializes recordTrigger's read and write of a token. It is
	// process-local: two nodes writing one key can still lose a hop count.
	tokenMu sync.Mutex

	// followUpDelay spaces the enqueue retries after a collapsed enqueue.
	followUpDelay time.Duration

	// runner and engine are set by bind once the services exist: the
	// handler needs the scheduled Lua deps, which are built after the entity
	// manager that triggers it.
	runner automationJobRunner
	engine *automation.Engine
}

// automationJobRunner is what a job needs from the assembled services.
type automationJobRunner interface {
	ScheduledLuaWriteDeps() lua.WriteDeps
	ExecuteFile(ctx context.Context, path string, deps lua.WriteDeps, e *entity.Entity) error
	// BindIdentity attaches one ACL request for the ctx principal, so the
	// job's reads, writes and transition guards all decide as that identity.
	BindIdentity(ctx context.Context) (context.Context, error)
}

// rawEntityLister reads every face of a renamed entity unredacted, the way
// a save's cascade sees it when it evaluates triggers.
type rawEntityLister interface {
	ListEntities(ctx context.Context, q store.EntityQuery) iter.Seq2[*entity.Entity, error]
}

type keyLock struct {
	mu   sync.Mutex
	refs int
}

const (
	followUpAttempts     = 5
	defaultFollowUpDelay = 500 * time.Millisecond
)

func newAutomationJobs(
	meta *metamodel.Metamodel, kv state.KV, raw rawEntityLister, queue jobs.Client,
) (*automationJobs, error) {
	if meta == nil || kv == nil || raw == nil {
		return nil, errors.New("appbuild: automation jobs need a metamodel, a state store and a store")
	}
	return &automationJobs{
		meta: meta, kv: kv, raw: raw, queue: queue,
		locks: map[string]*keyLock{}, followUpDelay: defaultFollowUpDelay,
	}, nil
}

// bind connects the runner and, in queue mode, registers the job kind.
// A registration failure fails assembly: without the handler every
// matching save would be refused at enqueue.
func (a *automationJobs) bind(r automationJobRunner, engine *automation.Engine) error {
	a.runner = r
	a.engine = engine
	if a.queue == nil || !hasBackgroundActions(a.meta) {
		return nil
	}
	if err := a.queue.Register(automationJobKind, a.handle); err != nil {
		return fmt.Errorf("appbuild: register automation jobs: %w", err)
	}
	return nil
}

// automationJobPayload is the job's payload. Every field selects or
// records; none grants.
type automationJobPayload struct {
	Automation string
	LuaFile    string
	ID         string
	Face       entity.Face
	Hops       int
	// By is the user whose save triggered the job, for the audit label.
	By string
}

func (p automationJobPayload) key() string {
	return fmt.Sprintf("automation:%s:%s:%s", p.Automation, p.LuaFile, entity.FormatStateRef(p.ID, p.Face))
}

func (p automationJobPayload) toMap() map[string]any {
	return map[string]any{
		"automation": p.Automation, "lua_file": p.LuaFile,
		"id": p.ID, "face": string(p.Face), "hops": p.Hops, "by": p.By,
	}
}

func payloadFromMap(m map[string]any) (automationJobPayload, error) {
	str := func(k string) string { v, _ := m[k].(string); return v }
	p := automationJobPayload{
		Automation: str("automation"), LuaFile: str("lua_file"),
		ID: str("id"), Face: entity.Face(str("face")), By: str("by"),
	}
	// A JSON round-trip turns the int into a float64.
	switch h := m["hops"].(type) {
	case int:
		p.Hops = h
	case float64:
		p.Hops = int(h)
	}
	if p.Automation == "" || p.LuaFile == "" || p.ID == "" {
		return p, errors.New("automation job: incomplete payload")
	}
	return p, nil
}

// runningJobsKey marks a ctx as running the jobs with those keys, so their
// own writes, and the writes of jobs they trigger, do not schedule them
// again. The value is a []string, copied on every addition.
type runningJobsKey struct{}

// jobHopsKey carries the hop count of the running job into the triggers its
// writes cause.
type jobHopsKey struct{}

func runningJobs(ctx context.Context) []string {
	keys, _ := ctx.Value(runningJobsKey{}).([]string)
	return keys
}

// jobContext is the context a job starts from: base, plus the loop guards
// carried over from the ctx that triggered it, and nothing else.
func jobContext(base, from context.Context) context.Context {
	if keys := runningJobs(from); keys != nil {
		base = context.WithValue(base, runningJobsKey{}, keys)
	}
	if hops, ok := from.Value(jobHopsKey{}).(int); ok {
		base = context.WithValue(base, jobHopsKey{}, hops)
	}
	return base
}

// EnqueueScript implements autocascade.BackgroundScripts.
func (a *automationJobs) EnqueueScript(ctx context.Context, s autocascade.BackgroundScript) error {
	p := automationJobPayload{
		Automation: s.Automation, LuaFile: s.LuaFile, ID: s.Ref.ID, Face: s.Ref.Face,
		By: principal.From(ctx).User,
	}
	if slices.Contains(runningJobs(ctx), p.key()) {
		return nil // a write by this job, or by a job it triggered
	}
	hops, _ := ctx.Value(jobHopsKey{}).(int)
	p.Hops = hops + 1
	if p.Hops > maxAutomationJobHops {
		return fmt.Errorf("automation %q: background jobs triggered each other %d times; not scheduling %s again",
			s.Automation, maxAutomationJobHops, s.LuaFile)
	}
	act, ok := findBackgroundAction(a.meta, p.Automation, p.LuaFile)
	if !ok {
		return fmt.Errorf("automation %q: no background action for %s", p.Automation, p.LuaFile)
	}
	slog.Debug("automation background action triggered", "automation", p.Automation,
		"lua_file", p.LuaFile, "entity", p.ID, "by", p.By)
	if a.queue == nil {
		return a.runForeground(ctx, p)
	}
	token, err := a.recordTrigger(ctx, p.key(), p.Hops)
	if err != nil {
		return fmt.Errorf("automation %q: record trigger: %w", p.Automation, err)
	}
	err = a.enqueue(ctx, p, act)
	if errors.Is(err, jobs.ErrDuplicateJob) {
		// The follow-up outlives the save, and the save's ctx may carry a
		// transaction's job deferral that is flushed by then.
		go a.followUp(p, act, string(token)) //nolint:gosec,contextcheck // see above
		return nil
	}
	return err
}

func (a *automationJobs) enqueue(ctx context.Context, p automationJobPayload, act metamodel.AutomationAction) error {
	return a.queue.Enqueue(ctx, jobs.Job{
		Kind:           automationJobKind,
		Payload:        p.toMap(),
		Retry:          jobRetry(act.Retry),
		IdempotencyKey: p.key(),
	})
}

// runForeground runs the job now, on a fresh context. The saver learns only
// that the action failed; the script's error, which may name what the job's
// identity can read, goes to the log.
func (a *automationJobs) runForeground(ctx context.Context, p automationJobPayload) error {
	if err := a.run(jobContext(context.Background(), ctx), p); err != nil {
		slog.Warn("automation background action failed",
			"automation", p.Automation, "lua_file", p.LuaFile, "entity", p.ID, "error", err)
		return fmt.Errorf("automation %q: background action failed", p.Automation)
	}
	return nil
}

// followUp covers a save whose enqueue collapsed into a job that may already
// have made its last token check. It re-enqueues until a run has handled
// the token, a later save has taken over, or a new job is queued. A job
// that is still pending or running sees the token itself.
func (a *automationJobs) followUp(p automationJobPayload, act metamodel.AutomationAction, token string) {
	ctx := context.Background()
	key := p.key()
	for i := range followUpAttempts {
		time.Sleep(a.followUpDelay * time.Duration(i+1))
		current, readErr := a.token(ctx, key)
		if readErr != nil || current != token {
			return // unreadable, or a later save follows up
		}
		if handled, hErr := a.handled(ctx, key); hErr != nil || handled == token {
			return
		}
		err := a.enqueue(ctx, p, act)
		if !errors.Is(err, jobs.ErrDuplicateJob) {
			if err != nil {
				slog.Warn("automation background action not scheduled",
					"automation", p.Automation, "entity", p.ID, "error", err)
			}
			return
		}
	}
}

// EntityRenamed schedules, for the new id, the background actions whose
// triggers match the renamed entity: a pending job names the old id and
// would find nothing. It reads every face raw, as a save's cascade does, so
// the trigger decides on the same values whoever renamed it. A failure is
// logged here and never returned: the rename stands, and the hook that
// calls this reports its errors as alias-rewrite failures.
func (a *automationJobs) EntityRenamed(ctx context.Context, _, newID string) error {
	if a.engine == nil || !hasBackgroundActions(a.meta) {
		return nil
	}
	q := store.EntityQuery{IDs: []string{newID}, Faces: store.AllFaces()}
	for e, err := range a.raw.ListEntities(ctx, q) {
		if err != nil {
			slog.Warn("automation background actions not scheduled after rename", "entity", newID, "error", err)
			return nil
		}
		res := a.engine.Process(ctx, automation.Event{Type: automation.EventEntityRenamed, Entity: e})
		for _, l := range res.LuaToExecute {
			if !l.Background {
				continue
			}
			err := a.EnqueueScript(ctx, autocascade.BackgroundScript{
				Automation: l.AutomationName, LuaFile: l.FilePath, Ref: e.Ref(),
			})
			if err != nil {
				slog.Warn("automation background action failed after rename",
					"automation", l.AutomationName, "entity", newID, "error", err)
			}
		}
	}
	return nil
}

// EntityDeleted implements entitymanager.AliasRewriter; a job for a deleted
// entity finds nothing and ends.
func (a *automationJobs) EntityDeleted(context.Context, string) error { return nil }

// EntityFaceDeleted implements entitymanager.AliasRewriter.
func (a *automationJobs) EntityFaceDeleted(context.Context, string, entity.Face) error { return nil }

// handle is the queue handler. The queue's ctx carries no request.
func (a *automationJobs) handle(ctx context.Context, j jobs.Job) error {
	p, err := payloadFromMap(j.Payload)
	if err != nil {
		slog.Warn("automation job dropped", "error", err)
		return nil
	}
	return a.run(ctx, p)
}

// run executes the action. In queue mode it repeats while saves arrive
// during the run. A configuration problem ends the job without an error,
// since a retry would meet it again; a script error is returned, so the
// queue retries.
func (a *automationJobs) run(ctx context.Context, p automationJobPayload) error {
	if a.runner == nil {
		return errors.New("automation job: services not bound")
	}
	act, ok := findBackgroundAction(a.meta, p.Automation, p.LuaFile)
	if !ok {
		slog.Warn("automation job dropped: the action is no longer configured",
			"automation", p.Automation, "lua_file", p.LuaFile, "entity", p.ID)
		return nil
	}
	key := p.key()
	if a.queue != nil {
		// Foreground runs need no lock: each save runs its own job on its
		// own goroutine, and a nested run of the same key is suppressed.
		defer a.lock(key)()
	}

	user := act.RunAs
	if user == "" {
		user = principal.UserAutomation
	}
	label := "automation-job:" + p.Automation
	if p.By != "" {
		label += ";by=" + p.By
	}
	ctx = principal.With(ctx, principal.Principal{User: user, Tool: principal.ToolAutomationJob})
	ctx = audit.WithTriggeredBy(ctx, label)
	ctx = context.WithValue(ctx, runningJobsKey{}, append(slices.Clone(runningJobs(ctx)), key))
	// Queue mode overrides this per run with the hop count of the trigger
	// it handles; a foreground run handles only its own trigger.
	ctx = context.WithValue(ctx, jobHopsKey{}, p.Hops)
	ctx, err := a.runner.BindIdentity(ctx)
	if err != nil {
		return fmt.Errorf("automation %q: identity %q: %w", p.Automation, user, err)
	}

	if a.queue == nil {
		return a.runOnce(ctx, p, act, user)
	}
	for range maxAutomationJobRuns {
		before, err := a.token(ctx, key)
		if err != nil {
			return err
		}
		handled, err := a.handled(ctx, key)
		if err != nil {
			return err
		}
		if before != "" && handled == before {
			return nil // an earlier run handled this save
		}
		// A save that collapsed into this job may sit further along a
		// chain than the save that queued it. Run with its hop count, or
		// jobs that trigger each other while running would reset the
		// count and outlast maxAutomationJobHops (BUG-WKL0M2).
		p.Hops = max(p.Hops, tokenHops(before))
		runCtx := context.WithValue(ctx, jobHopsKey{}, p.Hops)
		if runErr := a.runOnce(runCtx, p, act, user); runErr != nil {
			return runErr
		}
		if err = a.kv.Put(ctx, handledKey(key), []byte(before)); err != nil {
			return fmt.Errorf("automation job: record handled trigger: %w", err)
		}
		after, err := a.token(ctx, key)
		if err != nil {
			return err
		}
		if after == before {
			return nil
		}
	}
	// Returning an error lets the retry policy run the job again for the
	// save it has not handled.
	return fmt.Errorf("automation %q: entity %s kept changing; stopped after %d runs",
		p.Automation, p.ID, maxAutomationJobRuns)
}

// runOnce reads the entity as the job's identity and runs the script on it.
func (a *automationJobs) runOnce(
	ctx context.Context, p automationJobPayload, act metamodel.AutomationAction, user string,
) error {
	deps := a.runner.ScheduledLuaWriteDeps()
	http, ai, mail, writeFile, secrets := act.Capabilities.Fields()
	deps.Capabilities = lua.Capabilities{HTTP: http, AI: ai, Mail: mail, WriteFile: writeFile, Secrets: secrets}
	e, err := deps.VisibleReader.GetAddress(ctx, entity.FormatStateRef(p.ID, p.Face))
	if err != nil {
		slog.Warn("automation job dropped: entity not found, or this identity cannot read it",
			"automation", p.Automation, "entity", p.ID, "principal", user, "error", err)
		return nil
	}
	if err := a.runner.ExecuteFile(ctx, p.LuaFile, deps, e); err != nil {
		return fmt.Errorf("automation %q: %s: %w", p.Automation, p.LuaFile, err)
	}
	return nil
}

// lock takes the per-key lock and returns its release. An entry lives only
// while someone holds or waits for it.
func (a *automationJobs) lock(key string) func() {
	a.locksMu.Lock()
	l, ok := a.locks[key]
	if !ok {
		l = &keyLock{}
		a.locks[key] = l
	}
	l.refs++
	a.locksMu.Unlock()
	l.mu.Lock()
	return func() {
		l.mu.Unlock()
		a.locksMu.Lock()
		l.refs--
		if l.refs == 0 {
			delete(a.locks, key)
		}
		a.locksMu.Unlock()
	}
}

func (a *automationJobs) handled(ctx context.Context, key string) (string, error) {
	return a.read(ctx, handledKey(key))
}

func (a *automationJobs) token(ctx context.Context, key string) (string, error) {
	return a.read(ctx, tokenKey(key))
}

func (a *automationJobs) read(ctx context.Context, k string) (string, error) {
	b, err := a.kv.Get(ctx, k)
	if err != nil {
		if os.IsNotExist(err) || errors.Is(err, fs.ErrNotExist) {
			return "", nil
		}
		return "", fmt.Errorf("automation job: read %s: %w", k, err)
	}
	return string(b), nil
}

// tokenKey hashes the job key, which holds operator- and user-chosen text,
// into a fixed-shape state key.
func tokenKey(jobKey string) string {
	sum := sha256.Sum256([]byte(jobKey))
	return "automation-jobs/" + hex.EncodeToString(sum[:])
}

// handledKey records the token of the last save a run handled.
func handledKey(jobKey string) string { return tokenKey(jobKey) + "-handled" }

// recordTrigger writes a new token for key and returns it. While the current
// token is unhandled, the new one keeps the higher hop count: otherwise an
// unrelated save between two reruns would lower a chain's count
// (BUG-WKL0M2). A fresh save may therefore inherit a chain's count, which
// errs toward stopping early.
func (a *automationJobs) recordTrigger(ctx context.Context, key string, hops int) ([]byte, error) {
	a.tokenMu.Lock()
	defer a.tokenMu.Unlock()
	current, err := a.token(ctx, key)
	if err != nil {
		return nil, err
	}
	if current != "" {
		handled, hErr := a.handled(ctx, key)
		if hErr != nil {
			return nil, hErr
		}
		if handled != current {
			hops = max(hops, tokenHops(current))
		}
	}
	token := newToken(hops)
	if err := a.kv.Put(ctx, tokenKey(key), token); err != nil {
		return nil, err
	}
	return token, nil
}

// newToken returns a fresh trigger token that ends in the trigger's hop
// count, which [tokenHops] reads back.
func newToken(hops int) []byte {
	b := make([]byte, 16)
	_, _ = rand.Read(b) // never fails (crypto/rand)
	return fmt.Appendf(nil, "%s/%d", hex.EncodeToString(b), hops)
}

// tokenHops returns the hop count in a token from [newToken]. A token
// without one, written before tokens carried it, reads as 0, so the job's
// own payload count applies.
func tokenHops(token string) int {
	_, hops, ok := strings.Cut(token, "/")
	if !ok {
		return 0
	}
	n, err := strconv.Atoi(hops)
	if err != nil {
		return 0
	}
	return n
}

func jobRetry(r metamodel.JobRetry) jobs.Retry {
	switch r {
	case metamodel.JobRetryNever:
		return jobs.RetryNever
	case metamodel.JobRetryPersistent:
		return jobs.RetryPersistent
	default:
		return jobs.RetryBounded
	}
}

// findBackgroundAction resolves a job to its configured action. Load
// validation makes (automation name, lua_file) unique among background
// actions, so at most one matches.
func findBackgroundAction(meta *metamodel.Metamodel, name, file string) (metamodel.AutomationAction, bool) {
	for _, auto := range meta.Automations {
		if auto.Name != name {
			continue
		}
		for _, act := range auto.Do {
			if act.Background && act.LuaFile == file {
				return act, true
			}
		}
	}
	return metamodel.AutomationAction{}, false
}

func hasBackgroundActions(meta *metamodel.Metamodel) bool {
	for _, auto := range meta.Automations {
		for _, act := range auto.Do {
			if act.Background {
				return true
			}
		}
	}
	return false
}

// servicesJobRunner adapts the assembled services to automationJobRunner,
// keeping the two methods off Services, which is at its plimsoll cap.
type servicesJobRunner struct{ s *Services }

func (r servicesJobRunner) ScheduledLuaWriteDeps() lua.WriteDeps { return r.s.ScheduledLuaWriteDeps() }

func (r servicesJobRunner) BindIdentity(ctx context.Context) (context.Context, error) {
	if r.s.aclDeclarative == nil {
		return ctx, nil
	}
	req, err := r.s.aclDeclarative.ForPrincipal(principal.From(ctx))
	if err != nil {
		return nil, err
	}
	return acl.WithRequest(ctx, req), nil
}

func (r servicesJobRunner) ExecuteFile(ctx context.Context, path string, deps lua.WriteDeps, e *entity.Entity) error {
	return r.s.scriptEngine.ExecuteFile(ctx, path, deps, e, nil)
}
