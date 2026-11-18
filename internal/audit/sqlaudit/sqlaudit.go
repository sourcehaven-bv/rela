// Package sqlaudit is the audit log kept in a SQL database: the one
// implementation over [sqlexec.DB] that pgaudit and sqliteaudit adapt to
// their driver, as sqlflowrun does for flow runs, so the two database
// backends cannot drift apart.
//
// # Schema
//
// Three tables, created by the pgstore migrations and the sqlitedb DDL, so
// they live in the tenant's schema next to the graph:
//
//   - audit_log: one row per record. seq numbers the rows without gaps,
//     at is when the row was appended (microseconds since the epoch), doc
//     is the record as JSON (audit.Prepare applied, the same bytes the
//     JSONL sink writes), and prev_hash and hash chain the rows (see below).
//   - audit_subjects: the entity ids each row names (subject, relation
//     endpoints, rename before and after), for the per-entity read.
//   - audit_anchor: at most one row, the seq and hash of the last row a
//     prune deleted, so the chain still verifies after a prune.
//
// # Hash chain
//
// Each row's hash is [ChainHash] of the previous row's hash and every other
// column of the row ([Row]), so no stored value can change unnoticed. The
// first row after an empty log or a prune links to the anchor ("" when
// nothing was ever pruned). [Log.Verify] recomputes every hash and checks
// every link, so a row that was edited, deleted or inserted shows as a
// break. A prune records the anchor it set in an [audit.OpAuditPrune] row,
// and Verify requires the newest such row to name the current anchor, so
// deleting the oldest rows and forging the anchor breaks the chain too.
//
// The chain does not stop a party with write access to the database from
// rewriting every row after the one it changed, or from deleting the newest
// rows; that needs the head hash kept outside the database, which is why
// [Log.Verify] reports it.
//
// # Locking
//
// An append reads the head and inserts the next row in one transaction
// under the dialect's chain lock, so appends from several processes form
// one linear chain and seq order is commit order. SQLite needs no lock: its
// write transaction already excludes every other writer.
package sqlaudit

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/sqlexec"
)

// Dialect holds what differs between the databases.
type Dialect struct {
	// Name prefixes error messages, e.g. "pgaudit".
	Name string
	// LockChain takes the chain lock for the rest of the transaction. No
	// arguments. Empty when the transaction itself excludes every other
	// writer.
	LockChain string
}

// Log is a hash-chained audit log in a SQL database.
type Log struct {
	db    sqlexec.DB
	d     Dialect
	clock func() time.Time
}

// New returns a log over db.
//
// Nil: db rejected, so a wiring mistake fails here rather than at the first
// write.
func New(db sqlexec.DB, d Dialect) (*Log, error) {
	if db == nil {
		return nil, fmt.Errorf("%s: New requires a database handle", d.Name)
	}
	return &Log{db: db, d: d, clock: time.Now}, nil
}

// WithClock returns a copy of l that stamps appends with clock. For tests.
func (l *Log) WithClock(clock func() time.Time) *Log {
	c := *l
	c.clock = clock
	return &c
}

func (l *Log) errf(format string, args ...any) error {
	return fmt.Errorf("%s: "+format, append([]any{l.d.Name}, args...)...)
}

// Row is the hashed content of one audit_log row: every column except
// prev_hash and hash.
type Row struct {
	Seq int64
	// At is the append time in microseconds since the epoch.
	At   int64
	Op   string
	User string
	Doc  []byte
}

// ChainHash is the hash of r whose predecessor's hash is prev: hex SHA-256
// over prev, seq and at (each followed by a newline), then op and user (each
// prefixed with its byte length and a colon), then doc. The length prefixes
// keep the encoding unambiguous. Exported so an external verifier can
// recompute it.
func ChainHash(prev string, r Row) string {
	h := sha256.New()
	fmt.Fprintf(h, "%s\n%d\n%d\n%d:%s%d:%s", prev, r.Seq, r.At, len(r.Op), r.Op, len(r.User), r.User)
	h.Write(r.Doc)
	return hex.EncodeToString(h.Sum(nil))
}

// Append writes rec as the next row, in a transaction of its own.
func (l *Log) Append(ctx context.Context, rec audit.Record) error {
	return l.db.Transact(ctx, func(q sqlexec.Querier) error {
		c, err := l.begin(ctx, q)
		if err != nil {
			return err
		}
		return c.append(ctx, rec, l.now())
	})
}

// AppendAll writes recs as the next rows, in one transaction. It reads the
// chain head once, so a write that records many rows pays for one lock and
// one head read.
func (l *Log) AppendAll(ctx context.Context, recs []audit.Record) error {
	return l.db.Transact(ctx, func(q sqlexec.Querier) error {
		c, err := l.begin(ctx, q)
		if err != nil {
			return err
		}
		at := l.now()
		for _, rec := range recs {
			if err := c.append(ctx, rec, at); err != nil {
				return err
			}
		}
		return nil
	})
}

func (l *Log) now() int64 { return l.clock().UTC().UnixMicro() }

// chain appends rows on one transaction. It holds the chain lock and the
// head, which it advances itself, so a batch reads the head once.
type chain struct {
	l    *Log
	q    sqlexec.Querier
	seq  int64
	hash string
}

// begin takes the chain lock on q, which must be in a transaction, and
// reads the head.
func (l *Log) begin(ctx context.Context, q sqlexec.Querier) (*chain, error) {
	if err := l.lock(ctx, q); err != nil {
		return nil, err
	}
	seq, hash, err := l.head(ctx, q)
	if err != nil {
		return nil, err
	}
	return &chain{l: l, q: q, seq: seq, hash: hash}, nil
}

// append writes rec as the next row, appended at at (microseconds).
func (c *chain) append(ctx context.Context, rec audit.Record, at int64) error {
	rec = audit.Prepare(rec)
	doc, err := json.Marshal(rec)
	if err != nil { // coverage-ignore: defensive: a Record has only marshalable fields
		return c.l.errf("encode record: %w", err)
	}
	row := Row{Seq: c.seq + 1, At: at, Op: rec.Op, User: rec.Principal.User, Doc: doc}
	hash := ChainHash(c.hash, row)
	if _, err := c.q.Exec(ctx, `INSERT INTO audit_log (seq, at, op, principal_user, doc, prev_hash, hash)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		row.Seq, row.At, row.Op, row.User, string(row.Doc), c.hash, hash); err != nil {
		return c.l.errf("append: %w", err)
	}
	for _, id := range subjectIDs(rec) {
		_, err := c.q.Exec(ctx, `INSERT INTO audit_subjects (entity_id, seq) VALUES (?, ?)`, id, row.Seq)
		if err != nil {
			return c.l.errf("append subject: %w", err)
		}
	}
	c.seq, c.hash = row.Seq, hash
	return nil
}

func (l *Log) lock(ctx context.Context, q sqlexec.Querier) error {
	if l.d.LockChain == "" {
		return nil
	}
	if _, err := q.Exec(ctx, l.d.LockChain); err != nil {
		return l.errf("lock: %w", err)
	}
	return nil
}

// head returns the seq and hash the next row links to: the last row's, or
// the anchor's when the log is empty.
func (l *Log) head(ctx context.Context, q sqlexec.Querier) (seq int64, hash string, err error) {
	err = q.QueryRow(ctx, `SELECT seq, hash FROM audit_log ORDER BY seq DESC LIMIT 1`).Scan(&seq, &hash)
	if err == nil {
		return seq, hash, nil
	}
	if !errors.Is(err, sqlexec.ErrNoRows) {
		return 0, "", l.errf("read head: %w", err)
	}
	return l.anchor(ctx, q)
}

// anchor returns the seq and hash of the last pruned row, or 0 and "" when
// nothing was pruned.
func (l *Log) anchor(ctx context.Context, q sqlexec.Querier) (seq int64, hash string, err error) {
	err = q.QueryRow(ctx, `SELECT seq, hash FROM audit_anchor WHERE id = 1`).Scan(&seq, &hash)
	switch {
	case err == nil:
		return seq, hash, nil
	case errors.Is(err, sqlexec.ErrNoRows):
		return 0, "", nil
	default:
		return 0, "", l.errf("read anchor: %w", err)
	}
}

// subjectIDs returns the distinct entity ids rec names, sorted.
func subjectIDs(rec audit.Record) []string {
	var ids []string
	for _, s := range []*audit.Subject{rec.Subject, rec.Before, rec.After} {
		if s == nil {
			continue
		}
		for _, id := range []string{s.ID, s.FromID, s.ToID} {
			if id != "" {
				ids = append(ids, id)
			}
		}
	}
	slices.Sort(ids)
	return slices.Compact(ids)
}
