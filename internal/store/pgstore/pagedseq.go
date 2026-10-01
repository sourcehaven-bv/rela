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
	ctx context.Context, db DBTX, sql string, args []any,
	scan func(scanner) (T, error), keyOf func(T) K,
) (items []T, next *K, err error) {
	size := int(iteratorPageSize.Load())
	items, err = queryAll(ctx, db, sql+fmt.Sprintf(" LIMIT %d", size), args, scan)
	if err != nil || len(items) < size {
		return items, nil, err
	}
	k := keyOf(items[len(items)-1])
	return items, &k, nil
}
