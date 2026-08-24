// Package scheduler runs Lua scripts on recurring schedules defined in
// schedules.yaml.
//
// The scheduler is a single-threaded sequential loop: tasks execute one at
// a time in config order. Each task gets a fresh ws.LuaWriteDeps() from the
// workspace; the store is the source of truth, so no explicit sync is
// needed. Last-run timestamps are persisted in .rela/scheduler-state.json.
// Tasks that missed their scheduled window run immediately on startup.
// Shutdown is graceful on SIGINT/SIGTERM.
//
// Schedule values in schedules.yaml:
//
//	day          once per day (after midnight local time)
//	<weekday>    once per week on that weekday (monday, friday, ...)
//	week         alias for monday
//	30m, 2h      fixed interval (any Go duration)
//	15           bare number interpreted as minutes
//
// A task that fails enters a backoff ladder (5m, 10m, 20m, 40m, 80m, then
// every 2h) which REPLACES its schedule until it succeeds — while a retry is
// pending the task fires only on ladder steps, never on its normal cadence.
// The ladder is identical for every schedule, so it slows a failing
// short-interval task down and speeds a failing daily one up. Only a
// successful run resets it.
//
// See Config/TaskConfig for the YAML shape and Schedule.IsDue for the
// due-time logic.
package scheduler

import (
	"context"
	"errors"
	"iter"
	"log/slog"
	"maps"
	"slices"
	"sync"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/config"
	"github.com/Sourcehaven-BV/rela/internal/jobs"
	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/script"
	"github.com/Sourcehaven-BV/rela/internal/state"
)

// tickInterval is how often the scheduler wakes to check for due tasks.
const tickInterval = 60 * time.Second

// Retry ladder for failed tasks. The delay doubles per consecutive failure
// from baseRetryDelay (5m, 10m, 20m, 40m, 80m) and then holds at
// maxRetryDelay, repeating until the task succeeds.
//
// The cap keeps a persistently broken job to roughly a dozen attempts a day
// rather than silencing it, while still recovering an intermittent failure
// within minutes. It is not tied to any schedule: see retryDelay.
const (
	baseRetryDelay = 5 * time.Minute
	maxRetryDelay  = 2 * time.Hour

	// persistentFailureThreshold is the consecutive-failure count at which
	// "task failed" escalates from WARN to ERROR — by this point the retries
	// are demonstrably not helping and the job needs a human.
	persistentFailureThreshold = 4
)

// maxLadderSteps is the failure count at which doubling first reaches
// maxRetryDelay; beyond it every retry is capped. Derived rather than written
// as a literal so retuning baseRetryDelay or maxRetryDelay cannot silently
// desynchronise the bound from the ladder it describes.
var maxLadderSteps = func() int {
	steps := 1
	for d := baseRetryDelay; d < maxRetryDelay; d *= 2 {
		steps++
	}
	return steps
}()

// WorkspaceProvider is the subset of workspace.Workspace the scheduler needs.
type WorkspaceProvider interface {
	Paths() *project.Context
	Config() config.Loader
	State() state.KV

	// ScheduledLuaWriteDeps returns the per-task capability bundle. Its
	// reads are ACL-bound to whatever principal is on the ctx at call time
	// (DEC-O59WM4), which is the task's — see stampTaskAuditContext. The
	// bundle is therefore identity-agnostic and safe to rebuild per task.
	ScheduledLuaWriteDeps() lua.WriteDeps
}

// StartBackground starts the scheduler in a background goroutine if
// schedules.yaml exists. It is a no-op if the file is missing. The scheduler
// runs until ctx is cancelled. Errors are logged, not returned.
func StartBackground(
	ctx context.Context,
	ws WorkspaceProvider,
	logger *slog.Logger,
) {
	data, err := ws.Config().Load(ctx, ConfigFile)
	if err != nil {
		// No schedules.yaml — nothing to do.
		return
	}

	cfg, err := ParseConfig(data)
	if err != nil {
		logger.Error("invalid schedules.yaml, scheduler not started", "error", err)
		return
	}

	if len(cfg.Tasks) == 0 {
		return
	}

	engine := script.NewEngine()
	s := New(cfg, engine, ws, logger)

	// Route execution through the job queue when the provider carries one.
	//
	// An optional interface rather than a parameter so the two call sites
	// (rela-server, rela-desktop) keep passing the services bundle they
	// already have. A provider without a queue runs inline, which is the
	// pre-port behavior and is what the scheduling tests exercise.
	if jp, ok := ws.(jobQueueProvider); ok {
		if err := s.UseQueue(jp.Jobs()); err != nil {
			// Not fatal: inline execution is a working scheduler, and a
			// project's scheduled tasks should not stop running because the
			// job queue could not be attached.
			logger.Error("scheduler: could not use job queue, executing inline", "error", err)
		}
	}

	go func() {
		logger.Info("background scheduler starting", "tasks", len(cfg.Tasks))
		if runErr := s.Run(ctx); runErr != nil {
			logger.Error("scheduler stopped with error", "error", runErr)
		}
	}()
}

// jobQueueProvider is the optional capability a WorkspaceProvider may carry to
// hand the scheduler a job queue.
//
// Optional because the scheduler predates the queue and still works without
// one: the interface is type-asserted at StartBackground rather than added to
// WorkspaceProvider, so a provider that has no queue (and every existing test
// double) keeps compiling.
type jobQueueProvider interface {
	Jobs() jobs.Client
}

// stampTaskAuditContext stamps the task's Principal and the per-task
// triggered_by label on a child context so audit records produced by the
// Lua script (directly via rela.create_entity, or indirectly via automation
// cascades) carry the right attribution.
//
// The stamped principal is ALSO what the script's reads resolve against
// (DEC-O59WM4) — the scheduler's identity is the one thing that decides
// what a job can see, via acl.yaml. runAs overrides the default, giving a
// job its own identity for both audit and read scope.
//
// The default is the FIXED [principal.UserScheduler], not the OS user: a
// grantable constant an operator can write into acl.yaml once, rather than
// a per-host value that is "unknown" under systemd (which acl rejects as
// unstamped). See that constant's godoc.
//
// Extracted so the stamping logic can be unit-tested without booting
// the script engine.
func stampTaskAuditContext(ctx context.Context, taskName, runAs string) context.Context {
	user := runAs
	if user == "" {
		user = principal.UserScheduler
	}
	out := principal.With(ctx, principal.Principal{
		User: user,
		Tool: principal.ToolScheduler,
	})
	return audit.WithTriggeredBy(out, "schedule:"+taskName)
}

// Scheduler runs Lua scripts sequentially on simple recurring schedules.
type Scheduler struct {
	config *Config
	engine *script.Engine
	ws     WorkspaceProvider
	state  *State
	logger *slog.Logger
	now    func() time.Time // for testing

	// executeTaskFunc overrides task execution for testing.
	// When nil, doExecuteTask is used.
	executeTaskFunc func(ctx context.Context, task TaskConfig)

	// engineRunner overrides the Lua engine call for testing, WITHOUT
	// bypassing the job handler around it.
	//
	// Distinct from executeTaskFunc, which replaces the whole execution step:
	// a test that wants to exercise the queue path must keep the real handler
	// (it is what reports completion back to the waiting submitter) and
	// substitute only the engine. When nil, the real engine runs.
	engineRunner func(ctx context.Context, task TaskConfig) error

	// queue routes script execution through the job seam. Nil means execute
	// inline, which is the pre-port behavior and what the scheduling tests
	// exercise. Set by UseQueue at wiring time — see jobs.go.
	queue JobQueue

	// inflight tracks the one running execution per task, so a slow task is
	// skipped rather than queued behind itself. Guarded by inflightMu because
	// the writer is the scheduler goroutine and the reader is a queue worker.
	inflightMu sync.Mutex
	inflight   map[string]chan error
}

// New creates a Scheduler.
func New(
	cfg *Config,
	engine *script.Engine,
	ws WorkspaceProvider,
	logger *slog.Logger,
) *Scheduler {
	return &Scheduler{
		config: cfg,
		engine: engine,
		ws:     ws,
		logger: logger,
		now:    time.Now,
		// Initialized here, not left for loadState: recordFailure and
		// recordSuccess write three maps unconditionally, so a nil state
		// turns any call ordering other than Run's into a nil-map panic.
		state: newState(),
	}
}

// Run starts the scheduler and blocks until ctx is cancelled.
// Tasks are executed sequentially in a single goroutine — no concurrent
// script execution, no mutexes needed.
func (s *Scheduler) Run(ctx context.Context) error {
	s.loadState(ctx)

	if len(s.config.Tasks) == 0 {
		s.logger.Info("no tasks configured, waiting for shutdown")
		<-ctx.Done()
		return nil
	}

	for _, t := range s.config.Tasks {
		s.logger.Info("scheduled task", "name", t.Name, "every", t.Every, "script", t.Script)
	}

	// Run due tasks immediately (handles first-ever and missed runs).
	s.runDueTasks(ctx)

	s.logger.Info("scheduler started", "tasks", len(s.config.Tasks))

	ticker := time.NewTicker(tickInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.logger.Info("scheduler stopped")
			return nil
		case <-ticker.C:
			s.runDueTasks(ctx)
		}
	}
}

// runDueTasks checks each task and executes it if due. All execution is
// sequential in the caller's goroutine.
func (s *Scheduler) runDueTasks(ctx context.Context) {
	now := s.now()
	for _, task := range s.config.Tasks {
		if ctx.Err() != nil {
			return
		}

		// A failing task is driven ENTIRELY by the retry ladder: while a
		// retry is pending, the ordinary schedule is suppressed, so the
		// task fires exactly once per ladder step and never on its normal
		// cadence. This is the gate that closes BUG-ZKK2UL — a failed
		// attempt always sets NextRetry, so it can no longer fall through
		// to the "first run" branch below and execute on every tick.
		if retryAt, retrying := s.state.NextRetry[task.Name]; retrying {
			// A pending retry can never legitimately be further out than
			// the longest rung. Anything beyond that came from a clock
			// that jumped (VM snapshot resume, NTP step, bad RTC) or a
			// hand-edited state file, and because the file is the source
			// of truth it would otherwise wedge the task FOREVER — silently,
			// since this branch is the one that logs nothing.
			if retryAt.Sub(now) > maxRetryDelay {
				s.logger.Warn("retry time is implausibly far in the future, retrying now",
					"name", task.Name,
					"scheduled_for", retryAt,
					"max_delay", maxRetryDelay)
				retryAt = now
				s.state.NextRetry[task.Name] = retryAt
			}
			if !now.Before(retryAt) {
				s.logger.Info("retrying failed task",
					"name", task.Name,
					"failures", s.state.Failures[task.Name],
					"scheduled_for", retryAt)
				s.executeTask(ctx, task)
			}
			continue
		}

		lastRun, recorded := s.state.Tasks[task.Name]
		if !recorded {
			s.logger.Info("first run, executing immediately", "name", task.Name)
			s.executeTask(ctx, task)
			continue
		}

		if task.Every.IsDue(lastRun, now) {
			s.logger.Info("task due", "name", task.Name, "last_run", lastRun)
			s.executeTask(ctx, task)
		}
	}
}

func (s *Scheduler) executeTask(ctx context.Context, task TaskConfig) {
	if s.executeTaskFunc != nil {
		s.executeTaskFunc(ctx, task)
		return
	}
	s.doExecuteTask(ctx, task)
}

func (s *Scheduler) doExecuteTask(ctx context.Context, task TaskConfig) {
	if ctx.Err() != nil {
		s.logger.Warn("skipping task, scheduler shutting down", "name", task.Name)
		return
	}

	s.logger.Info("task started", "name", task.Name, "script", task.Script)
	start := s.now()

	err := s.runScript(ctx, task)
	elapsed := s.now().Sub(start)

	// A task still running from a previous slot is skipped, not failed: it has
	// not gone wrong, it is merely slow. Recording a failure would advance the
	// retry ladder against a healthy task and suppress its normal cadence.
	if errors.Is(err, errTaskInFlight) {
		s.logger.Warn("skipping task, previous run still in flight",
			"name", task.Name, "duration", elapsed)
		return
	}

	if err != nil {
		s.recordFailure(ctx, task, start, elapsed, err)
		return
	}

	s.logger.Info("task completed", "name", task.Name, "duration", elapsed)
	s.recordSuccess(ctx, task, start)
}

// runScript executes a task's script, through the job queue when one is wired
// and inline otherwise.
//
// The inline path is not a fallback for a broken queue — it is the pre-port
// behavior, kept because every scheduling test drives execution synchronously
// and asserting on state right afterwards is exactly what those tests are for.
// A scheduler with no queue is a valid scheduler.
func (s *Scheduler) runScript(ctx context.Context, task TaskConfig) error {
	if s.queue != nil {
		// The deadline is measured from the task's last successful run. For a
		// first-ever run there is none, and the zero time would put NextRun in
		// year 1 — a deadline already past, so the queue would drop the job
		// before running it. Measure from now instead: the next slot is one
		// interval away either way.
		lastRun, ran := s.state.Tasks[task.Name]
		if !ran {
			lastRun = s.now()
		}
		return s.enqueueTask(ctx, task, lastRun)
	}

	// The principal goes on the CTX (not into the deps bundle) because the
	// read seam resolves identity per call — see ScheduledLuaWriteDeps.
	taskCtx := stampTaskAuditContext(ctx, task.Name, task.RunAs)
	return s.runEngine(taskCtx, task)
}

// runEngine executes a task's script, honoring the engineRunner test override.
//
// One place, so the inline and queued paths cannot diverge in what they call.
func (s *Scheduler) runEngine(ctx context.Context, task TaskConfig) error {
	if s.engineRunner != nil {
		return s.engineRunner(ctx, task)
	}

	// TKT-YH52OM: the task's declared capabilities are the only ambient grant.
	// A scheduled job runs unattended inside the server process, so an
	// undeclared capability stays absent rather than inheriting the trusted
	// default that `rela script` gets at the operator shell.
	deps := s.ws.ScheduledLuaWriteDeps()
	http, ai, writeFile, secrets := task.Capabilities.Fields()
	deps.Capabilities = lua.Capabilities{
		HTTP: http, AI: ai, WriteFile: writeFile, Secrets: secrets,
	}
	return s.engine.ExecuteFile(ctx, task.Script, deps, nil, nil)
}

// recordSuccess stamps a completed run and clears any retry ladder.
//
// It takes the run's START time rather than reading s.now(): the schedule is
// evaluated against this stamp, so recording completion would let a task that
// begins at 23:59 and runs past midnight land on the next day and silently
// skip that day's execution. It also keeps interval schedules from drifting
// forward by each run's duration.
//
// This is a method rather than inline bookkeeping so tests exercising the
// scheduling loop through executeTaskFunc can call the SAME code the
// production path uses. A hand-copied double here is what previously let a
// reverted start-time fix pass green (RR-F6182G/RR-3BCWQ4).
func (s *Scheduler) recordSuccess(ctx context.Context, task TaskConfig, start time.Time) {
	// No mutex needed, single goroutine.
	s.state.Tasks[task.Name] = start

	// Success clears the ladder. This is the ONLY reset: elapsed scheduled
	// slots must not clear it, or a short-interval task (whose slots pass
	// faster than the ladder climbs) would never back off at all.
	delete(s.state.Failures, task.Name)
	delete(s.state.NextRetry, task.Name)

	s.saveState(ctx)
}

// recordFailure advances the retry ladder for a failed task and persists it.
//
// Every failure writes state, so a failed task always has a pending retry and
// can never be perpetually due — that omission was BUG-ZKK2UL. Note it does
// NOT touch s.state.Tasks: the schedule is evaluated against the last
// *successful* run, so a failure must not count as having run.
func (s *Scheduler) recordFailure(
	ctx context.Context,
	task TaskConfig,
	start time.Time,
	elapsed time.Duration,
	err error,
) {
	failures := s.state.Failures[task.Name] + 1
	delay := retryDelay(failures)
	retryAt := start.Add(delay)

	s.state.Failures[task.Name] = failures
	s.state.NextRetry[task.Name] = retryAt

	// Escalate severity with consecutive failures: an intermittent blip that
	// recovers on the next retry should not read like a job that has been
	// broken for hours.
	logAt := s.logger.Warn
	if failures >= persistentFailureThreshold {
		logAt = s.logger.Error
	}
	logAt("task failed",
		"name", task.Name,
		"duration", elapsed,
		"failures", failures,
		"retry_in", delay,
		"retry_at", retryAt,
		"error", err)

	s.saveState(ctx)
}

// retryDelay returns the backoff for the nth consecutive failure (n >= 1):
// 5m, 10m, 20m, 40m, 80m, then capped at maxRetryDelay and repeating.
//
// The ladder is identical for every schedule. It replaces the schedule while
// a task is failing, so it deliberately slows a short-interval task down
// (a failing 5m task stops hammering every 5m) and speeds a daily one up
// (an intermittent failure recovers without waiting 24h).
func retryDelay(failures int) time.Duration {
	if failures < 1 {
		// Only reachable from a corrupt or hand-edited state file; treat
		// it as the first failure rather than computing a nonsense delay.
		failures = 1
	}
	// maxLadderSteps is where doubling first meets the cap, so anything
	// beyond it is the cap. Bounding the shift here also makes overflow
	// structurally impossible for a large or wrapped failure count.
	if failures > maxLadderSteps {
		return maxRetryDelay
	}
	return min(baseRetryDelay<<(failures-1), maxRetryDelay)
}

func (s *Scheduler) loadState(ctx context.Context) {
	data, err := s.ws.State().Get(ctx, stateFile)
	if err != nil {
		s.state = newState()
		return
	}
	s.state = parseState(data)
	s.pruneOrphanedState()
}

// pruneOrphanedState drops entries for tasks no longer in schedules.yaml.
//
// Nothing else removes them: runDueTasks only ever reads state by the names in
// the current config, so a deleted or renamed task's rows would accumulate
// indefinitely — and with the retry ladder that is up to three entries per
// dead task rather than one stale timestamp.
func (s *Scheduler) pruneOrphanedState() {
	if s.config == nil {
		// Nothing to prune against; keep the state as loaded rather than
		// treating every task as orphaned.
		return
	}
	live := make(map[string]struct{}, len(s.config.Tasks))
	for _, t := range s.config.Tasks {
		live[t.Name] = struct{}{}
	}
	orphans := make(map[string]struct{})
	for _, m := range []iter.Seq[string]{
		maps.Keys(s.state.Tasks),
		maps.Keys(s.state.Failures),
		maps.Keys(s.state.NextRetry),
	} {
		for name := range m {
			if _, ok := live[name]; !ok {
				orphans[name] = struct{}{}
			}
		}
	}
	if len(orphans) == 0 {
		return
	}
	for name := range orphans {
		delete(s.state.Tasks, name)
		delete(s.state.Failures, name)
		delete(s.state.NextRetry, name)
	}
	dropped := slices.Sorted(maps.Keys(orphans))
	s.logger.Info("pruned state for tasks no longer configured", "tasks", dropped)
}

func (s *Scheduler) saveState(ctx context.Context) {
	data, err := s.state.marshal()
	if err != nil {
		s.logger.Error("failed to marshal scheduler state", "error", err)
		return
	}
	if err := s.ws.State().Put(ctx, stateFile, data); err != nil {
		s.logger.Error("failed to save scheduler state", "error", err)
	}
}
