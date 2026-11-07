// Package sqlflowrun is the one implementation of [flowrun.Store] over a
// SQL database. pgflowrun and sqliteflowrun adapt their driver to
// [sqlexec.DB] and pick a [Dialect]; the queries and the locking order
// live here once, as sqlchannels does for channels, so the two database
// backends cannot drift apart.
//
// # Schema
//
// Two tables, created by the pgstore and sqlitedb migrations:
//
//   - flow_definitions: one row per (flow, version), the definition
//     bytes, their SHA-256 digest and when the first run stored them.
//   - flow_runs: one row per run. The run's execution state is the JSON
//     doc column ([flowrun.EncodeState]); its due time is a column too,
//     for the due index. A foreign key ties each run to its definition,
//     and a unique index allows one singleton run per flow and subject
//     id, as backstops to the locking below.
//
// Times are BIGINT microseconds since the epoch, 0 for none, so both
// databases compare and order them alike. Ids sort bytewise (COLLATE "C"
// on PostgreSQL, SQLite's default), as Go strings do.
//
// # Locking
//
// Update and Delete are one compare-and-set statement on rev each. Create
// takes the dialect's definitions lock shared and then the lock of the
// subject id, so the id, singleton and cap checks, the definition
// snapshot and the insert are one step, and creates on other subjects do
// not wait on each other. PruneDefinitions takes the definitions lock
// exclusive, so no create is between its checks and its insert while a
// prune deletes. RekeySubject takes the locks of the old and the new
// subject id, in sorted order, before it reads: a create on either id,
// of any flow, waits for it. The lock is per subject id rather than per
// flow and subject id for that reason. SQLite needs no locks: its write
// transaction already excludes every other writer.
package sqlflowrun

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/flow"
	"github.com/Sourcehaven-BV/rela/internal/flowrun"
	"github.com/Sourcehaven-BV/rela/internal/sqlexec"
)

// Dialect holds what differs between the databases. An empty statement
// means the transaction itself excludes every other writer.
type Dialect struct {
	// Name prefixes error messages, e.g. "pgflowrun".
	Name string
	// LockDefinitionsShared and LockDefinitions take the definitions lock
	// shared and exclusive for the rest of the transaction. No arguments.
	LockDefinitionsShared string
	LockDefinitions       string
	// LockSubject takes the lock of one subject id for the rest of the
	// transaction. One `?` argument: the subject id.
	LockSubject string
}

// Store is the SQL backend.
type Store struct {
	db sqlexec.DB
	d  Dialect
}

var _ flowrun.Store = (*Store)(nil)

// New returns a store over db.
//
// Nil: db rejected, so a wiring mistake fails here rather than at the
// first run.
func New(db sqlexec.DB, d Dialect) (*Store, error) {
	if db == nil {
		return nil, fmt.Errorf("%s: New requires a database handle", d.Name)
	}
	return &Store{db: db, d: d}, nil
}

const runColumns = `id, flow, version, subject_id, subject_face, singleton, created_at, updated_at, rev, doc`

func micros(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return flow.NormTime(t).UnixMicro()
}

func fromMicros(v int64) time.Time {
	if v == 0 {
		return time.Time{}
	}
	return time.UnixMicro(v).UTC()
}

func boolInt(b bool) int64 {
	if b {
		return 1
	}
	return 0
}

func (s *Store) errf(format string, args ...any) error {
	return fmt.Errorf("%s: "+format, append([]any{s.d.Name}, args...)...)
}

func (s *Store) lock(ctx context.Context, q sqlexec.Querier, stmt string, args ...any) error {
	if stmt == "" {
		return nil
	}
	if _, err := q.Exec(ctx, stmt, args...); err != nil {
		return s.errf("lock: %w", err)
	}
	return nil
}

func (s *Store) scanRun(row sqlexec.Row) (flowrun.Run, error) {
	var (
		r                                 flowrun.Run
		id, face, doc                     string
		singleton, created, updated, revv int64
	)
	if err := row.Scan(&id, &r.Flow, &r.Version, &r.Subject.ID, &face, &singleton, &created, &updated,
		&revv, &doc); err != nil {
		return flowrun.Run{}, err
	}
	inst, err := flowrun.DecodeState([]byte(doc))
	if err != nil {
		return flowrun.Run{}, s.errf("run %s: %w", id, err)
	}
	r.ID, r.Subject.Face, r.Singleton = flowrun.RunID(id), entity.Face(face), singleton != 0
	r.CreatedAt, r.UpdatedAt, r.Rev, r.Instance = fromMicros(created), fromMicros(updated), revv, inst
	return r, nil
}

func (s *Store) queryRuns(ctx context.Context, q sqlexec.Querier, query string, args ...any) ([]flowrun.Run, error) {
	rows, err := q.Query(ctx, query, args...)
	if err != nil {
		return nil, s.errf("query runs: %w", err)
	}
	defer rows.Close()
	out := []flowrun.Run{}
	for rows.Next() {
		r, err := s.scanRun(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, s.errf("query runs: %w", err)
	}
	return out, nil
}

func (s *Store) getRun(ctx context.Context, q sqlexec.Querier, id flowrun.RunID) (flowrun.Run, error) {
	r, err := s.scanRun(q.QueryRow(ctx, `SELECT `+runColumns+` FROM flow_runs WHERE id = ?`, string(id)))
	if errors.Is(err, sqlexec.ErrNoRows) {
		return flowrun.Run{}, s.errf("run %s: %w", id, flowrun.ErrNotFound)
	}
	if err != nil {
		return flowrun.Run{}, s.errf("get run %s: %w", id, err)
	}
	return r, nil
}

// subjectRuns returns how many runs of flowName are stored on subjectID
// and whether one of them is a singleton.
func (s *Store) subjectRuns(ctx context.Context, q sqlexec.Querier, flowName, subjectID string) (
	count int, singleton bool, err error) {
	var n, singles int64
	err = q.QueryRow(ctx,
		`SELECT COUNT(*), COALESCE(SUM(singleton), 0) FROM flow_runs WHERE flow = ? AND subject_id = ?`,
		flowName, subjectID).Scan(&n, &singles)
	if err != nil {
		return 0, false, s.errf("count subject runs: %w", err)
	}
	return int(n), singles > 0, nil
}

// Create implements [flowrun.Store.Create].
func (s *Store) Create(ctx context.Context, nr flowrun.NewRun) (flowrun.Run, error) {
	if err := flowrun.CheckNewRun(nr); err != nil {
		return flowrun.Run{}, err
	}
	run := flowrun.FromNew(nr)
	doc, encErr := flowrun.EncodeState(run.Instance)
	if encErr != nil {
		return flowrun.Run{}, s.errf("encode state: %w", encErr)
	}
	err := s.db.Transact(ctx, func(q sqlexec.Querier) error {
		if err := s.lock(ctx, q, s.d.LockDefinitionsShared); err != nil {
			return err
		}
		if err := s.lock(ctx, q, s.d.LockSubject, nr.Subject.ID); err != nil {
			return err
		}
		var one int
		switch err := q.QueryRow(ctx, `SELECT 1 FROM flow_runs WHERE id = ?`, string(nr.ID)).Scan(&one); {
		case err == nil:
			return s.errf("run %s: %w", nr.ID, flowrun.ErrExists)
		case !errors.Is(err, sqlexec.ErrNoRows):
			return s.errf("check run %s: %w", nr.ID, err)
		}
		n, single, countErr := s.subjectRuns(ctx, q, nr.Flow, nr.Subject.ID)
		if countErr != nil {
			return countErr
		}
		if nr.Singleton && single {
			return s.errf("%s on %s: %w", nr.Flow, nr.Subject.ID, flowrun.ErrSubjectBusy)
		}
		if n >= flowrun.MaxRunsPerSubject {
			return s.errf("%s on %s: %w", nr.Flow, nr.Subject.ID, flowrun.ErrSubjectFull)
		}
		if err := s.storeDefinition(ctx, q, nr, run.CreatedAt); err != nil {
			return err
		}
		inserted, insErr := q.Exec(ctx, `INSERT INTO flow_runs (`+runColumns+`, next_due_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, 1, ?, ?) ON CONFLICT (id) DO NOTHING`,
			string(nr.ID), nr.Flow, nr.Version, nr.Subject.ID, string(nr.Subject.Face), boolInt(nr.Singleton),
			micros(run.CreatedAt), micros(run.UpdatedAt), string(doc), micros(run.Instance.NextDueAt))
		if insErr != nil {
			return s.errf("insert run %s: %w", nr.ID, insErr)
		}
		if inserted == 0 {
			// A create of the same id on another subject committed first.
			return s.errf("run %s: %w", nr.ID, flowrun.ErrExists)
		}
		return nil
	})
	if err != nil {
		return flowrun.Run{}, err
	}
	return run, nil
}

// storeDefinition stores nr's definition when its version has none, or
// checks it against the stored one.
func (s *Store) storeDefinition(ctx context.Context, q sqlexec.Querier, nr flowrun.NewRun, at time.Time) error {
	digest := flowrun.Digest(nr.Definition)
	inserted, err := q.Exec(ctx, `INSERT INTO flow_definitions (flow, version, def, digest, saved_at)
		VALUES (?, ?, ?, ?, ?) ON CONFLICT (flow, version) DO NOTHING`,
		nr.Flow, nr.Version, nr.Definition, digest, micros(at))
	if err != nil {
		return s.errf("insert definition: %w", err)
	}
	if inserted == 1 {
		return nil
	}
	var held string
	if err := q.QueryRow(ctx, `SELECT digest FROM flow_definitions WHERE flow = ? AND version = ?`,
		nr.Flow, nr.Version).Scan(&held); err != nil {
		return s.errf("read definition: %w", err)
	}
	if held != digest {
		return s.errf("%s/%s: %w", nr.Flow, nr.Version, flowrun.ErrDefinitionConflict)
	}
	return nil
}

// Get implements [flowrun.Store.Get].
func (s *Store) Get(ctx context.Context, id flowrun.RunID) (flowrun.Run, error) {
	if err := flowrun.CheckID(id); err != nil {
		return flowrun.Run{}, err
	}
	return s.getRun(ctx, s.db, id)
}

// missOrConflict tells why a compare-and-set on id matched no row.
func (s *Store) missOrConflict(ctx context.Context, q sqlexec.Querier, id flowrun.RunID) error {
	var rev int64
	err := q.QueryRow(ctx, `SELECT rev FROM flow_runs WHERE id = ?`, string(id)).Scan(&rev)
	switch {
	case errors.Is(err, sqlexec.ErrNoRows):
		return s.errf("run %s: %w", id, flowrun.ErrNotFound)
	case err != nil:
		return s.errf("read run %s: %w", id, err)
	}
	return s.errf("run %s: %w", id, flowrun.ErrConflict)
}

// Update implements [flowrun.Store.Update].
func (s *Store) Update(ctx context.Context, id flowrun.RunID, expectRev int64, inst flow.Instance,
	at time.Time) (flowrun.Run, error) {
	if err := flowrun.CheckUpdate(id, expectRev, inst, at); err != nil {
		return flowrun.Run{}, err
	}
	inst = flowrun.NormInstance(inst)
	doc, encErr := flowrun.EncodeState(inst)
	if encErr != nil {
		return flowrun.Run{}, s.errf("encode state: %w", encErr)
	}
	var out flowrun.Run
	err := s.db.Transact(ctx, func(q sqlexec.Querier) error {
		n, err := q.Exec(ctx, `UPDATE flow_runs SET doc = ?, next_due_at = ?, updated_at = ?,
			rev = rev + 1 WHERE id = ? AND rev = ?`,
			string(doc), micros(inst.NextDueAt), micros(at), string(id), expectRev)
		if err != nil {
			return s.errf("update run %s: %w", id, err)
		}
		if n == 0 {
			return s.missOrConflict(ctx, q, id)
		}
		// The row stays locked until commit, so this reads the update.
		out, err = s.getRun(ctx, q, id)
		return err
	})
	return out, err
}

// Delete implements [flowrun.Store.Delete].
func (s *Store) Delete(ctx context.Context, id flowrun.RunID, expectRev int64) error {
	if err := flowrun.CheckDelete(id, expectRev); err != nil {
		return err
	}
	n, err := s.db.Exec(ctx, `DELETE FROM flow_runs WHERE id = ? AND rev = ?`, string(id), expectRev)
	if err != nil {
		return s.errf("delete run %s: %w", id, err)
	}
	if n == 0 {
		return s.missOrConflict(ctx, s.db, id)
	}
	return nil
}

// ListDue implements [flowrun.Store.ListDue].
func (s *Store) ListDue(ctx context.Context, until time.Time, after flowrun.DueCursor,
	limit int) ([]flowrun.Run, error) {
	if err := flowrun.CheckDue(until, after, limit); err != nil {
		return nil, err
	}
	afterAt := micros(after.At)
	return s.queryRuns(ctx, s.db, `SELECT `+runColumns+` FROM flow_runs
		WHERE next_due_at > 0 AND next_due_at <= ? AND (next_due_at > ? OR (next_due_at = ? AND id > ?))
		ORDER BY next_due_at, id LIMIT ?`,
		micros(until), afterAt, afterAt, string(after.ID), limit)
}

// List implements [flowrun.Store.List].
func (s *Store) List(ctx context.Context, q flowrun.Query) ([]flowrun.Run, error) {
	if err := flowrun.CheckQuery(q); err != nil {
		return nil, err
	}
	where := []string{"id > ?"}
	args := []any{string(q.After)}
	if q.Flow != "" {
		where = append(where, "flow = ?")
		args = append(args, q.Flow)
	}
	if q.SubjectID != "" {
		where = append(where, "subject_id = ?")
		args = append(args, q.SubjectID)
	}
	args = append(args, q.Limit)
	return s.queryRuns(ctx, s.db, `SELECT `+runColumns+` FROM flow_runs WHERE `+
		strings.Join(where, " AND ")+` ORDER BY id LIMIT ?`, args...)
}

// RekeySubject implements [flowrun.Store.RekeySubject].
func (s *Store) RekeySubject(ctx context.Context, oldID, newID string) (int, error) {
	if err := flowrun.CheckRekey(oldID, newID); err != nil {
		return 0, err
	}
	if oldID == newID {
		return 0, nil
	}
	moved := 0
	err := s.db.Transact(ctx, func(q sqlexec.Querier) error {
		keys := []string{oldID, newID}
		slices.Sort(keys)
		for _, k := range keys {
			if err := s.lock(ctx, q, s.d.LockSubject, k); err != nil {
				return err
			}
		}
		joining, err := s.joining(ctx, q, oldID)
		if err != nil || len(joining) == 0 {
			return err
		}
		flows := slices.Sorted(maps.Keys(joining))
		for _, flowName := range flows {
			j := joining[flowName]
			n, single, countErr := s.subjectRuns(ctx, q, flowName, newID)
			if countErr != nil {
				return countErr
			}
			if single && j.singles > 0 {
				return s.errf("%s on %s: %w", flowName, newID, flowrun.ErrSubjectBusy)
			}
			if n+j.n > flowrun.MaxRunsPerSubject {
				return s.errf("%s on %s: %w", flowName, newID, flowrun.ErrSubjectFull)
			}
		}
		n, err := q.Exec(ctx, `UPDATE flow_runs SET subject_id = ? WHERE subject_id = ?`, newID, oldID)
		if err != nil {
			return s.errf("rekey runs: %w", err)
		}
		moved = int(n)
		return nil
	})
	if err != nil {
		return 0, err
	}
	return moved, nil
}

type joiningRuns struct{ n, singles int }

// joining returns, per flow, how many runs subjectID holds and how many
// of them are singletons.
func (s *Store) joining(ctx context.Context, q sqlexec.Querier, subjectID string) (map[string]joiningRuns, error) {
	rows, err := q.Query(ctx, `SELECT flow, COUNT(*), COALESCE(SUM(singleton), 0) FROM flow_runs
		WHERE subject_id = ? GROUP BY flow`, subjectID)
	if err != nil {
		return nil, s.errf("count runs: %w", err)
	}
	defer rows.Close()
	out := map[string]joiningRuns{}
	for rows.Next() {
		var (
			flowName  string
			n, single int64
		)
		if err := rows.Scan(&flowName, &n, &single); err != nil {
			return nil, s.errf("count runs: %w", err)
		}
		out[flowName] = joiningRuns{n: int(n), singles: int(single)}
	}
	if err := rows.Err(); err != nil {
		return nil, s.errf("count runs: %w", err)
	}
	return out, nil
}

// Definition implements [flowrun.Store.Definition].
func (s *Store) Definition(ctx context.Context, flowName, version string) ([]byte, error) {
	if err := flowrun.CheckDefinitionKey(flowName, version); err != nil {
		return nil, err
	}
	var def []byte
	err := s.db.QueryRow(ctx, `SELECT def FROM flow_definitions WHERE flow = ? AND version = ?`,
		flowName, version).Scan(&def)
	if errors.Is(err, sqlexec.ErrNoRows) {
		return nil, s.errf("definition %s/%s: %w", flowName, version, flowrun.ErrNotFound)
	}
	if err != nil {
		return nil, s.errf("read definition: %w", err)
	}
	return def, nil
}

// PruneDefinitions implements [flowrun.Store.PruneDefinitions].
func (s *Store) PruneDefinitions(ctx context.Context, before time.Time) (int, error) {
	if err := flowrun.CheckPrune(before); err != nil {
		return 0, err
	}
	dropped := 0
	err := s.db.Transact(ctx, func(q sqlexec.Querier) error {
		if err := s.lock(ctx, q, s.d.LockDefinitions); err != nil {
			return err
		}
		n, err := q.Exec(ctx, `DELETE FROM flow_definitions WHERE saved_at < ? AND NOT EXISTS (
			SELECT 1 FROM flow_runs r WHERE r.flow = flow_definitions.flow AND r.version = flow_definitions.version)`,
			micros(before))
		if err != nil {
			return s.errf("prune definitions: %w", err)
		}
		dropped = int(n)
		return nil
	})
	if err != nil {
		return 0, err
	}
	return dropped, nil
}
