// Package pgschedstate is the PostgreSQL-backed [schedulerstate.Store].
//
// It exists for the multi-process deployment docs/postgres-backend.md
// describes: several rela-server processes against one database. Each task and
// each run is its own row, so outcomes for different tasks never touch the same
// row, and the one-active-run rule is a unique index that holds across
// processes.
//
// # Tables
//
// scheduler_tasks, scheduler_runs and scheduler_run_children are created by the
// pgstore migration ladder (0017_scheduler_runs.sql), in the tenant's schema.
// This package never creates or migrates them, and it does not import
// internal/store: it takes an injected handle, like pgcomments.
//
// # Locking
//
// Every write that ends a run locks the run row, then its task row. Run
// creation locks only the task row, and checks for an active run with a plain
// read before inserting, so it never waits on a run another transaction is
// ending. That keeps the two orders from deadlocking.
package pgschedstate

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync/atomic"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/Sourcehaven-BV/rela/internal/schedulerstate"
)

// DBTX is the subset of a pgx pool this package needs.
type DBTX interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Begin(ctx context.Context) (pgx.Tx, error)
}

// querier is what the helpers below run against: the pool or a transaction.
type querier interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// Store is the PostgreSQL [schedulerstate.Store].
type Store struct {
	db DBTX
	// closed is set by Close. It does not close the pool, which the wiring
	// site owns.
	closed atomic.Bool
}

// New returns a Store over db.
//
// Nil: rejected — a nil handle is a wiring bug, and every task would look
// permanently due.
func New(db DBTX) (*Store, error) {
	if db == nil {
		return nil, errors.New("pgschedstate: db must not be nil")
	}
	return &Store{db: db}, nil
}

// compile-time check
var _ schedulerstate.Store = (*Store)(nil)

const runColumns = `id, task, status, created_at, started_at, finished_at, lease_until,
	node, error, occurrence, children, children_settled, children_failed`

const activeStatuses = `('queued', 'running')`

// Load implements [schedulerstate.Store].
func (s *Store) Load(ctx context.Context, tasks []string) (map[string]schedulerstate.TaskState, error) {
	if s.closed.Load() {
		return nil, schedulerstate.ErrClosed
	}
	out := make(map[string]schedulerstate.TaskState, len(tasks))

	rows, err := s.db.Query(ctx, `
		SELECT task, last_run, failures, next_retry, version
		FROM scheduler_tasks WHERE task = ANY($1)`, tasks)
	if err != nil {
		return nil, fmt.Errorf("pgschedstate: load tasks: %w", err)
	}
	for rows.Next() {
		var (
			name               string
			lastRun, nextRetry *time.Time
			ts                 schedulerstate.TaskState
		)
		if scanErr := rows.Scan(&name, &lastRun, &ts.Failures, &nextRetry, &ts.Version); scanErr != nil {
			rows.Close()
			return nil, fmt.Errorf("pgschedstate: scan task: %w", scanErr)
		}
		ts.LastRun, ts.NextRetry = timeOf(lastRun), timeOf(nextRetry)
		out[name] = ts
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("pgschedstate: load tasks: %w", err)
	}

	runs, err := queryRuns(ctx, s.db, `SELECT `+runColumns+` FROM scheduler_runs
		WHERE task = ANY($1) AND status IN `+activeStatuses, tasks)
	if err != nil {
		return nil, err
	}
	for _, run := range runs {
		ts := out[run.Task]
		ts.Active = &run
		out[run.Task] = ts
	}
	return out, nil
}

// Seed implements [schedulerstate.Store].
func (s *Store) Seed(ctx context.Context, task string, st schedulerstate.TaskState) error {
	if s.closed.Load() {
		return schedulerstate.ErrClosed
	}
	if task == "" {
		return schedulerstate.ErrNoTask
	}
	touched := st.LastRun
	if st.NextRetry.After(touched) {
		touched = st.NextRetry
	}
	_, err := s.db.Exec(ctx, `
		INSERT INTO scheduler_tasks (task, last_run, failures, next_retry, version, touched_at)
		VALUES ($1, $2, $3, $4, 0, $5)
		ON CONFLICT (task) DO NOTHING`,
		task, nullTime(st.LastRun), st.Failures, nullTime(st.NextRetry), touched)
	if err != nil {
		return fmt.Errorf("pgschedstate: seed %q: %w", task, err)
	}
	return nil
}

// CreateRun implements [schedulerstate.Store].
func (s *Store) CreateRun(ctx context.Context, run schedulerstate.Run, expectVersion int64) error {
	if s.closed.Load() {
		return schedulerstate.ErrClosed
	}
	if run.Task == "" {
		return schedulerstate.ErrNoTask
	}
	if run.ID == "" {
		return schedulerstate.ErrNoRun
	}
	return s.inTx(ctx, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `
			INSERT INTO scheduler_tasks (task, touched_at) VALUES ($1, $2)
			ON CONFLICT (task) DO NOTHING`, run.Task, run.CreatedAt); err != nil {
			return fmt.Errorf("pgschedstate: ensure task %q: %w", run.Task, err)
		}
		var version int64
		if err := tx.QueryRow(ctx, `SELECT version FROM scheduler_tasks WHERE task = $1 FOR UPDATE`,
			run.Task).Scan(&version); err != nil {
			return fmt.Errorf("pgschedstate: lock task %q: %w", run.Task, err)
		}

		// A plain read, before the insert: see the package doc on locking.
		var active bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM scheduler_runs
			WHERE task = $1 AND status IN `+activeStatuses+`)`, run.Task).Scan(&active); err != nil {
			return fmt.Errorf("pgschedstate: check active run %q: %w", run.Task, err)
		}
		if active {
			return schedulerstate.ErrRunActive
		}
		if version != expectVersion {
			return schedulerstate.ErrStale
		}

		_, err := tx.Exec(ctx, `
			INSERT INTO scheduler_runs (id, task, status, created_at, lease_until, occurrence)
			VALUES ($1, $2, 'queued', $3, $4, $5)`, run.ID, run.Task, run.CreatedAt, run.LeaseUntil, run.Occurrence)
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "scheduler_runs_one_active" {
			return schedulerstate.ErrRunActive
		}
		if err != nil {
			return fmt.Errorf("pgschedstate: create run %q: %w", run.ID, err)
		}
		return nil
	})
}

// StartRun implements [schedulerstate.Store].
func (s *Store) StartRun(
	ctx context.Context, id, node string, now, leaseUntil time.Time,
) (schedulerstate.Run, error) {
	if s.closed.Load() {
		return schedulerstate.Run{}, schedulerstate.ErrClosed
	}
	runs, err := queryRuns(ctx, s.db, `
		UPDATE scheduler_runs SET status = 'running', started_at = $2, node = $3, lease_until = $4
		WHERE id = $1 AND status = 'queued'
		RETURNING `+runColumns, id, now, node, leaseUntil)
	if err != nil {
		return schedulerstate.Run{}, err
	}
	if len(runs) == 1 {
		return runs[0], nil
	}
	current, err := getRun(ctx, s.db, id, false)
	if err != nil {
		return schedulerstate.Run{}, err
	}
	return current, schedulerstate.ErrNotQueued
}

// ExtendLease implements [schedulerstate.Store].
func (s *Store) ExtendLease(ctx context.Context, id string, leaseUntil time.Time) error {
	if s.closed.Load() {
		return schedulerstate.ErrClosed
	}
	tag, err := s.db.Exec(ctx, `
		UPDATE scheduler_runs SET lease_until = $2
		WHERE id = $1 AND status IN `+activeStatuses+` AND lease_until < $2`, id, leaseUntil)
	if err != nil {
		return fmt.Errorf("pgschedstate: extend lease %q: %w", id, err)
	}
	if tag.RowsAffected() == 0 {
		_, err := getRun(ctx, s.db, id, false)
		return err
	}
	return nil
}

// ExpectChildren implements [schedulerstate.Store].
func (s *Store) ExpectChildren(ctx context.Context, id string, subjects []string) error {
	if s.closed.Load() {
		return schedulerstate.ErrClosed
	}
	return s.inTx(ctx, func(tx pgx.Tx) error {
		if _, err := getRun(ctx, tx, id, true); err != nil {
			return err
		}
		tag, err := tx.Exec(ctx, `
			INSERT INTO scheduler_run_children (run_id, subject)
			SELECT $1, unnest($2::text[])
			ON CONFLICT DO NOTHING`, id, subjects)
		if err != nil {
			return fmt.Errorf("pgschedstate: expect children of %q: %w", id, err)
		}
		if _, err := tx.Exec(ctx, `UPDATE scheduler_runs SET children = children + $2 WHERE id = $1`,
			id, tag.RowsAffected()); err != nil {
			return fmt.Errorf("pgschedstate: count children of %q: %w", id, err)
		}
		return nil
	})
}

// SettleChild implements [schedulerstate.Store].
func (s *Store) SettleChild(
	ctx context.Context, id, subject string, out schedulerstate.Outcome, policy schedulerstate.RetryPolicy,
) (bool, *schedulerstate.Finished, error) {
	if s.closed.Load() {
		return false, nil, schedulerstate.ErrClosed
	}
	var (
		settled bool
		done    *schedulerstate.Finished
	)
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		if _, err := getRun(ctx, tx, id, true); err != nil {
			return err
		}
		tag, err := tx.Exec(ctx, `
			UPDATE scheduler_run_children SET settled = true, error = $3
			WHERE run_id = $1 AND subject = $2 AND NOT settled`, id, subject, out.Error)
		if err != nil {
			return fmt.Errorf("pgschedstate: settle %q/%q: %w", id, subject, err)
		}
		if tag.RowsAffected() == 0 {
			var known bool
			if err = tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM scheduler_run_children
				WHERE run_id = $1 AND subject = $2)`, id, subject).Scan(&known); err != nil {
				return fmt.Errorf("pgschedstate: settle %q/%q: %w", id, subject, err)
			}
			if !known {
				return fmt.Errorf("pgschedstate: run %q does not expect subject %q", id, subject)
			}
			return nil
		}
		settled = true

		failed := 0
		if out.Error != "" {
			failed = 1
		}
		runs, err := queryRuns(ctx, tx, `
			UPDATE scheduler_runs SET
				children_settled = children_settled + 1,
				children_failed = children_failed + $2,
				error = CASE WHEN error = '' THEN $3 ELSE error END
			WHERE id = $1
			RETURNING `+runColumns, id, failed, out.Error)
		if err != nil {
			return err
		}
		run := runs[0]
		if run.ChildrenSettled < run.Children || !run.Status.Active() {
			return nil
		}
		fin, err := finish(ctx, tx, run, schedulerstate.ChildOutcome(run, run.Error, out.At), run.Status, policy)
		if err != nil {
			return err
		}
		done = &fin
		return nil
	})
	if err != nil {
		return false, nil, err
	}
	return settled, done, nil
}

// SucceededSubjects implements [schedulerstate.Store].
func (s *Store) SucceededSubjects(ctx context.Context, task, occurrence string) ([]string, error) {
	if s.closed.Load() {
		return nil, schedulerstate.ErrClosed
	}
	rows, err := s.db.Query(ctx, `
		SELECT DISTINCT c.subject
		FROM scheduler_run_children c JOIN scheduler_runs r ON r.id = c.run_id
		WHERE r.task = $1 AND r.occurrence = $2 AND c.settled AND c.error = ''
		ORDER BY c.subject`, task, occurrence)
	if err != nil {
		return nil, fmt.Errorf("pgschedstate: succeeded subjects of %q: %w", task, err)
	}
	out, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return nil, fmt.Errorf("pgschedstate: succeeded subjects of %q: %w", task, err)
	}
	return out, nil
}

// FinishRun implements [schedulerstate.Store].
func (s *Store) FinishRun(
	ctx context.Context, id string, out schedulerstate.Outcome, policy schedulerstate.RetryPolicy,
) (schedulerstate.Finished, error) {
	if s.closed.Load() {
		return schedulerstate.Finished{}, schedulerstate.ErrClosed
	}
	var fin schedulerstate.Finished
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		run, err := getRun(ctx, tx, id, true)
		if err != nil {
			return err
		}
		if !run.Status.Active() {
			fin = schedulerstate.Finished{Run: run}
			return nil
		}
		fin, err = finish(ctx, tx, run, out, run.Status, policy)
		return err
	})
	return fin, err
}

// Reap implements [schedulerstate.Store].
func (s *Store) Reap(
	ctx context.Context, now time.Time, policy schedulerstate.RetryPolicy,
) ([]schedulerstate.Finished, error) {
	if s.closed.Load() {
		return nil, schedulerstate.ErrClosed
	}
	var out []schedulerstate.Finished
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		// SKIP LOCKED: a run another transaction is ending right now is not
		// expired in any sense that matters, and another reaper that got
		// there first owns it.
		expired, err := queryRuns(ctx, tx, `SELECT `+runColumns+` FROM scheduler_runs
			WHERE status IN `+activeStatuses+` AND lease_until < $1
			ORDER BY id FOR UPDATE SKIP LOCKED`, now)
		if err != nil {
			return err
		}
		for _, run := range expired {
			msg := fmt.Sprintf("lease expired while %s", run.Status)
			fin, err := finish(ctx, tx, run, schedulerstate.Outcome{Error: msg, At: now},
				schedulerstate.RunAbandoned, policy)
			if err != nil {
				return err
			}
			out = append(out, fin)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// Prune implements [schedulerstate.Store].
func (s *Store) Prune(ctx context.Context, before time.Time) ([]string, error) {
	if s.closed.Load() {
		return nil, schedulerstate.ErrClosed
	}
	var removed []string
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `DELETE FROM scheduler_runs
			WHERE status NOT IN `+activeStatuses+` AND finished_at < $1`, before); err != nil {
			return fmt.Errorf("pgschedstate: prune runs: %w", err)
		}
		rows, err := tx.Query(ctx, `
			DELETE FROM scheduler_tasks t
			WHERE touched_at < $1 AND NOT EXISTS (
				SELECT 1 FROM scheduler_runs r WHERE r.task = t.task AND r.status IN `+activeStatuses+`)
			RETURNING task`, before)
		if err != nil {
			return fmt.Errorf("pgschedstate: prune tasks: %w", err)
		}
		removed, err = pgx.CollectRows(rows, pgx.RowTo[string])
		if err != nil {
			return fmt.Errorf("pgschedstate: prune tasks: %w", err)
		}
		// The run table may still hold ended runs of a pruned task that
		// finished after the cut-off; they go with their task.
		if _, err := tx.Exec(ctx, `DELETE FROM scheduler_runs WHERE task = ANY($1)`, removed); err != nil {
			return fmt.Errorf("pgschedstate: prune runs of removed tasks: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	slices.Sort(removed)
	return removed, nil
}

// Close implements [schedulerstate.Store]. Idempotent; the pool stays open.
func (s *Store) Close() error {
	s.closed.Store(true)
	return nil
}

// finish ends run and applies the outcome to its task inside tx. The run row
// must already be locked by the caller.
func finish(
	ctx context.Context, tx pgx.Tx, run schedulerstate.Run, out schedulerstate.Outcome,
	status schedulerstate.RunStatus, policy schedulerstate.RetryPolicy,
) (schedulerstate.Finished, error) {
	switch {
	case status == schedulerstate.RunAbandoned:
	case out.Error == "":
		status = schedulerstate.RunSucceeded
	default:
		status = schedulerstate.RunFailed
	}
	if _, err := tx.Exec(ctx, `UPDATE scheduler_runs SET status = $2, finished_at = $3, error = $4 WHERE id = $1`,
		run.ID, string(status), out.At, out.Error); err != nil {
		return schedulerstate.Finished{}, fmt.Errorf("pgschedstate: finish run %q: %w", run.ID, err)
	}
	run.Status, run.FinishedAt, run.Error = status, out.At, out.Error

	var (
		ts                 schedulerstate.TaskState
		lastRun, nextRetry *time.Time
	)
	if err := tx.QueryRow(ctx, `SELECT last_run, failures, next_retry, version
		FROM scheduler_tasks WHERE task = $1 FOR UPDATE`, run.Task).
		Scan(&lastRun, &ts.Failures, &nextRetry, &ts.Version); err != nil {
		return schedulerstate.Finished{}, fmt.Errorf("pgschedstate: lock task %q: %w", run.Task, err)
	}
	ts.LastRun, ts.NextRetry = timeOf(lastRun), timeOf(nextRetry)

	next, changed := schedulerstate.ApplyOutcome(ts, run.CreatedAt, out, policy)
	if changed {
		if _, err := tx.Exec(ctx, `
			UPDATE scheduler_tasks SET last_run = $2, failures = $3, next_retry = $4, version = $5, touched_at = $6
			WHERE task = $1`,
			run.Task, nullTime(next.LastRun), next.Failures, nullTime(next.NextRetry), next.Version, out.At,
		); err != nil {
			return schedulerstate.Finished{}, fmt.Errorf("pgschedstate: record outcome of %q: %w", run.Task, err)
		}
	}

	fin := schedulerstate.Finished{Run: run, Applied: true}
	if out.Error != "" {
		fin.Failures = next.Failures
		fin.NextRetry = next.NextRetry
	}
	return fin, nil
}

// getRun reads one run, locking it when forUpdate is set.
func getRun(ctx context.Context, q querier, id string, forUpdate bool) (schedulerstate.Run, error) {
	query := `SELECT ` + runColumns + ` FROM scheduler_runs WHERE id = $1`
	if forUpdate {
		query += ` FOR UPDATE`
	}
	runs, err := queryRuns(ctx, q, query, id)
	if err != nil {
		return schedulerstate.Run{}, err
	}
	if len(runs) == 0 {
		return schedulerstate.Run{}, schedulerstate.ErrNoRun
	}
	return runs[0], nil
}

// queryRuns runs a query returning runColumns.
func queryRuns(ctx context.Context, q querier, sql string, args ...any) ([]schedulerstate.Run, error) {
	rows, err := q.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("pgschedstate: query runs: %w", err)
	}
	defer rows.Close()

	var out []schedulerstate.Run
	for rows.Next() {
		var (
			r                   schedulerstate.Run
			status              string
			started, finishedAt *time.Time
		)
		if err := rows.Scan(&r.ID, &r.Task, &status, &r.CreatedAt, &started, &finishedAt, &r.LeaseUntil,
			&r.Node, &r.Error, &r.Occurrence, &r.Children, &r.ChildrenSettled, &r.ChildrenFailed); err != nil {
			return nil, fmt.Errorf("pgschedstate: scan run: %w", err)
		}
		r.Status = schedulerstate.RunStatus(status)
		r.StartedAt, r.FinishedAt = timeOf(started), timeOf(finishedAt)
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("pgschedstate: query runs: %w", err)
	}
	return out, nil
}

// inTx runs fn in a transaction, committing when it returns nil.
func (s *Store) inTx(ctx context.Context, fn func(pgx.Tx) error) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("pgschedstate: begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := fn(tx); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("pgschedstate: commit: %w", err)
	}
	return nil
}

// nullTime maps the zero time to SQL NULL.
func nullTime(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}

// timeOf maps SQL NULL to the zero time.
func timeOf(t *time.Time) time.Time {
	if t == nil {
		return time.Time{}
	}
	return *t
}
