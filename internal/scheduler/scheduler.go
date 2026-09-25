// Package scheduler runs Lua scripts on recurring schedules defined in
// schedules.yaml.
//
// Each tick decides which tasks are due, records a run for each in the
// [schedulerstate.Store], and enqueues it on the job queue. It never waits for
// a run to finish: the worker that executes a run records its outcome, and a
// run whose worker vanished is abandoned when its lease expires (BUG-TKL08E).
// A task with an active run is skipped, so a slow task never stacks behind
// itself. Tasks that missed their scheduled window run on the first tick after
// startup. Shutdown is graceful on SIGINT/SIGTERM.
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
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/config"
	"github.com/Sourcehaven-BV/rela/internal/jobs"
	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/schedulerstate"
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
	// "run finished" with a failure escalates from WARN to ERROR — by this
	// point the retries are demonstrably not helping and the job needs a
	// human.
	persistentFailureThreshold = 4
)

// Run leases. A run whose lease passes without an outcome is abandoned by the
// next tick of any scheduler sharing the store, and counts as a failure.
//
// Fixed rather than heartbeat-renewed: the queue already caps a handler at 15
// minutes, so a run that is still genuinely executing can never outlive
// runningLease. The margins also absorb clock skew between nodes, since each
// node stamps leases from its own clock.
const (
	// queuedLease bounds the wait between enqueue and a worker starting the
	// run. Generous, because a busy queue is not a lost run. A job that starts
	// after its run was abandoned finds it no longer queued and does nothing.
	queuedLease = 30 * time.Minute

	// runningLease must exceed the queue's per-handler cap (15m) with margin,
	// so a run is abandoned only when its result is genuinely lost, never
	// merely because a script was slow. for_each progress extends it.
	runningLease = 20 * time.Minute
)

// Run-state housekeeping. pruneAge must exceed the longest schedule (a week),
// or a weekly task's record could be pruned just before it is due.
const (
	pruneInterval = time.Hour
	pruneAge      = 14 * 24 * time.Hour
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

// WorkspaceProvider is the subset of the application services the scheduler
// needs.
type WorkspaceProvider interface {
	Paths() *project.Context
	Config() config.Loader

	// State holds the legacy scheduler-state.json, read once to import it.
	State() state.KV

	// SchedulerState is where task state and runs live. Shared by every
	// process on the same backend, which is what lets them coordinate.
	SchedulerState() schedulerstate.Store

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

	s, err := NewWithQueue(cfg, script.NewEngine(), ws, logger)
	if err != nil {
		logger.Error("scheduler not started", "error", err)
		return
	}

	go func() {
		logger.Info("background scheduler starting", "tasks", len(cfg.Tasks))
		if runErr := s.Run(ctx); runErr != nil {
			// coverage-ignore-start: defensive: Scheduler.Run only ever returns nil (on ctx.Done or empty config), so
			// this error branch is unreachable
			logger.Error("scheduler stopped with error", "error", runErr)
			// coverage-ignore-end
		}
	}()
}

// jobQueueProvider is the capability a WorkspaceProvider must carry to hand the
// scheduler a job queue.
//
// Type-asserted rather than added to WorkspaceProvider so the existing test
// doubles keep compiling, but it is NOT optional in practice: script execution
// happens exclusively on the queue, so a provider without one yields a
// scheduler that cannot run anything.
type jobQueueProvider interface {
	Jobs() jobs.Client
}

// attachQueue wires the workspace's job queue onto s.
//
// Shared by every entry point — StartBackground (rela-server, rela-desktop) and
// the `rela scheduler` command — because forgetting it produces a scheduler
// that starts cleanly, logs a due task every tick, and fails every one of them
// with "no job queue configured". That is exactly the regression a demo caught
// after the unit tests missed it: they all call UseQueue directly, so none of
// them exercised a wiring site.
//
// Nil: rejected — returns an error rather than leaving the scheduler unable to
// execute, so the caller can refuse to start.
func attachQueue(s *Scheduler, ws WorkspaceProvider) error {
	jp, ok := ws.(jobQueueProvider)
	if !ok {
		return errors.New("scheduler: the workspace provides no job queue")
	}
	if err := s.UseQueue(jp.Jobs()); err != nil {
		return fmt.Errorf("scheduler: could not use the job queue: %w", err)
	}
	return nil
}

// NewWithQueue builds a scheduler with its job queue attached.
//
// The constructor entry points should use: script execution happens only on the
// queue, so a Scheduler built by New alone cannot run anything until UseQueue
// is called. Returning an error here means a caller cannot accidentally start a
// scheduler that will fail every task.
func NewWithQueue(
	cfg *Config, engine *script.Engine, ws WorkspaceProvider, logger *slog.Logger,
) (*Scheduler, error) {
	s, err := New(cfg, engine, ws, logger)
	if err != nil {
		return nil, err
	}
	if err := attachQueue(s, ws); err != nil {
		return nil, err
	}
	return s, nil
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

// Scheduler decides which tasks are due and hands them to the job queue.
type Scheduler struct {
	config *Config
	engine *script.Engine
	ws     WorkspaceProvider
	runs   schedulerstate.Store
	logger *slog.Logger
	now    func() time.Time // for testing

	// node identifies this process on the runs it executes, so an operator
	// can tell which node ran (or lost) a run.
	node string

	// engineRunner overrides the Lua engine call for testing, WITHOUT
	// bypassing the job handler around it: the handler is what records the
	// run's outcome, so a test must keep it. When nil, the real engine runs.
	engineRunner func(ctx context.Context, task TaskConfig) error

	// queue is where script execution happens. REQUIRED: there is no inline
	// path, so a scheduler without one cannot run anything. Set by UseQueue at
	// wiring time — see jobs.go.
	queue jobs.Client

	// lastPrune is when the tick last pruned run-state. Only the scheduler
	// goroutine touches it.
	lastPrune time.Time
}

// New creates a Scheduler.
//
// Nil: rejected — cfg, engine, ws, logger and ws.SchedulerState() are all
// required. A scheduler without run-state would treat every task as due on
// every tick.
func New(
	cfg *Config,
	engine *script.Engine,
	ws WorkspaceProvider,
	logger *slog.Logger,
) (*Scheduler, error) {
	switch {
	case cfg == nil:
		return nil, errors.New("scheduler: config must not be nil")
	case engine == nil:
		return nil, errors.New("scheduler: script engine must not be nil")
	case ws == nil:
		return nil, errors.New("scheduler: workspace must not be nil")
	case logger == nil:
		return nil, errors.New("scheduler: logger must not be nil")
	}
	runs := ws.SchedulerState()
	if runs == nil {
		return nil, errors.New("scheduler: the workspace provides no run-state store")
	}
	return &Scheduler{
		config: cfg,
		engine: engine,
		ws:     ws,
		runs:   runs,
		logger: logger,
		now:    time.Now,
		node:   nodeID(),
	}, nil
}

// nodeID names this process: host and pid.
func nodeID() string {
	host, err := os.Hostname()
	if err != nil || host == "" {
		host = "unknown-host"
	}
	return host + ":" + strconv.Itoa(os.Getpid())
}

// Run starts the scheduler and blocks until ctx is cancelled.
func (s *Scheduler) Run(ctx context.Context) error {
	if len(s.config.Tasks) == 0 {
		s.logger.Info("no tasks configured, waiting for shutdown")
		<-ctx.Done()
		return nil
	}

	s.importLegacyState(ctx)

	for _, t := range s.config.Tasks {
		s.logger.Info("scheduled task", "task", t.Name, "every", t.Every, "script", t.Script)
	}

	// Run due tasks immediately (handles first-ever and missed runs).
	s.tick(ctx)

	s.logger.Info("scheduler started", "tasks", len(s.config.Tasks), "node", s.node)

	ticker := time.NewTicker(tickInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.logger.Info("scheduler stopped")
			return nil
		case <-ticker.C:
			s.tick(ctx)
		}
	}
}

// tick abandons lost runs, then queues a run for every due task. It never
// waits for a run, so one slow or lost run cannot delay any other task.
func (s *Scheduler) tick(ctx context.Context) {
	now := s.now()

	reaped, err := s.runs.Reap(ctx, now, retryDelay)
	if err != nil {
		s.logger.Error("scheduler: could not reap expired runs", "error", err)
	}
	for _, fin := range reaped {
		s.logger.Error("run abandoned",
			"task", fin.Run.Task,
			"run_id", fin.Run.ID,
			"node", fin.Run.Node,
			"reason", fin.Run.Error,
			"failures", fin.Failures,
			"retry_at", fin.NextRetry)
	}

	names := make([]string, 0, len(s.config.Tasks))
	for _, task := range s.config.Tasks {
		names = append(names, task.Name)
	}
	states, err := s.runs.Load(ctx, names)
	if err != nil {
		// Without state every task would look due. Skipping the tick is the
		// safe direction: the next one tries again.
		s.logger.Error("scheduler: could not load run-state, skipping tick", "error", err)
		return
	}

	for _, task := range s.config.Tasks {
		if ctx.Err() != nil {
			return
		}
		ts, recorded := states[task.Name]
		reason, due := s.dueReason(task, ts, recorded, now)
		if !due {
			continue
		}
		if ts.Active != nil {
			s.logger.Info("task due but its previous run is still active, skipping",
				"task", task.Name,
				"run_id", ts.Active.ID,
				"status", ts.Active.Status,
				"node", ts.Active.Node,
				"age", now.Sub(ts.Active.CreatedAt).Round(time.Second))
			continue
		}
		s.queueRun(ctx, task, ts.Version, reason, now)
	}

	s.maybePrune(ctx, now)
}

// dueReason reports whether task is due at now, and why.
func (s *Scheduler) dueReason(
	task TaskConfig, ts schedulerstate.TaskState, recorded bool, now time.Time,
) (string, bool) {
	// A failing task is driven ENTIRELY by the retry ladder: while a retry
	// is pending, the ordinary schedule is suppressed, so the task fires
	// exactly once per ladder step and never on its normal cadence
	// (BUG-ZKK2UL).
	if ts.Failing() {
		clamped, fixed := schedulerstate.ClampRetry(ts, now, maxRetryDelay)
		if fixed {
			s.logger.Warn("retry time is implausibly far in the future, retrying now",
				"task", task.Name,
				"scheduled_for", ts.NextRetry,
				"max_delay", maxRetryDelay)
		}
		return "retry", !now.Before(clamped.NextRetry)
	}
	if !recorded || ts.LastRun.IsZero() {
		return "first run", true
	}
	return "due", task.Every.IsDue(ts.LastRun, now)
}

// queueRun records a run of task and enqueues the job that executes it.
//
// Creation is conditional on the task-state version the tick read, so two
// schedulers ticking at once create one run, and a scheduler whose view went
// stale (another node just finished this task) creates none.
func (s *Scheduler) queueRun(ctx context.Context, task TaskConfig, version int64, reason string, now time.Time) {
	run := schedulerstate.Run{
		ID:         uuid.NewString(),
		Task:       task.Name,
		CreatedAt:  now,
		LeaseUntil: now.Add(queuedLease),
	}
	job, err := s.jobFor(task, run.ID, now)
	if err == nil {
		run.Occurrence, _ = job.Payload[payloadOccurrence].(string)
		err = s.runs.CreateRun(ctx, run, version)
	}
	switch {
	case errors.Is(err, schedulerstate.ErrRunActive), errors.Is(err, schedulerstate.ErrStale):
		s.logger.Info("task already handled by another scheduler, skipping",
			"task", task.Name, "reason", err)
		return
	case err != nil:
		s.logger.Error("could not create run", "task", task.Name, "error", err)
		return
	}

	if s.queue == nil {
		err = errNoQueue
	} else {
		err = s.queue.Enqueue(ctx, job)
	}
	if err != nil {
		// The run exists but its job does not, so nothing will ever start
		// it. Ending it now advances the ladder immediately instead of
		// holding the task until the queued lease expires.
		s.finish(ctx, run, fmt.Errorf("enqueue: %w", err))
		return
	}
	s.logger.Info("run queued", "task", task.Name, "run_id", run.ID, "reason", reason)
}

// maybePrune drops old run-state at most once per pruneInterval.
func (s *Scheduler) maybePrune(ctx context.Context, now time.Time) {
	if now.Sub(s.lastPrune) < pruneInterval {
		return
	}
	s.lastPrune = now
	removed, err := s.runs.Prune(ctx, now.Add(-pruneAge))
	if err != nil {
		s.logger.Warn("scheduler: could not prune run-state", "error", err)
		return
	}
	if len(removed) > 0 {
		s.logger.Info("pruned run-state for tasks idle since the cut-off",
			"tasks", removed, "cutoff", now.Add(-pruneAge))
	}
}

// beginRun claims a queued run for execution on this node. It returns false
// when this delivery must not execute: the run already started or ended (a
// duplicate or late delivery), or its record is gone.
func (s *Scheduler) beginRun(ctx context.Context, job jobs.Job, id, task string) (schedulerstate.Run, bool) {
	now := s.now()
	run, err := s.runs.StartRun(ctx, id, s.node, now, now.Add(runningLease))
	switch {
	case errors.Is(err, schedulerstate.ErrNotQueued):
		s.logger.Warn("duplicate delivery skipped, run is no longer queued",
			"task", task, "run_id", id, "status", run.Status, "node", run.Node, "attempt", job.Attempt)
		return run, false
	case errors.Is(err, schedulerstate.ErrNoRun):
		s.logger.Warn("job skipped, its run record no longer exists", "task", task, "run_id", id)
		return run, false
	case err != nil:
		// Not executed: the run stays queued and is abandoned when its lease
		// expires, which is the recoverable direction.
		s.logger.Error("could not start run", "task", task, "run_id", id, "error", err)
		return run, false
	}
	s.logger.Info("run started",
		"task", task,
		"run_id", id,
		"node", s.node,
		"attempt", job.Attempt,
		"queue_wait", now.Sub(run.CreatedAt).Round(time.Millisecond))
	return run, true
}

// finishTimeout bounds recording an outcome. The handler's own ctx may already
// be cancelled (shutdown, the queue's handler timeout), and the outcome must be
// recorded regardless.
const finishTimeout = 30 * time.Second

// finish ends run with runErr's outcome and logs it.
func (s *Scheduler) finish(ctx context.Context, run schedulerstate.Run, runErr error) {
	out := schedulerstate.Outcome{At: s.now()}
	if runErr != nil {
		out.Error = runErr.Error()
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), finishTimeout)
	defer cancel()
	fin, err := s.runs.FinishRun(ctx, run.ID, out, retryDelay)
	if err != nil {
		// The lease reaps it later and the task retries; logged because the
		// real outcome is lost.
		s.logger.Error("could not record run outcome",
			"task", run.Task, "run_id", run.ID, "outcome_error", out.Error, "error", err)
		return
	}
	s.logFinished(fin)
}

// logFinished reports a run's end: INFO on success, WARN on failure, ERROR once
// failures persist.
func (s *Scheduler) logFinished(fin schedulerstate.Finished) {
	run := fin.Run
	if !fin.Applied {
		s.logger.Warn("late result discarded, run had already ended",
			"task", run.Task, "run_id", run.ID, "status", run.Status)
		return
	}
	attrs := []any{"task", run.Task, "run_id", run.ID, "status", run.Status}
	if !run.StartedAt.IsZero() {
		attrs = append(attrs, "duration", run.FinishedAt.Sub(run.StartedAt).Round(time.Millisecond))
	}
	if run.Children > 0 {
		attrs = append(attrs, "subjects", run.Children, "subjects_failed", run.ChildrenFailed)
	}
	if run.Status == schedulerstate.RunSucceeded {
		s.logger.Info("run finished", attrs...)
		return
	}
	logAt := s.logger.Warn
	if fin.Failures >= persistentFailureThreshold {
		logAt = s.logger.Error
	}
	attrs = append(attrs,
		"failures", fin.Failures,
		"retry_at", fin.NextRetry,
		"error", run.Error)
	logAt("run finished", attrs...)
}

// runEngine executes a task's script, honoring the engineRunner test override.
func (s *Scheduler) runEngine(ctx context.Context, task TaskConfig) error {
	if s.engineRunner != nil {
		return s.engineRunner(ctx, task)
	}

	// TKT-YH52OM: the task's declared capabilities are the only ambient grant.
	// A scheduled job runs unattended inside the server process, so an
	// undeclared capability stays absent rather than inheriting the trusted
	// default that `rela script` gets at the operator shell.
	deps := s.ws.ScheduledLuaWriteDeps()
	http, ai, mail, writeFile, secrets := task.Capabilities.Fields()
	deps.Capabilities = lua.Capabilities{
		HTTP: http, AI: ai, Mail: mail, WriteFile: writeFile, Secrets: secrets,
	}
	return s.engine.ExecuteFile(ctx, task.Script, deps, nil, nil)
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
		// Only reachable from a corrupt or hand-edited record; treat it as
		// the first failure rather than computing a nonsense delay.
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

// importLegacyState moves a scheduler-state.json written by an older release
// into the run-state store, then deletes it.
//
// Seed never overwrites, so a second process importing the same file, or a
// file that reappears, cannot roll newer state back. The file is deleted only
// after every task seeded, so a failed import is retried on the next start.
func (s *Scheduler) importLegacyState(ctx context.Context) {
	data, err := s.ws.State().Get(ctx, stateFile)
	if err != nil {
		return
	}
	legacy := parseState(data)
	for name, lastRun := range legacy.Tasks {
		if err := s.runs.Seed(ctx, name, schedulerstate.TaskState{
			LastRun: lastRun, Failures: legacy.Failures[name], NextRetry: legacy.NextRetry[name],
		}); err != nil {
			s.logger.Error("could not import legacy scheduler state", "task", name, "error", err)
			return
		}
	}
	for name, retryAt := range legacy.NextRetry {
		if _, seeded := legacy.Tasks[name]; seeded {
			continue
		}
		if err := s.runs.Seed(ctx, name, schedulerstate.TaskState{
			Failures: legacy.Failures[name], NextRetry: retryAt,
		}); err != nil {
			s.logger.Error("could not import legacy scheduler state", "task", name, "error", err)
			return
		}
	}
	if err := s.ws.State().Delete(ctx, stateFile); err != nil {
		s.logger.Warn("imported legacy scheduler state but could not delete it", "error", err)
		return
	}
	s.logger.Info("imported legacy scheduler state", "tasks", len(legacy.Tasks))
}
