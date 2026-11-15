package pgstore

import (
	"context"
	"fmt"
	"iter"
	"sync/atomic"

	"github.com/jackc/pgx/v5"
)

// defaultIteratorPageSize is how many rows a store iterator reads per
// statement.
//
// # Why iterators page instead of streaming one cursor
//
// A pgx cursor keeps its pool connection checked out until its rows are
// closed. An iterator that yields inside rows.Next() therefore holds that
// connection for as long as the CALLER's loop body runs, and caller code
// routinely makes store calls of its own: visibility redaction asks the ACL
// about every row, which reads relations. Each concurrent lister then holds
// one connection and waits for a second, so as many listers as the pool has
// connections deadlock the whole process (BUG-9TGOH1).
//
// So every iterator reads one page completely, closes its rows, and only
// then yields. A connection is held while a page is fetched, never while
// caller code runs. On a Tx view this also keeps the single transaction
// connection free for the caller's nested statements.
//
// The cost is that an iteration is no longer one snapshot: a row written
// between two pages is seen or missed depending on where its key sorts
// relative to the cursor. Keyset paging never yields a key twice. A Tx does
// not restore the snapshot: it runs at READ COMMITTED, so each page sees
// what other sessions committed before it, and it takes the write lock, so
// it is no read-only alternative either. pgstore offers no snapshot
// iteration; a caller must not depend on one.
const defaultIteratorPageSize = 500

// iteratorPageSize holds [defaultIteratorPageSize]. It is atomic only so
// tests can shrink it while other tests' iterators read it.
var iteratorPageSize atomic.Int64

func init() { iteratorPageSize.Store(defaultIteratorPageSize) }

// pagedSeq yields the items of successive keyset pages. fetch reads the page
// after the keyset key `after` (nil for the first page) with its rows already
// closed, and returns the key to resume after, nil when there is no further
// page.
//
// The key is typed, never a rendered string parsed back: a key that failed to
// parse would restart the listing on every page and never end. As a second
// guard, a fetch that reports more rows but makes no progress past `after`
// ends the iteration with an error rather than looping.
func pagedSeq[T any, K comparable](fetch func(after *K) (items []T, next *K, err error)) iter.Seq2[T, error] {
	return func(yield func(T, error) bool) {
		var after *K
		for {
			items, next, err := fetch(after)
			if err != nil {
				var zero T
				yield(zero, err)
				return
			}
			for _, it := range items {
				if !yield(it, nil) {
					return
				}
			}
			if next == nil {
				return
			}
			if after != nil && *next == *after {
				var zero T
				yield(zero, fmt.Errorf("pgstore: keyset paging made no progress past %v", *after))
				return
			}
			after = next
		}
	}
}

// yieldAll is pagedSeq for a result read in one statement.
func yieldAll[T any](fetch func() ([]T, error)) iter.Seq2[T, error] {
	return pagedSeq(func(*struct{}) ([]T, *struct{}, error) {
		items, err := fetch()
		return items, nil, err
	})
}

// queryAll runs sql and scans every row, closing the rows before it returns.
func queryAll[T any](ctx context.Context, db DBTX, sql string, args []any, scan func(scanner) (T, error)) ([]T, error) {
	rows, err := db.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (T, error) { return scan(row) })
}

// queryPage is [queryAll] for one keyset page: sql must be ordered by the
// keyset, and keyOf returns a row's key. It appends the page LIMIT and reports
// a nil next key once a page comes back short.
func queryPage[T any, K any](
	ctx context.Context, db DBTX, sql string, args []any, p pagePlan,
	scan func(scanner) (T, error), keyOf func(T) K,
) (items []T, next *K, err error) {
	size := int(iteratorPageSize.Load())
	if p.oneShot {
		size = max(size, p.size)
		args = append([]any{pgx.QueryExecModeDescribeExec}, args...)
	}
	items, err = queryAll(ctx, db, sql+fmt.Sprintf(" LIMIT %d", size), args, scan)
	if err != nil || len(items) < size {
		return items, nil, err
	}
	k := keyOf(items[len(items)-1])
	return items, &k, nil
}

// pagePlan is how [queryPage] reads a statement that carries id arrays (an
// EntityIDs, PeerIDs or IDs filter). The zero value is the cached statement
// and the fixed page, for a statement with no large array.
//
// A large array changes two things (TKT-K7EM7Z5):
//
//   - The statement runs unnamed (oneShot), so PostgreSQL plans it for its
//     arguments. pgx prepares and caches every statement, and after five
//     runs PostgreSQL may switch a cached statement to a generic plan, made
//     without the argument values. A generic plan cannot hash an `= ANY($n)`
//     array, so a filter such as `to_id = ANY($3)` compares each row with
//     every element: rows times ids. Measured on the gantt edge read at 57k
//     entities: 8.4 s with the generic plan, 33 ms with a plan for the
//     arguments. Planning for the arguments costs about a millisecond per
//     thousand ids, so a small array keeps the cached statement.
//   - The page holds size rows instead of the fixed number. Every page sends
//     the arrays again, so at the fixed page size n ids that match about n
//     rows cost n/500 statements that each carry all n ids. With pages of n
//     rows the ids cross the wire about once.
//
// The page grows only for a one-shot statement. The LIMIT is part of the
// statement text, so a cached statement with a page sized to its batch would
// put one statement per batch size into pgx's statement cache.
type pagePlan struct {
	oneShot bool
	size    int
}

// oneShotIDs is the array length above which a statement runs one-shot.
const oneShotIDs = 500

// idBatchPage plans a read whose page rows are small (headers, relations):
// past the threshold it runs one-shot with a page of total rows. longest is
// the longest id array, the one a generic plan filters linearly; total is
// all ids the statement carries.
func idBatchPage(longest, total int) pagePlan {
	if longest <= oneShotIDs {
		return pagePlan{}
	}
	return pagePlan{oneShot: true, size: total}
}

// bodyPage plans a read of full entities: one-shot past the threshold, but
// on the fixed page, because a row carries its markdown body and a page of
// every requested id could hold far more memory than the ids do.
func bodyPage(ids int) pagePlan {
	return pagePlan{oneShot: ids > oneShotIDs}
}
