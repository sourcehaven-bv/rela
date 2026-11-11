//go:build postgres

package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivertype"
)

// pgConcurrency is the worker count for a postgres queue.
//
// Higher than the memory tier: this is the networked deployment, where jobs
// are I/O-bound against external systems and the process is not sharing a
// machine with a user's desktop session.
const pgConcurrency = 10

// pgPoolSize is the size of the queue's own connection pool.
//
// River does not hold a connection while a handler runs, so the pool does not
// grow with pgConcurrency. It holds River's LISTEN connection for the life of
// the queue, plus the fetches, completions, maintenance and inserts that share
// the rest. Fewer than four starves Enqueue behind River's own work.
const pgPoolSize = 6

// riverQueueName is the River queue every rela job is inserted into. rela
// routes by [Kind] inside one queue rather than giving each kind a River queue
// of its own: one worker pool with a dispatch table keeps concurrency
// governable in one place.
const riverQueueName = "rela"

// finishedJobRetention is how long River keeps the row of a finished job:
// completed, cancelled or discarded. A day leaves an operator time to inspect
// a failure; nothing in rela reads the rows. River's job cleaner deletes them
// (TKT-RJS3KKJ).
const finishedJobRetention = 24 * time.Hour

// Unknown-kind handling. A worker can fetch a kind it has not registered: with
// two binary versions on one database during a deploy, or before a subsystem
// has registered after Start. The job is snoozed, which does not use an
// attempt, and dropped once it has waited unknownKindMaxSnoozes times.
const (
	unknownKindSnooze     = time.Minute
	unknownKindMaxSnoozes = 60
)

// NewPostgresQueue returns a DURABLE queue backed by PostgreSQL through River
// (github.com/riverqueue/river): jobs survive process restarts and are safe
// to process from several rela-server processes against one database.
//
// The queue's tables live in the schema the DSN's search_path resolves to,
// like every other table rela keeps in postgres; River's notification topics
// are prefixed with the same schema, so tenants never wake each other's
// workers. The constructor migrates River's tables and moves any jobs a neoq
// build left behind (see neoqcutover.go).
//
// It takes a DSN and opens a pool of its own rather than sharing the store's.
// appbuild hands the store pool around as pgstore.DBTX, which keeps pgx types
// out of the composition root, and River needs a concrete *pgxpool.Pool.
//
// The DSN comes from RELA_DATABASE_URL via appbuild.Config — never a flag, so
// the credential stays out of ps output and shell history. It must never reach
// an error string.
//
// Nil: rejected — a nil logger is a wiring bug.
func NewPostgresQueue(ctx context.Context, logger *slog.Logger, databaseURL string) (Queue, error) {
	if logger == nil {
		return nil, errors.New("jobs: logger must not be nil")
	}
	if databaseURL == "" {
		return nil, errors.New("jobs: database URL must not be empty")
	}
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		// pgx redacts the password in parse errors, but the rest of the DSN
		// is not ours to echo either.
		return nil, errors.New("jobs: parse database URL")
	}
	cfg.MaxConns = pgPoolSize
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("jobs: open queue pool: %w", err)
	}
	q, err := newPgQueue(ctx, logger, pool)
	if err != nil {
		pool.Close()
		return nil, err
	}
	return q, nil
}

func newPgQueue(ctx context.Context, logger *slog.Logger, pool *pgxpool.Pool) (*pgQueue, error) {
	var schema *string
	if err := pool.QueryRow(ctx, `SELECT current_schema()`).Scan(&schema); err != nil {
		return nil, fmt.Errorf("jobs: resolve queue schema: %w", err)
	}
	// NULL means no schema on the search_path exists. River would then fail
	// on its first table; saying so here names the actual cause.
	if schema == nil {
		return nil, errors.New("jobs: no schema on the search_path exists")
	}

	q := &pgQueue{
		logger:   logger,
		pool:     pool,
		schema:   *schema,
		handlers: make(map[Kind]Handler),
	}
	workers := river.NewWorkers()
	if err := river.AddWorkerSafely(workers, &riverWorker{q: q}); err != nil {
		return nil, fmt.Errorf("jobs: register river worker: %w", err)
	}
	client, err := river.NewClient(riverpgxv5.New(pool), &river.Config{
		Schema:                      q.schema,
		Logger:                      logger.With("component", "river"),
		Queues:                      map[string]river.QueueConfig{riverQueueName: {MaxWorkers: pgConcurrency}},
		Workers:                     workers,
		JobTimeout:                  handlerTimeout,
		CompletedJobRetentionPeriod: finishedJobRetention,
		CancelledJobRetentionPeriod: finishedJobRetention,
		DiscardedJobRetentionPeriod: finishedJobRetention,
		ErrorHandler:                &riverErrorHandler{logger: logger},
	})
	if err != nil {
		return nil, fmt.Errorf("jobs: create river client: %w", err)
	}
	q.client = client

	if err := migrateRiver(ctx, pool, q.schema, logger); err != nil {
		return nil, err
	}
	cutoverFromNeoq(ctx, q)
	return q, nil
}

// pgQueue is the River-backed [Queue].
type pgQueue struct {
	logger *slog.Logger
	pool   *pgxpool.Pool
	client *river.Client[pgx.Tx]
	schema string

	// mu guards the fields below it. Enqueue holds it as a reader across
	// the insert, so Close, which takes it as a writer to set closed, never
	// closes the pool under an insert.
	mu       sync.RWMutex
	handlers map[Kind]Handler
	started  bool
	closed   bool
	// drained is closed once the client has stopped and the pool is
	// closed. Set by the first Close; later calls wait on it too.
	drained chan struct{}

	// closing is set before Close cancels the running handlers, so a worker
	// can tell a cancellation caused by shutdown from a failure.
	closing atomic.Bool
}

// riverArgs is the one River job kind rela inserts. It carries rela's own
// job description; River sees a single kind and rela dispatches inside it.
type riverArgs struct {
	// RelaKind routes the job to a registered [Handler].
	RelaKind Kind `json:"kind"`
	// Payload is the caller's payload, untouched.
	Payload map[string]any `json:"payload,omitempty"`
	// Retry is the caller's retry intent. River's MaxAttempts is set from
	// it at insert; it is carried so the handler sees it.
	Retry Retry `json:"retry"`
	// Deadline is the effective deadline in Unix nanoseconds, zero for none.
	Deadline int64 `json:"deadline,omitempty"`
	// Key is [idempotencyID] of a keyed job, empty otherwise. It is River's
	// uniqueness field, and is set only together with uniqueOpts.
	Key string `json:"key,omitempty" river:"unique"`
	// AttemptOffset is how many attempts the job used before it reached
	// River. Only a job moved from neoq has one.
	AttemptOffset int `json:"attempt_offset,omitempty"`
}

// Kind implements river.JobArgs.
func (riverArgs) Kind() string { return "rela" }

// uniqueOpts makes a keyed job unique among PENDING jobs only. A completed,
// cancelled or discarded job frees its key, which is what makes the key a
// concurrency guard rather than a record of what has run. River requires the
// first four states; retryable is the job between attempts, still pending.
var uniqueOpts = river.UniqueOpts{ //nolint:gochecknoglobals // a constant River cannot express as one
	ByArgs: true,
	ByState: []rivertype.JobState{
		rivertype.JobStateAvailable,
		rivertype.JobStatePending,
		rivertype.JobStateRunning,
		rivertype.JobStateScheduled,
		rivertype.JobStateRetryable,
	},
}

// Register implements [Queue].
func (q *pgQueue) Register(kind Kind, h Handler) error {
	if kind == "" {
		return ErrNoKind
	}
	if h == nil {
		return fmt.Errorf("jobs: nil handler for kind %q", kind)
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.closed {
		return ErrClosed
	}
	if _, exists := q.handlers[kind]; exists {
		return fmt.Errorf("%w: %q", ErrDuplicateKind, kind)
	}
	q.handlers[kind] = h
	return nil
}

// Enqueue implements [Queue].
func (q *pgQueue) Enqueue(ctx context.Context, job Job) error {
	if err := job.validate(); err != nil {
		return err
	}
	q.mu.RLock()
	closed := q.closed
	_, known := q.handlers[job.Kind]
	q.mu.RUnlock()
	if closed {
		return ErrClosed
	}
	if !known {
		return fmt.Errorf("%w: %q", ErrUnknownKind, job.Kind)
	}

	now := time.Now()
	if job.expired(now) {
		q.logger.Debug("job dropped, deadline already passed",
			"kind", job.Kind, "deadline", job.Deadline)
		return nil
	}
	if deferEnqueue(ctx, job) {
		return nil
	}

	args, opts := insertParams(job, now)

	q.mu.RLock()
	defer q.mu.RUnlock()
	switch {
	case q.closed:
		return ErrClosed
	case !q.started:
		// River would insert without a started client. The contract says
		// a queue must run before it accepts work, so both tiers agree.
		return ErrNotStarted
	}
	res, err := q.client.Insert(ctx, args, opts)
	if err != nil {
		return fmt.Errorf("jobs: enqueue %q: %w", job.Kind, err)
	}
	if res.UniqueSkippedAsDuplicate {
		q.logger.Debug("job already pending, not queued again",
			"kind", job.Kind, "idempotency_key", job.IdempotencyKey)
		return ErrDuplicateJob
	}
	return nil
}

// insertParams builds the River args and options for job, enqueued at now.
func insertParams(job Job, now time.Time) (riverArgs, *river.InsertOpts) {
	args := riverArgs{RelaKind: job.Kind, Payload: job.Payload, Retry: job.Retry}
	if dl := job.Retry.effectiveDeadline(job, now); !dl.IsZero() {
		args.Deadline = dl.UnixNano()
	}
	opts := &river.InsertOpts{Queue: riverQueueName, MaxAttempts: job.Retry.maxAttempts()}
	if job.IdempotencyKey != "" {
		args.Key = idempotencyID(job)
		opts.UniqueOpts = uniqueOpts
	}
	return args, opts
}

// Start implements [Queue].
func (q *pgQueue) Start(ctx context.Context) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.closed {
		return ErrClosed
	}
	if q.started {
		return errors.New("jobs: queue already started")
	}
	// River's client lives as long as the context it was started with. The
	// queue lives until Close, not until the call that happened to start it.
	if err := q.client.Start(context.WithoutCancel(ctx)); err != nil {
		return fmt.Errorf("jobs: start queue: %w", err)
	}
	q.started = true
	return nil
}

// Close implements [Queue].
//
// Running handlers' contexts are cancelled. A handler that returns because of
// that does not use an attempt: its job goes back to the queue for another
// process. Jobs not yet fetched stay in the database.
//
// When ctx ends before the handlers return, the goroutine that waits for them
// lives on until they do, and closes the pool after them.
func (q *pgQueue) Close(ctx context.Context) error {
	q.mu.Lock()
	if !q.closed {
		q.closed = true
		q.closing.Store(true)
		started := q.started
		drained := make(chan struct{})
		go func() {
			defer close(drained)
			if started {
				// Without ctx's cancellation on purpose: the caller's ctx
				// bounds how long Close waits, not how long the handlers
				// may take.
				if err := q.client.StopAndCancel(context.WithoutCancel(ctx)); err != nil {
					q.logger.Warn("jobs: stop river client", "error", err)
				}
			}
			q.pool.Close()
		}()
		q.drained = drained
	}
	drained := q.drained
	q.mu.Unlock()

	select {
	case <-drained:
		return nil
	case <-ctx.Done():
	}
	select {
	case <-drained:
		return nil
	default:
		return fmt.Errorf("jobs: handlers still running at close: %w", ctx.Err())
	}
}

// riverWorker is River's worker for [riverArgs]; it hands each job to q.
type riverWorker struct {
	river.WorkerDefaults[riverArgs]
	q *pgQueue
}

// NextRetry implements river.Worker with rela's own backoff.
func (w *riverWorker) NextRetry(job *river.Job[riverArgs]) time.Time {
	return time.Now().Add(retryDelay(job.Attempt + job.Args.AttemptOffset))
}

// Work implements river.Worker.
func (w *riverWorker) Work(ctx context.Context, rj *river.Job[riverArgs]) error {
	return w.q.work(ctx, rj)
}

// work runs one attempt. Its return value tells River what follows: nil
// completes the job, river.JobCancel stops it for good, river.JobSnooze puts
// it back without using an attempt, and any other error is a failed attempt
// that River retries until MaxAttempts.
func (q *pgQueue) work(ctx context.Context, rj *river.Job[riverArgs]) error {
	args := rj.Args
	job := Job{
		Kind:    args.RelaKind,
		Payload: args.Payload,
		Retry:   args.Retry,
		Attempt: rj.Attempt + args.AttemptOffset,
	}
	if args.Deadline != 0 {
		job.Deadline = time.Unix(0, args.Deadline)
	}
	if job.Payload == nil {
		job.Payload = map[string]any{}
	}

	q.mu.RLock()
	h, ok := q.handlers[job.Kind]
	q.mu.RUnlock()
	if !ok {
		return q.unknownKind(rj, job)
	}

	if !job.Deadline.IsZero() && !time.Now().Before(job.Deadline) {
		q.logger.Warn("dropping job past its deadline",
			"kind", job.Kind, "job_id", rj.ID, "attempt", job.Attempt, "deadline", job.Deadline)
		return river.JobCancel(errors.New("jobs: deadline passed"))
	}

	err := h(ctx, job)
	if err == nil {
		return nil
	}
	if q.closing.Load() && ctx.Err() != nil {
		// Close cancelled the handler, so the failure is not the job's
		// fault: put it back for another process without using an attempt.
		// Not for a job with no attempt left. Its side effect may already
		// have happened, and running it again would break its retry intent,
		// as dropping it on the memory tier does not.
		if job.Retry != RetryNever && rj.Attempt < rj.MaxAttempts {
			return river.JobSnooze(0)
		}
		q.logger.Warn("dropping job interrupted by close, it has no attempt left",
			"kind", job.Kind, "job_id", rj.ID, "attempt", job.Attempt)
		// %v, not %w: River puts back any job whose error wraps the stop's
		// context.Canceled, even a cancelled one.
		return river.JobCancel(fmt.Errorf("jobs: interrupted by close: %v", err)) //nolint:errorlint // see above
	}
	// The payload is NOT logged: it may carry entity content.
	q.logger.Warn("job attempt failed",
		"kind", job.Kind, "job_id", rj.ID, "attempt", job.Attempt, "error", err)
	if rj.Attempt >= rj.MaxAttempts {
		q.logger.Error("dropping job, retry budget exhausted",
			"kind", job.Kind, "job_id", rj.ID, "attempts", job.Attempt, "retry", job.Retry.String())
		return err
	}
	if !job.Deadline.IsZero() && !time.Now().Add(retryDelayFloor(job.Attempt)).Before(job.Deadline) {
		q.logger.Warn("dropping job, its deadline passes before the next attempt",
			"kind", job.Kind, "job_id", rj.ID, "attempt", job.Attempt, "deadline", job.Deadline)
		return river.JobCancel(err)
	}
	return err
}

// unknownKind snoozes a job whose kind this process has not registered, and
// drops it once it has waited long enough that no registration is coming.
func (q *pgQueue) unknownKind(rj *river.Job[riverArgs], job Job) error {
	var meta struct {
		Snoozes int `json:"snoozes"`
	}
	_ = json.Unmarshal(rj.Metadata, &meta) // absent or malformed counts as zero
	expired := !job.Deadline.IsZero() && !time.Now().Add(unknownKindSnooze).Before(job.Deadline)
	if meta.Snoozes >= unknownKindMaxSnoozes || expired {
		q.logger.Error("no handler registered for kind, dropping", "kind", job.Kind, "job_id", rj.ID)
		return river.JobCancel(fmt.Errorf("jobs: no handler registered for kind %q", job.Kind))
	}
	q.logger.Warn("no handler registered for kind, retrying later", "kind", job.Kind, "job_id", rj.ID)
	return river.JobSnooze(unknownKindSnooze)
}

// riverErrorHandler logs handler panics. River recovers them and counts the
// attempt as failed; ordinary errors are logged by [pgQueue.work].
type riverErrorHandler struct{ logger *slog.Logger }

// HandleError implements river.ErrorHandler.
func (*riverErrorHandler) HandleError(context.Context, *rivertype.JobRow, error) *river.ErrorHandlerResult {
	return nil
}

// HandlePanic implements river.ErrorHandler.
func (h *riverErrorHandler) HandlePanic(
	_ context.Context, job *rivertype.JobRow, panicVal any, trace string,
) *river.ErrorHandlerResult {
	h.logger.Error("job handler panicked",
		"job_id", job.ID, "attempt", job.Attempt, "panic", panicSummary(panicVal), "stack", trace)
	return nil
}
