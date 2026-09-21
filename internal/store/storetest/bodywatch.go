package storetest

import (
	"context"
	"iter"
	"sort"
	"strings"
	"sync"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// BodyWatch decorates a store.Store and counts how many entity BODIES a read
// path was served, so a test can pin that a collection read never materializes
// the markdown it will not render (BUG-SDMD6O).
//
// It is the third sibling of [Counting] and [Breadth], and the three answer
// three different questions about one read path. Counting asks "how many
// round-trips?", which an N+1 inflates. Breadth asks "how wide was each
// round-trip?", which an over-fetch inflates and an N+1 does not move.
// BodyWatch asks "how much CONTENT crossed the seam?", which neither of the
// other two moves at all: swapping ListEntityHeaders for ListEntities keeps
// both the call count and the id breadth byte-identical while changing the
// bytes transferred by two orders of magnitude.
//
// # Why a counter and not a heap measurement
//
// The motivating defect was measured at +101 MB vs +1 MB on pgstore to render
// 50 rows of a 5,000-row type, and the obvious instrument is therefore a heap
// assertion. That instrument is the wrong one here, for a reason the bug's own
// history records: the SAME probe on memstore reports ~1 MB whether or not the
// defect is present, because memstore's Clone shares the body STRING rather
// than copying it (Go strings are immutable, so a clone costs a header, not
// the bytes). A heap assertion is thus a test that cannot fail on the default
// backend, and one that can only run DB-gated — skipped on every developer
// machine without RELA_TEST_DATABASE_URL, and skipped in every CI job but one.
//
// Counting bodies removes the backend from the question entirely. "The list
// pipeline was handed N bodies to render 50 rows" is true on memstore, fsstore
// and pgstore alike, is exact rather than statistical, needs no threshold, no
// GC, no tolerance for allocator noise, and fails identically everywhere. The
// backend-dependent quantity (what those bodies COST) is what varies; the
// defect itself — that they were read at all — does not.
//
// A body is counted when a served row carries a non-empty Content, which is
// what the header/entity split exists to avoid. Rows are counted as they are
// YIELDED, so a lazily-drained iterator is charged only for what the caller
// actually pulled — the same accounting the retention it stands in for has.
//
// Reads are counted; writes pass through untouched, matching its two siblings:
// a test seeds through the same handle and seeding is not the measurement.
type BodyWatch struct {
	store.Store

	mu     sync.Mutex
	bodies map[string]int
	calls  map[string]int
}

// NewBodyWatch wraps s.
func NewBodyWatch(s store.Store) *BodyWatch {
	return &BodyWatch{Store: s, bodies: map[string]int{}, calls: map[string]int{}}
}

func (b *BodyWatch) call(method string) {
	b.mu.Lock()
	b.calls[method]++
	b.mu.Unlock()
}

func (b *BodyWatch) body(method string, e *entity.Entity) {
	if e == nil || e.Content == "" {
		return
	}
	b.mu.Lock()
	b.bodies[method]++
	b.mu.Unlock()
}

// Reset clears the recorded counts.
func (b *BodyWatch) Reset() {
	b.mu.Lock()
	b.bodies = map[string]int{}
	b.calls = map[string]int{}
	b.mu.Unlock()
}

// Bodies returns the total number of non-empty bodies served since the last
// Reset, across every read method.
//
// Summed across methods rather than reported per method because the property
// under test is a property of the PIPELINE — "this request read no body" — and
// which call would have carried one is exactly the implementation detail a
// test should not encode. [BodyWatch.String] renders the per-method split for
// failure messages.
func (b *BodyWatch) Bodies() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	n := 0
	for _, v := range b.bodies {
		n += v
	}
	return n
}

// String renders the counts as "Method=bodies/calls", sorted, for test failure
// messages. Both maps are read under ONE acquisition so the rendered line is a
// single consistent snapshot.
func (b *BodyWatch) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	keys := make([]string, 0, len(b.calls))
	for k := range b.calls {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var sb strings.Builder
	for i, k := range keys {
		if i > 0 {
			sb.WriteByte(' ')
		}
		sb.WriteString(k)
		sb.WriteByte('=')
		sb.WriteString(itoa(b.bodies[k]))
		sb.WriteByte('/')
		sb.WriteString(itoa(b.calls[k]))
	}
	return sb.String()
}

// countingSeq wraps an entity iterator, charging each row as it is yielded.
func (b *BodyWatch) countingSeq(
	method string, seq iter.Seq2[*entity.Entity, error],
) iter.Seq2[*entity.Entity, error] {
	return func(yield func(*entity.Entity, error) bool) {
		for e, err := range seq {
			if err == nil {
				b.body(method, e)
			}
			if !yield(e, err) {
				return
			}
		}
	}
}

func (b *BodyWatch) GetEntity(ctx context.Context, id string) (*entity.Entity, error) {
	b.call("GetEntity")
	e, err := b.Store.GetEntity(ctx, id)
	b.body("GetEntity", e)
	return e, err
}

func (b *BodyWatch) GetEntityState(ctx context.Context, id string, face entity.Face) (*entity.Entity, error) {
	b.call("GetEntityState")
	e, err := b.Store.GetEntityState(ctx, id, face)
	b.body("GetEntityState", e)
	return e, err
}

func (b *BodyWatch) ListEntities(ctx context.Context, q store.EntityQuery) iter.Seq2[*entity.Entity, error] {
	b.call("ListEntities")
	return b.countingSeq("ListEntities", b.Store.ListEntities(ctx, q))
}

func (b *BodyWatch) ListEntitiesPage(ctx context.Context, q store.EntityQuery) (store.Page[*entity.Entity], error) {
	b.call("ListEntitiesPage")
	p, err := b.Store.ListEntitiesPage(ctx, q)
	for _, e := range p.Items {
		b.body("ListEntitiesPage", e)
	}
	return p, err
}

func (b *BodyWatch) GraphQuery(ctx context.Context, q store.GraphQuery) iter.Seq2[*entity.Entity, error] {
	b.call("GraphQuery")
	return b.countingSeq("GraphQuery", b.Store.GraphQuery(ctx, q))
}

// ListEntityHeaders forwards to the wrapped store's header reader, or to the
// generic projection when it has none. Counted as a call, never as a body:
// a header carries no content by construction, which is the whole point of
// the capability. Declared so the decorator does not HIDE a capability the
// wrapped store has — dropping it would send every caller down the generic
// fallback and change the very routing under test.
func (b *BodyWatch) ListEntityHeaders(
	ctx context.Context, q store.EntityQuery,
) iter.Seq2[store.EntityHeader, error] {
	b.call("ListEntityHeaders")
	return store.ListEntityHeaders(ctx, b.Store, q)
}

// GraphQueryHeaders mirrors ListEntityHeaders for the graph-query shape.
//
// Forwarding through store.GraphQueryHeaders rather than asserting the
// capability here is deliberate: on a backend WITHOUT a native
// GraphHeaderQueryer the generic fallback drains GraphQuery, which this
// decorator also wraps — so the bodies that fallback materializes in-process
// are still counted, and a test cannot mistake "the backend has no header
// capability" for "the pipeline asked for no bodies". That is the honest
// reading: the fallback bounds retention, not transfer, and only the native
// path makes the bodies genuinely absent.
func (b *BodyWatch) GraphQueryHeaders(
	ctx context.Context, q store.GraphQuery,
) iter.Seq2[store.EntityHeader, error] {
	b.call("GraphQueryHeaders")
	return store.GraphQueryHeaders(ctx, b.Store, q)
}

// Tx forwards the transaction, wrapping the view so reads inside it count.
func (b *BodyWatch) Tx(ctx context.Context, fn func(store.Store) error) error {
	return b.Store.Tx(ctx, func(view store.Store) error {
		return fn(&bodyWatchView{parent: b, Store: view})
	})
}

// bodyWatchView is the in-transaction face of a [BodyWatch]: it reads through
// the transaction's store view while recording against the parent recorder, so
// both halves stay under the parent's single mutex. Mirrors breadthView; a
// second BodyWatch sharing the parent's MAPS but not its mutex would be a data
// race the race detector reports only when a test happens to read inside a Tx.
type bodyWatchView struct {
	store.Store

	parent *BodyWatch
}

func (v *bodyWatchView) GetEntity(ctx context.Context, id string) (*entity.Entity, error) {
	v.parent.call("GetEntity")
	e, err := v.Store.GetEntity(ctx, id)
	v.parent.body("GetEntity", e)
	return e, err
}

func (v *bodyWatchView) GetEntityState(ctx context.Context, id string, face entity.Face) (*entity.Entity, error) {
	v.parent.call("GetEntityState")
	e, err := v.Store.GetEntityState(ctx, id, face)
	v.parent.body("GetEntityState", e)
	return e, err
}

func (v *bodyWatchView) ListEntities(ctx context.Context, q store.EntityQuery) iter.Seq2[*entity.Entity, error] {
	v.parent.call("ListEntities")
	return v.parent.countingSeq("ListEntities", v.Store.ListEntities(ctx, q))
}

func (v *bodyWatchView) ListEntitiesPage(
	ctx context.Context, q store.EntityQuery,
) (store.Page[*entity.Entity], error) {
	v.parent.call("ListEntitiesPage")
	p, err := v.Store.ListEntitiesPage(ctx, q)
	for _, e := range p.Items {
		v.parent.body("ListEntitiesPage", e)
	}
	return p, err
}

func (v *bodyWatchView) GraphQuery(ctx context.Context, q store.GraphQuery) iter.Seq2[*entity.Entity, error] {
	v.parent.call("GraphQuery")
	return v.parent.countingSeq("GraphQuery", v.Store.GraphQuery(ctx, q))
}

func (v *bodyWatchView) ListEntityHeaders(
	ctx context.Context, q store.EntityQuery,
) iter.Seq2[store.EntityHeader, error] {
	v.parent.call("ListEntityHeaders")
	return store.ListEntityHeaders(ctx, v.Store, q)
}

func (v *bodyWatchView) GraphQueryHeaders(
	ctx context.Context, q store.GraphQuery,
) iter.Seq2[store.EntityHeader, error] {
	v.parent.call("GraphQueryHeaders")
	return store.GraphQueryHeaders(ctx, v.Store, q)
}
