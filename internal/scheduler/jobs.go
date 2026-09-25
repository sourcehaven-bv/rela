package scheduler

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/jobs"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// This file is the scheduler's half of the job seam.
//
// What moves onto the queue is SCRIPT EXECUTION — the slow part, which calls
// out to Lua and from there to HTTP, mail and LLM providers. What stays with
// the scheduler is everything about *scheduling*: which tasks are due, the
// retry ladder, the clock-jump guard. The run's outcome is recorded by the
// handler into the shared run-state store, never handed back through the
// process that enqueued it (BUG-TKL08E).
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
// retry mechanics, durability — is what this file uses it for.

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
	payloadRunID     = "run_id"
	payloadScript    = "script"
	payloadRunAs     = "run_as"
	payloadHTTP      = "capability_http"
	payloadAI        = "capability_ai"
	payloadMail      = "capability_mail"
	payloadWriteFile = "capability_write_file"
	payloadSecrets   = "capability_secrets"
)

// UseQueue routes script execution through q.
//
// Call it once at wiring time, before Run. The handlers are registered here
// rather than by the wiring site because the scheduler owns its kinds — a
// subsystem that owns a kind should be the thing that binds it.
//
// Takes [jobs.Client] — production plus registration, no lifecycle — rather
// than a locally-declared interface. The call-site-interface convention exists
// to avoid binding a consumer to methods it does not use, and Client is already
// exactly the two the scheduler needs. Starting and stopping the queue stays
// out of reach, which is the property that mattered.
//
// Nil: rejected — a scheduler without a queue cannot execute anything.
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

// jobFor builds the job that executes one run of task.
//
// Retry is [jobs.RetryNever] because the scheduler owns retrying: its ladder
// (5m→2h, suppressing the normal cadence) already encodes hard-won behavior
// including the BUG-ZKK2UL clock-jump guard. Two retry mechanisms stacked on
// one task would multiply, not compose.
//
// The idempotency key is the RUN id, never the bare task name. Non-overlap is
// the run-state store's job (one active run per task, across processes). A
// task-name key made the queue a second, invisible source of truth: a job row
// the queue could not complete held the key forever and rejected every later
// run as "already pending" (BUG-TKL08E).
//
// It also returns the run's occurrence: the calendar slot a for_each run
// serves, or "" for a plain task.
func (s *Scheduler) jobFor(task TaskConfig, runID string, now time.Time) (jobs.Job, string, error) {
	if task.ForEach != nil {
		occurrence, ok := task.Every.Occurrence(now)
		if !ok {
			return jobs.Job{}, "", fmt.Errorf("scheduler: task %q has for_each without a calendar occurrence", task.Name)
		}
		return jobs.Job{
			Kind: ExpandKind,
			Payload: map[string]any{
				payloadTaskName:   task.Name,
				payloadRunID:      runID,
				payloadOccurrence: occurrence,
			},
			Retry:          jobs.RetryNever,
			IdempotencyKey: runID,
		}, occurrence, nil
	}

	http, ai, mail, writeFile, secrets := task.Capabilities.Fields()
	return jobs.Job{
		Kind: TaskKind,
		Payload: map[string]any{
			payloadTaskName:  task.Name,
			payloadRunID:     runID,
			payloadScript:    task.Script,
			payloadRunAs:     task.RunAs,
			payloadHTTP:      http,
			payloadAI:        ai,
			payloadMail:      mail,
			payloadWriteFile: writeFile,
			payloadSecrets:   append([]string(nil), secrets...),
		},
		Retry:          jobs.RetryNever,
		IdempotencyKey: runID,
	}, "", nil
}

// runTaskJob is the handler that executes a scheduled script and records its
// outcome.
//
// It runs on a queue worker, so the principal and audit attribution have to be
// re-derived HERE from the payload rather than inherited from the enqueueing
// goroutine's context — a worker's ctx is not the submitter's, and on the
// durable tier it may be another process.
func (s *Scheduler) runTaskJob(ctx context.Context, job jobs.Job) error {
	name, _ := job.Payload[payloadTaskName].(string)
	runID, _ := job.Payload[payloadRunID].(string)
	script, _ := job.Payload[payloadScript].(string)
	runAs, _ := job.Payload[payloadRunAs].(string)

	if runID == "" {
		// A job enqueued by an older release, which tracked runs in memory.
		// Nothing records its outcome, and the scheduler re-queues the task
		// under a run of its own.
		s.logger.Warn("job without a run id dropped", "task", name)
		return nil
	}
	run, ok := s.beginRun(ctx, job, runID, name)
	if !ok {
		return nil
	}

	var err error
	if script == "" {
		// Unrunnable, and no retry would make a script path appear.
		err = fmt.Errorf("scheduler: job %q carries no script", name)
	} else {
		err = s.runEngine(stampTaskAuditContext(ctx, name, runAs), TaskConfig{
			Name: name, Script: script, RunAs: runAs, Capabilities: capabilitiesFromPayload(job.Payload),
		})
	}
	s.finish(ctx, run, err)
	return err
}

// capabilitiesFromPayload restores the authorization snapshot captured when
// the task was enqueued. []any is the durable JSON round-trip shape; []string
// is what the in-memory backend preserves. Unknown values fail closed.
func capabilitiesFromPayload(payload map[string]any) metamodel.Capabilities {
	http, _ := payload[payloadHTTP].(bool)
	ai, _ := payload[payloadAI].(bool)
	mail, _ := payload[payloadMail].(bool)
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
		HTTP: http, AI: ai, Mail: mail, WriteFile: writeFile, Secrets: secrets,
	}
}

// errNoQueue reports that the scheduler has no job queue, so it cannot execute
// anything. Only reachable from a hand-built Scheduler that skipped UseQueue.
var errNoQueue = errors.New("scheduler: no job queue configured")
