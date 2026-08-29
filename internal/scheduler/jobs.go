package scheduler

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/jobs"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// This file is the scheduler's half of the job seam.
//
// What moves onto the queue is SCRIPT EXECUTION — the slow part, which calls
// out to Lua and from there to HTTP, mail and LLM providers. What stays here is
// everything about *scheduling*: which tasks are due, the retry ladder, the
// clock-jump guard, and the last-successful-run bookkeeping in
// .rela/scheduler-state.json.
//
// That split is deliberate and is the whole point of the port. The queue owns
// "run this, retry it like so"; the scheduler owns "is this due, and did it
// succeed". Moving the bookkeeping too would have meant either duplicating the
// ladder in a handler or letting the queue learn what a schedule is — and the
// seam exists precisely so it never has to.
//
// # Why not neoq's own scheduler
//
// neoq offers StartCron, and it is deliberately unused. It is an in-process
// robfig/cron ticker that enqueues an empty job (memory_backend.go:160):
//
//   - No missed-run detection. It fires only while the process is alive, so a
//     daily task would never run on a desktop app that is closed at the
//     moment it was due. Running a task whose window passed while the app was
//     shut is the main reason this scheduler persists last-run times.
//   - No persistence, on any backend. cron is in-process, so several server
//     instances would each fire every task.
//   - Cron syntax, not schedules.yaml's "day" / "friday" / "30m".
//   - The enqueue error is discarded, so a task that fails to queue vanishes.
//
// Adopting it would also drop the retry ladder, the clock-jump guard
// (BUG-ZKK2UL) and run_as attribution. What neoq is good at — worker pools,
// retry mechanics, deduplication, durability — is what this file uses it for.

// TaskKind is the job kind for a scheduled Lua script.
//
// Namespaced by owner so it cannot collide with another subsystem's kind: every
// subsystem registers into one process-wide queue.
var TaskKind = jobs.NewKind("scheduler", "run-task")

// Payload keys for a task job. The payload snapshots both what to run and its
// authorization: run_as is stamped onto the worker context, while capabilities
// become the Lua runtime's ambient grant.
const (
	payloadTaskName  = "task"
	payloadScript    = "script"
	payloadRunAs     = "run_as"
	payloadRunToken  = "run_token"
	payloadHTTP      = "capability_http"
	payloadAI        = "capability_ai"
	payloadWriteFile = "capability_write_file"
	payloadSecrets   = "capability_secrets"
)

// UseQueue routes script execution through q.
//
// Call it once at wiring time, before Run. It registers the task handler and
// switches doExecuteTask onto the queue; without it the scheduler
// executes inline exactly as before, which is what keeps every existing test
// meaningful.
//
// The handler is registered here rather than by the wiring site because the
// scheduler owns [TaskKind] — a subsystem that owns a kind should be the thing
// that binds it.
//
// Takes [jobs.Client] — production plus registration, no lifecycle — rather
// than a locally-declared interface. The call-site-interface convention exists
// to avoid binding a consumer to methods it does not use, and Client is already
// exactly the two the scheduler needs; restating them here would have been a
// second name for one concept, not a narrowing. Starting and stopping the queue
// stays out of reach, which is the property that mattered.
//
// Nil: rejected — returns an error rather than silently falling back to inline
// execution, since a scheduler that quietly stopped using the queue would be
// indistinguishable from one that never had it.
func (s *Scheduler) UseQueue(q jobs.Client) error {
	if q == nil {
		return errors.New("scheduler: job queue must not be nil")
	}
	if err := q.Register(TaskKind, s.runTaskJob); err != nil {
		return fmt.Errorf("scheduler: register task handler: %w", err)
	}
	if err := q.Register(ExpandKind, s.runExpandJob); err != nil {
		return fmt.Errorf("scheduler: register expansion handler: %w", err)
	}
	if err := q.Register(ChildKind, s.runChildJob); err != nil {
		return fmt.Errorf("scheduler: register child handler: %w", err)
	}
	s.queue = q
	return nil
}

// runTaskJob is the handler that actually executes a scheduled script.
//
// It runs on a queue worker, so the principal and audit attribution have to be
// re-derived HERE from the payload rather than inherited from the enqueueing
// goroutine's context — a worker's ctx is not the submitter's. That is the same
// stamping the inline path does (see stampTaskAuditContext); doing it in one
// place would be nicer but the two contexts are genuinely different.
func (s *Scheduler) runTaskJob(ctx context.Context, job jobs.Job) (err error) {
	name, _ := job.Payload[payloadTaskName].(string)
	script, _ := job.Payload[payloadScript].(string)
	runAs, _ := job.Payload[payloadRunAs].(string)
	token, _ := job.Payload[payloadRunToken].(string)
	capabilities := capabilitiesFromPayload(job.Payload)

	// Hand the outcome back to the waiting submitter, which owns the state
	// bookkeeping, on EVERY exit path.
	//
	// Deferred rather than called before each return, because enqueueTask
	// blocks on this report with only its ctx as an escape: a return that
	// skips it does not fail, it HANGS the scheduler goroutine until the
	// whole scheduler is cancelled, and then reports ctx.Err() in place of
	// the real error. A named return plus a defer makes that structural
	// instead of an obligation every future early return has to remember.
	defer func() { s.reportInFlight(name, token, err) }()

	if script == "" {
		// Unrunnable, and no retry would make a script path appear.
		return fmt.Errorf("scheduler: job %q carries no script", name)
	}

	taskCtx := stampTaskAuditContext(ctx, name, runAs)
	return s.runEngine(taskCtx, TaskConfig{
		Name: name, Script: script, RunAs: runAs, Capabilities: capabilities,
	})
}

// capabilitiesFromPayload restores the authorization snapshot captured when
// the task was enqueued. []any is the durable JSON round-trip shape; []string
// is what the in-memory backend preserves. Unknown values fail closed.
func capabilitiesFromPayload(payload map[string]any) metamodel.Capabilities {
	http, _ := payload[payloadHTTP].(bool)
	ai, _ := payload[payloadAI].(bool)
	writeFile, _ := payload[payloadWriteFile].(bool)

	var secrets []string
	switch values := payload[payloadSecrets].(type) {
	case []string:
		secrets = append([]string(nil), values...)
	case []any:
		for _, value := range values {
			if secret, ok := value.(string); ok {
				secrets = append(secrets, secret)
			}
		}
	}

	return metamodel.Capabilities{
		HTTP: http, AI: ai, WriteFile: writeFile, Secrets: secrets,
	}
}

// enqueueTask submits one task execution and waits for it to finish.
//
// Waiting looks odd for a job queue, and it is deliberate. The scheduler's
// contract is that a task's outcome updates its state before the next tick
// evaluates it: recordSuccess stamps the last successful run, recordFailure
// advances the retry ladder, and runDueTasks reads both. Returning before the
// script has run would let the next tick see stale state.
//
// # SINGLE-NODE ONLY (TKT-7XLVP7)
//
// Waiting here is what confines the scheduler to one process, and the limit is
// in the BOOKKEEPING, not in the queue. Jobs do not double-fire across nodes:
// the idempotency key becomes a fingerprint under a partial UNIQUE INDEX on
// neoq_jobs (queue, fingerprint, status), so two rela-server processes ticking
// at once both INSERT and PostgreSQL lets exactly one win — the loser gets
// ErrDuplicateJob and skips. That part is atomic in the database, not a
// check-then-act.
//
// What does NOT survive a second node is the state machine around it. Workers
// claim with FOR UPDATE SKIP LOCKED, so any node may execute any job: node B
// can run a task node A enqueued. B's reportInFlight then finds nothing in B's
// in-flight map (the map is per-process), A's submitter never hears back, and
// A eventually records a FAILURE via taskResultTimeout for a task that in fact
// SUCCEEDED. The bookkeeping compounds it: state is one document, loaded once
// at startup and rewritten whole on every update through a KV whose Put is an
// unconditional upsert, so two nodes overwrite each other's tasks rather than
// merging them (TKT-DK0X6O). Note the state itself IS shared on the postgres
// tier — it goes through state.KV, which is the state_kv table there — so the
// problem is the write, not the location.
//
// The fix is to stop waiting: give run-state its own per-task storage so a
// write is one record rather than the whole document (TKT-DK0X6O), then let the
// executing node own recordSuccess / recordFailure inside runTaskJob
// (TKT-7XLVP7). That removes the completion channel, both skip sentinels, the
// run token and taskResultTimeout together. Until then, run the
// scheduler on ONE node — docs/postgres-backend.md describes multi-process
// deployments, and this is the piece that does not yet participate.
//
// Retry is [jobs.RetryNever] because the scheduler owns retrying: its ladder
// (5m→2h, suppressing the normal cadence) already encodes hard-won behavior
// including the BUG-ZKK2UL clock-jump guard. Two retry mechanisms stacked on
// one task would multiply, not compose.
//
// The task name is the idempotency key, which is how "do not stack two runs of
// the same task" is expressed. An earlier version used a deadline derived from
// the cadence; that was wrong in a way worth recording, because the reasoning
// looked sound. A deadline makes the job VANISH when it cannot be started in
// time — so under load, when the queue is exactly as busy as the operator most
// wants the work done, scheduled runs are silently dropped. The intent was
// never "expire this"; it was "if one is already pending, that is enough" —
// a daily report delayed six hours should not then send twice. That is
// deduplication by identity, and it degrades the right way: the run happens
// late instead of not at all.
func (s *Scheduler) enqueueTask(ctx context.Context, task TaskConfig) error {
	// A missing queue is a wiring bug, not a runtime condition: StartBackground
	// refuses to start without one. Reported rather than dereferenced so the
	// failure names its cause instead of surfacing as a nil panic in a
	// goroutine.
	if s.queue == nil {
		return errNoQueue
	}

	token, done, err := s.claimInFlight(task.Name)
	if err != nil {
		return err
	}
	defer s.releaseInFlight(task.Name, token)
	if task.ForEach != nil {
		occurrence, ok := task.Every.Occurrence(s.now())
		if !ok {
			return fmt.Errorf("scheduler: task %q has for_each without a calendar occurrence", task.Name)
		}
		if err := s.queue.Enqueue(ctx, jobs.Job{
			Kind: ExpandKind,
			Payload: map[string]any{
				payloadTaskName:   task.Name,
				payloadRunToken:   token,
				payloadOccurrence: occurrence,
			},
			Retry:          jobs.RetryNever,
			IdempotencyKey: task.Name + "/" + occurrence,
		}); err != nil {
			if errors.Is(err, jobs.ErrDuplicateJob) {
				return errTaskPending
			}
			return fmt.Errorf("scheduler: enqueue expansion %q: %w", task.Name, err)
		}
		select {
		case err := <-done:
			return err
		case <-time.After(taskResultTimeout):
			return fmt.Errorf("scheduler: expansion %q reported no result within %s", task.Name, taskResultTimeout)
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	http, ai, writeFile, secrets := task.Capabilities.Fields()

	job := jobs.Job{
		Kind: TaskKind,
		Payload: map[string]any{
			payloadTaskName:  task.Name,
			payloadScript:    task.Script,
			payloadRunAs:     task.RunAs,
			payloadHTTP:      http,
			payloadAI:        ai,
			payloadWriteFile: writeFile,
			payloadSecrets:   append([]string(nil), secrets...),

			// Identifies THIS run, so a late report from a run whose
			// submitter gave up cannot be delivered to a later one.
			payloadRunToken: token,
		},
		Retry: jobs.RetryNever,

		// Scoped to the task, so two tasks never suppress each other. On the
		// durable backend this holds ACROSS processes, which the in-process
		// claim above cannot: two rela-server instances sharing a database
		// will not both queue the same task.
		IdempotencyKey: task.Name,
	}

	if err := s.queue.Enqueue(ctx, job); err != nil {
		// Already pending. Not a failure — the work is scheduled — and not a
		// success either, since this run did not happen. Reported up so
		// doExecuteTask can leave the state alone.
		if errors.Is(err, jobs.ErrDuplicateJob) {
			return errTaskPending
		}
		return fmt.Errorf("scheduler: enqueue task %q: %w", task.Name, err)
	}

	select {
	case err := <-done:
		return err
	case <-time.After(taskResultTimeout):
		// The queue accepted the job but never reported an outcome.
		//
		// Reachable without any handler bug: jobs.dispatch drops a job
		// WITHOUT invoking the handler when its deadline has passed or its
		// retry budget is spent, and a durable backend redelivering a row
		// whose Retries was already incremented (a crash mid-run, a worker
		// killed by the queue's own handler timeout) hits the latter on a
		// RetryNever job — which is every task the scheduler submits.
		//
		// Waiting forever here is not a stalled task, it is a stalled
		// SCHEDULER: runDueTasks executes sequentially in the ticker
		// goroutine, so one stranded wait stops every other task from being
		// evaluated for the life of the process. Reported as a failure so
		// the retry ladder advances and the operator sees it.
		return fmt.Errorf("scheduler: task %q: queue reported no result within %s", task.Name, taskResultTimeout)
	case <-ctx.Done():
		return ctx.Err()
	}
}

// taskResultTimeout bounds how long a submitter waits for a queued run.
//
// Must exceed the queue's own per-handler cap (15m) with margin, so this fires
// only when the result genuinely went missing — never merely because a script
// was slow. It is a liveness backstop, not a task deadline.
const taskResultTimeout = 20 * time.Minute

// claimInFlight reserves a task name and returns the channel its completion
// will be reported on.
//
// It returns [errTaskInFlight] when the task's previous run has not finished.
// The single-threaded loop used to give non-overlap for free; with execution on
// a worker pool it has to be stated, or a slow task on a short cadence queues
// behind itself and piles up.
func (s *Scheduler) claimInFlight(name string) (token string, done <-chan error, err error) {
	s.inflightMu.Lock()
	defer s.inflightMu.Unlock()

	if s.inflight == nil {
		s.inflight = make(map[string]inflightRun)
	}
	if _, busy := s.inflight[name]; busy {
		return "", nil, errTaskInFlight
	}
	// Buffered: the handler must never block reporting a result, even if the
	// submitter has already given up on ctx cancellation.
	ch := make(chan error, 1)
	token = strconv.FormatUint(s.runSeq.Add(1), 10)
	s.inflight[name] = inflightRun{token: token, ch: ch}
	return token, ch, nil
}

// releaseInFlight drops a task's claim, letting its next slot run.
//
// Scoped to the token: a submitter that gave up on ctx cancellation must not
// evict a claim that a LATER run has since installed under the same name.
func (s *Scheduler) releaseInFlight(name, token string) {
	s.inflightMu.Lock()
	defer s.inflightMu.Unlock()
	if cur, ok := s.inflight[name]; ok && cur.token == token {
		delete(s.inflight, name)
	}
}

// reportInFlight delivers a finished run's outcome to whoever is waiting.
//
// A missing entry is not an error: the submitter may have been cancelled, or
// the job may have been redelivered by a durable backend after a restart, in
// which case nothing in this process is waiting. Dropping the result is right
// in both cases — the alternative is blocking a worker forever.
func (s *Scheduler) reportInFlight(name, token string, err error) {
	s.inflightMu.Lock()
	cur, ok := s.inflight[name]
	s.inflightMu.Unlock()
	// Token mismatch means this result belongs to an EARLIER run whose
	// submitter has already given up, and the channel now under this name
	// belongs to a later one. Delivering would hand run N+1 run N's outcome,
	// and the scheduler would stamp the wrong result for the wrong run.
	if !ok || cur.token != token {
		return
	}
	ch := cur.ch
	select {
	case ch <- err:
	default:
	}
}

// errTaskInFlight reports that a task's previous run has not finished.
var errTaskInFlight = errors.New("scheduler: task already in flight")

// errNoQueue reports that the scheduler has no job queue, so it cannot execute
// anything. Only reachable from a hand-built Scheduler that skipped UseQueue.
var errNoQueue = errors.New("scheduler: no job queue configured")

// errTaskPending reports that an identical job is already queued, so this run
// was collapsed into it rather than stacked behind it.
var errTaskPending = errors.New("scheduler: an identical task job is already pending")

// inflightRun is one task execution awaiting its result.
//
// The token disambiguates runs that share a task name: the map is keyed by
// name (that is what enforces non-overlap), but a result must only reach the
// run that produced it.
//
// A process-local counter is enough — the token is only ever compared against
// this process's own map, never persisted or matched across restarts, so it
// needs uniqueness within one process and nothing more.
type inflightRun struct {
	token string
	ch    chan error
}
