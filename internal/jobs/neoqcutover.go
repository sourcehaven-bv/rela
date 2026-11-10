//go:build postgres

package jobs

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivermigrate"
)

// This file prepares a schema for the River queue: it migrates River's
// tables, and once moves the jobs a neoq build left pending into River.
//
// Both steps run under one advisory lock per schema, because rivermigrate
// takes no lock of its own and every rela process builds a queue at startup:
// two processes starting together would otherwise race the same CREATE TABLE.

// cutoverLockTimeout bounds how long the neoq cutover waits for a lock. A neoq
// process that is still running holds row locks on its in-flight jobs, and
// DROP TABLE would queue behind them, holding up this process's startup and
// every neoq query after it. Giving up is safe: the cutover runs again at the
// next start.
const cutoverLockTimeout = "5s"

// migrateRiver brings River's tables in schema up to date.
func migrateRiver(ctx context.Context, pool *pgxpool.Pool, schema string, logger *slog.Logger) error {
	migrator, err := rivermigrate.New(riverpgxv5.New(pool), &rivermigrate.Config{
		Schema: schema,
		Logger: logger.With("component", "river"),
	})
	if err != nil {
		return fmt.Errorf("jobs: create river migrator: %w", err)
	}
	// The lock is held on its own connection, at session level, because
	// Migrate runs each migration in a transaction of its own. It is the
	// same lock lockSchemaSetup takes per transaction.
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("jobs: migrate queue tables: %w", err)
	}
	defer conn.Release()
	if _, err = conn.Exec(ctx, `SELECT pg_advisory_lock(hashtextextended($1, 0))`, setupLockKey(schema)); err != nil {
		return fmt.Errorf("jobs: lock queue setup: %w", err)
	}
	defer func() {
		unlockCtx := context.WithoutCancel(ctx)
		_, uerr := conn.Exec(unlockCtx, `SELECT pg_advisory_unlock(hashtextextended($1, 0))`, setupLockKey(schema))
		if uerr != nil {
			// Closing the session releases the lock; the pool then drops
			// the closed connection.
			_ = conn.Conn().Close(unlockCtx)
		}
	}()
	if _, err = migrator.Migrate(ctx, rivermigrate.DirectionUp, nil); err != nil {
		return fmt.Errorf("jobs: migrate queue tables: %w", err)
	}
	return nil
}

// setupLockKey is the advisory lock key that serializes queue setup in
// schema across processes.
func setupLockKey(schema string) string { return "rela-jobs-setup:" + schema }

// lockSchemaSetup takes the transaction-scoped advisory lock that serializes
// queue setup in schema across processes.
func lockSchemaSetup(ctx context.Context, tx pgx.Tx, schema string) error {
	_, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, setupLockKey(schema))
	if err != nil {
		return fmt.Errorf("jobs: lock queue setup: %w", err)
	}
	return nil
}

// cutoverFromNeoq moves pending neoq jobs into River and drops neoq's tables.
//
// It never fails the queue's construction. A cutover that cannot get its locks
// (a neoq process still running) or fails for another reason leaves the neoq
// tables in place, and the next start tries again. Until then those jobs wait;
// nothing reads the neoq tables any more.
func cutoverFromNeoq(ctx context.Context, q *pgQueue) {
	moved, err := q.moveNeoqJobs(ctx)
	switch {
	case err == nil && moved >= 0:
		q.logger.Info("jobs: moved pending neoq jobs to river and dropped the neoq tables",
			"jobs", moved, "schema", q.schema)
	case err == nil:
		// No neoq tables: the usual case.
	case isLockUnavailable(err):
		q.logger.Warn("jobs: neoq cutover deferred, a neoq process still holds its jobs; "+
			"stop every older rela process, the next start retries", "schema", q.schema)
	default:
		q.logger.Error("jobs: neoq cutover failed, the next start retries", "schema", q.schema, "error", err)
	}
}

// moveNeoqJobs does the cutover in one transaction. It returns -1 when the
// schema has no neoq tables.
func (q *pgQueue) moveNeoqJobs(ctx context.Context) (int, error) {
	tx, err := q.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err = tx.Exec(ctx, `SET LOCAL lock_timeout = '`+cutoverLockTimeout+`'`); err != nil {
		return 0, err
	}
	err = lockSchemaSetup(ctx, tx, q.schema)
	if err != nil {
		return 0, err
	}

	table := func(name string) string { return pgx.Identifier{q.schema, name}.Sanitize() }
	var exists bool
	err = tx.QueryRow(ctx, `SELECT to_regclass($1) IS NOT NULL`, table("neoq_jobs")).Scan(&exists)
	if err != nil {
		return 0, err
	}
	if !exists {
		return -1, nil
	}

	// The table lock stops an older process from adding a job after the
	// SELECT, which DROP TABLE would then delete unmoved. It also conflicts
	// with the row lock a running neoq worker holds on its job: moving that
	// job would run it twice. NOWAIT turns either case into 55P03, and the
	// next start tries again.
	if _, err = tx.Exec(ctx, `LOCK TABLE `+table("neoq_jobs")+` IN ACCESS EXCLUSIVE MODE NOWAIT`); err != nil {
		return 0, err
	}
	rows, err := tx.Query(ctx, `SELECT fingerprint, payload, retries, run_after FROM `+table("neoq_jobs")+
		` WHERE status <> 'processed'`)
	if err != nil {
		return 0, err
	}
	var params []river.InsertManyParams
	for rows.Next() {
		var (
			fp       string
			payload  map[string]any
			retries  *int
			runAfter *time.Time
		)
		if err := rows.Scan(&fp, &payload, &retries, &runAfter); err != nil {
			rows.Close()
			return 0, err
		}
		p, ok := neoqJobParams(fp, payload, deref(retries), deref(runAfter))
		if !ok {
			q.logger.Warn("jobs: dropping a neoq job during cutover, it has no kind or no attempt left")
			continue
		}
		params = append(params, p)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}
	if len(params) > 0 {
		if _, err := q.client.InsertManyTx(ctx, tx, params); err != nil {
			return 0, fmt.Errorf("insert moved jobs: %w", err)
		}
	}

	drops := []string{
		`DROP TABLE IF EXISTS ` + table("neoq_jobs") + `, ` + table("neoq_dead_jobs") + `, ` +
			table("neoq_schema_migrations"),
		`DROP FUNCTION IF EXISTS ` + table("announce_job") + `()`,
		`DROP TYPE IF EXISTS ` + table("job_status"),
	}
	for _, stmt := range drops {
		if _, err := tx.Exec(ctx, stmt); err != nil {
			return 0, fmt.Errorf("drop neoq tables: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	return len(params), nil
}

// The payload keys rela's neoq adapter added to every job.
const (
	neoqKindKey     = "__rela_kind"
	neoqRetryKey    = "__rela_retry"
	neoqDeadlineKey = "__rela_deadline"
)

// neoqJobParams turns a pending neoq row into a River insert. It reports
// false for a row with no kind, which no handler could run, and for a row
// that used its whole budget. neoq was handed a budget no job reaches, and
// rela dropped a spent job only when neoq delivered it again, so such rows
// wait in neoq_jobs; moving one would run it once more than its policy
// allows.
//
// retries is how many attempts the job used under neoq. They count against
// its budget in River too, and the handler sees the attempt number continue.
func neoqJobParams(
	fingerprint string, payload map[string]any, retries int, runAfter time.Time,
) (river.InsertManyParams, bool) {
	kind, _ := payload[neoqKindKey].(string)
	if kind == "" {
		return river.InsertManyParams{}, false
	}
	retry := retryOf(payload)
	if retries >= retry.maxAttempts() {
		return river.InsertManyParams{}, false
	}
	args := riverArgs{RelaKind: Kind(kind), Retry: retry, AttemptOffset: retries}
	if dl, ok := deadlineOf(payload); ok {
		args.Deadline = dl.UnixNano()
	}
	args.Payload = maps.Clone(payload)
	delete(args.Payload, neoqKindKey)
	delete(args.Payload, neoqRetryKey)
	delete(args.Payload, neoqDeadlineKey)

	opts := &river.InsertOpts{
		Queue:       riverQueueName,
		MaxAttempts: retry.maxAttempts() - retries,
		ScheduledAt: runAfter,
	}
	// The neoq adapter set a keyed job's fingerprint to idempotencyID, a
	// SHA-256 in hex, and an unkeyed job's to a UUID. The hash is exactly
	// what River's uniqueness field holds.
	if isKeyHash(fingerprint) {
		args.Key = fingerprint
		opts.UniqueOpts = uniqueOpts
	}
	return river.InsertManyParams{Args: args, InsertOpts: opts}, true
}

// isKeyHash reports whether s is a SHA-256 in lowercase hex.
func isKeyHash(s string) bool {
	if len(s) != hex.EncodedLen(sha256.Size) {
		return false
	}
	_, err := hex.DecodeString(s)
	return err == nil
}

// retryOf reads the retry intent from a neoq payload. An absent or
// unrecognized value is RetryNever, the safest reading: guessing "retry it"
// could repeat a side effect the caller declared must not repeat.
func retryOf(payload map[string]any) Retry {
	r, ok := intFromPayload(payload, neoqRetryKey)
	if !ok || !Retry(r).valid() {
		return RetryNever
	}
	return Retry(r)
}

// deadlineOf reads the effective deadline from a neoq payload.
func deadlineOf(payload map[string]any) (time.Time, bool) {
	n, ok := intFromPayload(payload, neoqDeadlineKey)
	if !ok {
		return time.Time{}, false
	}
	return time.Unix(0, int64(n)), true
}

// intFromPayload reads an integer that round-tripped through JSON, where it
// comes back as float64.
func intFromPayload(payload map[string]any, key string) (int, bool) {
	switch v := payload[key].(type) {
	case int:
		return v, true
	case int64:
		return int(v), true
	case float64:
		return int(v), true
	default:
		return 0, false
	}
}

func deref[T any](p *T) T {
	var zero T
	if p == nil {
		return zero
	}
	return *p
}

// isLockUnavailable reports a lock_timeout or NOWAIT failure.
func isLockUnavailable(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "55P03"
}
