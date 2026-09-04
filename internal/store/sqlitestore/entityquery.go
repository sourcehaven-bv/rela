package sqlitestore

import (
	"fmt"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// The entity columns every read path selects, in the order scanEntity expects.
const entityColumns = "id, type, face, properties, content, updated_at"

// buildEntitySelectSQL builds the SELECT + WHERE + ORDER BY for entity
// listings. keysetAfter, when non-empty, resumes pagination after a cursor.
//
// Ordering is ascending (id, face): for the default-only zero-value query that
// is exactly the contract's historical ascending-id order (the face column is
// then constant, holding the empty default coordinate); under AllStates the
// states of an id sort immediately after its default row.
//
// That contiguity is a SHARED contract, not a detail of this backend: fs/mem
// match it via storeutil.CompareStateKeys, which orders their index by the same
// (bare id, face) tuple, and pgstore via `ORDER BY id ASC, face ASC`. Note
// fs/mem need an explicit comparator to get here — they key on the JOINED
// "id@face" string, and '@' (0x40) sorts after the digits (0x30-0x39), so plain
// string order puts PAGE-10's family inside PAGE-1's. Changing any one side's
// ordering breaks the others.
//
// For the DEFAULT world this is the historical flat SELECT, costing exactly
// what it did before worlds existed — a project that never declares a face must
// pay nothing (store.WorldScope.IsDefaultWorld).
//
// For a real world it becomes a windowed pick of each family's best-ranked
// candidate. Resolution cannot be a row predicate — see worldSQL — so the shape
// has to change, not just the WHERE clause.
//
// The keyset condition stays OUTSIDE the window: it must page over PRIMES, not
// over candidate rows. Applying it inside would let a cursor land mid-family
// and resolve a prime from a partial view, which is the wrong-prime hazard
// storeutil.PaginateWorldPrimes exists to avoid on the other backends.
func buildEntitySelectSQL(q store.EntityQuery, keysetAfter, columns string) (sqlText string, args []any) {
	if q.World.IsDefaultWorld() {
		where, wargs := entityWhere(q, keysetAfter)
		return `SELECT ` + columns + ` FROM entities` + where + ` ORDER BY id, face`, wargs
	}

	rank, rankArgs, candidate, candArgs := worldSQL(q.World, "")

	// ROW_NUMBER() OVER (PARTITION BY id ORDER BY rank, face) is SQLite's
	// DISTINCT ON: rn = 1 is the family's prime. The face tiebreak makes the
	// pick deterministic when two candidates share a rank, which only the
	// ELSE-0 arms can produce.
	inner := `SELECT ` + columns +
		`, ROW_NUMBER() OVER (PARTITION BY id ORDER BY (` + rank + `), face) AS rn` +
		` FROM entities`
	scope, scopeArgs := entityScopeWhere(q, candidate, candArgs)
	inner += scope

	// Positional '?' binds in the order the placeholders appear IN THE
	// STATEMENT TEXT, not in the order SQLite evaluates the clauses. The window
	// function sits in the SELECT list, so the rank's placeholders precede the
	// WHERE clause's even though the WHERE is evaluated first. Getting this
	// backwards produces a statement that runs, binds the type name where a
	// coordinate belongs, matches nothing, and silently excludes every entity.
	args = append(args, rankArgs...)
	args = append(args, scopeArgs...)

	outer := `SELECT ` + columns + ` FROM (` + inner + `) WHERE rn = 1`
	if keysetAfter != "" {
		// Cursor semantics match the default-world path: an unparseable cursor
		// RESTARTS rather than comparing against garbage.
		if cursorID, cursorFace, err := entity.ParseStateRef(keysetAfter); err == nil {
			outer += ` AND (id, face) > (?, ?)`
			args = append(args, cursorID, string(cursorFace))
		}
	}
	return outer + ` ORDER BY id, face`, args
}

// entityScopeWhere builds the WHERE clause for a WORLD-scoped listing: the
// world's candidate predicate plus the query's own type/IDs filters.
//
// It carries no keyset condition on purpose. Paging a world-scoped query must
// page over PRIMES, not candidate rows, so the cursor is applied OUTSIDE the
// window — see buildEntitySelectSQL.
func entityScopeWhere(q store.EntityQuery, candidate string, candArgs []any) (where string, args []any) {
	conds := []string{candidate}
	args = append(args, candArgs...)
	if q.Type != "" {
		conds = append(conds, "type = ?")
		args = append(args, q.Type)
	}
	if len(q.IDs) > 0 {
		conds = append(conds, "id IN ("+placeholders(len(q.IDs))+")")
		for _, id := range q.IDs {
			args = append(args, id)
		}
	}
	conds, args = appendFaceInCond(conds, q.FaceIn, args)
	return " WHERE " + strings.Join(conds, " AND "), args
}

// appendFaceInCond ANDs the FaceIn allowlist onto conds. Nil means every
// face (no condition). Applied to BOTH the default-world and the world-scoped
// candidate predicate: the ACL read path compiles a role's face grants into
// this set, and a backend that ignores it fails open (store.EntityQuery.FaceIn).
func appendFaceInCond(conds []string, faces []entity.Face, args []any) (outConds []string, outArgs []any) {
	if len(faces) == 0 {
		return conds, args
	}
	conds = append(conds, "face IN ("+placeholders(len(faces))+")")
	for _, f := range faces {
		args = append(args, string(f))
	}
	return conds, args
}

// entityWhere builds the WHERE clause for a DEFAULT-world listing.
//
// A non-default World does NOT come through here: it needs the widened
// candidate predicate plus a rank, which only buildEntitySelectSQL can pair up
// — see entityScopeWhere.
func entityWhere(q store.EntityQuery, keysetAfter string) (where string, args []any) {
	var conds []string
	// Default-world scope: the zero-value query returns default states only —
	// byte-identical behavior for faceless projects. AllStates is the raw
	// storage-truth escape hatch (see store.EntityQuery).
	if !q.AllStates {
		conds = append(conds, "face = ''")
	}
	if q.Type != "" {
		conds = append(conds, "type = ?")
		args = append(args, q.Type)
	}
	if len(q.IDs) > 0 {
		conds = append(conds, "id IN ("+placeholders(len(q.IDs))+")")
		for _, id := range q.IDs {
			args = append(args, id)
		}
	}
	conds, args = appendFaceInCond(conds, q.FaceIn, args)
	if keysetAfter != "" {
		// The cursor encodes the state key ("id" or "id@face"); a pre-states
		// cursor parses as a bare id with the zero face, so it keeps resuming
		// correctly. Row-value comparison matches the (id, face) ordering
		// exactly. An unparseable cursor genuinely RESTARTS (the keyset
		// condition is omitted) — comparing against the garbage string would
		// silently skip every row sorting below it, which a paging caller
		// cannot tell from end-of-results.
		if cursorID, cursorFace, err := entity.ParseStateRef(keysetAfter); err == nil {
			conds = append(conds, "(id, face) > (?, ?)")
			args = append(args, cursorID, string(cursorFace))
		}
	}
	if len(conds) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(conds, " AND "), args
}

// buildEntityCountSQL counts the entities in scope.
//
// Under a world it counts PRIMES, not candidate rows: an entity holding three
// faces is ONE entity in the world, so `count(*)` over the widened candidate
// set would over-count exactly those families that have several coordinates.
// `count(DISTINCT id)` is enough — the world admits at most one prime per id,
// so distinct ids and primes are the same number, and it avoids materializing
// the window.
//
// Counts must be world-scoped at all (RR-EHER1V): existence in a world is the
// publication bit, so an unscoped tally would tell a published-world surface
// how many unpublished drafts exist.
func buildEntityCountSQL(q store.EntityQuery) (sqlText string, args []any) {
	if q.World.IsDefaultWorld() {
		where, wargs := entityWhere(q, "")
		return "SELECT count(*) FROM entities" + where, wargs
	}
	_, _, candidate, candArgs := worldSQL(q.World, "")
	scope, scopeArgs := entityScopeWhere(q, candidate, candArgs)
	return "SELECT count(DISTINCT id) FROM entities" + scope, scopeArgs
}

// limitClause appends "LIMIT n" for a positive n. Interpolated rather than
// bound because it is an int the caller computed, never user text.
func limitClause(n int) string {
	if n <= 0 {
		return ""
	}
	return fmt.Sprintf(" LIMIT %d", n)
}
