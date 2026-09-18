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

// Breadth decorates a store.Store and records how many IDS each batched read
// was handed, so a test can pin how WIDE an operation's store calls are.
//
// It is the sibling of [Counting], and the two answer different questions.
// Counting asks "how many round-trips?", which is what an N+1 inflates.
// Breadth asks "how big was each round-trip?", which an N+1 does not move at
// all — a caller that over-fetches does so in ONE call whose argument slice is
// enormous. Pinning only the count is therefore blind to a whole class of cost
// defect, and pinning only the breadth is blind to N+1; a read path that cares
// about both needs both.
//
// The motivating defect (RR-HKHPYG, TKT-MJKZQ3) is exactly the blind spot: a
// `display: nested` section resolved its relation columns over every visible
// child rather than the rows its budget would emit, passing ~280,000 ids to
// render at most 2,000 rows. The call count was byte-identical before and
// after the fix, because the over-fetch was one very wide ListEntityHeaders.
//
// Three batch carriers are recorded, and they are the ones a batching read
// path grows: EntityQuery.IDs, RelationQuery.EntityIDs, and the explicit ids
// argument to MatchingIDs (the ACL membership walk, one round-trip per
// distinct type rather than per row). An empty slice means "no id filter"
// rather than "no ids", so it records 0 and a test asserting a bound is
// unaffected by it.
//
// Breadth is NOT a bound on how many entities a read RETURNS — a type-filtered
// query with no id list reads the whole type and records 0. It measures the
// size of the argument a caller chose to pass, which is the quantity a caller
// controls and an over-fetch inflates.
//
// Reads are recorded; writes pass through untouched, matching Counting: a test
// seeds through the same handle and seeding is not the measurement.
type Breadth struct {
	store.Store

	mu    sync.Mutex
	ids   map[string]int
	calls map[string]int
}

// NewBreadth wraps s.
func NewBreadth(s store.Store) *Breadth {
	return &Breadth{Store: s, ids: map[string]int{}, calls: map[string]int{}}
}

func (b *Breadth) record(method string, n int) {
	b.mu.Lock()
	b.ids[method] += n
	b.calls[method]++
	b.mu.Unlock()
}

// Reset clears the recorded breadth.
func (b *Breadth) Reset() {
	b.mu.Lock()
	b.ids = map[string]int{}
	b.calls = map[string]int{}
	b.mu.Unlock()
}

// IDs returns the total number of ids handed to method since the last Reset,
// summed across every call to it.
//
// Summed rather than maxed because the bound a caller cares about is the
// total work it caused: two calls of 2,000 cost what one call of 4,000 costs.
// [Breadth.String] renders the ids/calls split for failure messages.
func (b *Breadth) IDs(method string) int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.ids[method]
}

// String renders the recorded breadth as "Method=ids/calls", sorted, for test
// failure messages.
//
// Both maps are read under ONE acquisition so the rendered line is a single
// consistent snapshot; taking the lock per map would let a concurrent record
// land between them and print an id total that disagrees with its call count.
func (b *Breadth) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	keys := make([]string, 0, len(b.ids))
	for k := range b.ids {
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
		sb.WriteString(itoa(b.ids[k]))
		sb.WriteByte('/')
		sb.WriteString(itoa(b.calls[k]))
	}
	return sb.String()
}

func (b *Breadth) ListEntities(ctx context.Context, q store.EntityQuery) iter.Seq2[*entity.Entity, error] {
	b.record("ListEntities", len(q.IDs))
	return b.Store.ListEntities(ctx, q)
}

func (b *Breadth) ListEntitiesPage(ctx context.Context, q store.EntityQuery) (store.Page[*entity.Entity], error) {
	b.record("ListEntitiesPage", len(q.IDs))
	return b.Store.ListEntitiesPage(ctx, q)
}

func (b *Breadth) ListRelations(ctx context.Context, q store.RelationQuery) iter.Seq2[*entity.Relation, error] {
	b.record("ListRelations", len(q.EntityIDs))
	return b.Store.ListRelations(ctx, q)
}

func (b *Breadth) ListRelationsPage(ctx context.Context, q store.RelationQuery) (store.Page[*entity.Relation], error) {
	b.record("ListRelationsPage", len(q.EntityIDs))
	return b.Store.ListRelationsPage(ctx, q)
}

// ListEntityHeaders forwards to the wrapped store's header reader, or to the
// generic projection when it has none — the same forwarding Counting does, so
// a consumer that type-asserts the header capability still finds it.
// MatchingIDs is the ACL membership-walk batch: one round-trip per distinct
// type rather than one per row, so its width grows with the row set exactly
// like the two query fields above.
func (b *Breadth) MatchingIDs(ctx context.Context, q store.GraphQuery, ids []string) (map[string]bool, error) {
	b.record("MatchingIDs", len(ids))
	return b.Store.MatchingIDs(ctx, q, ids)
}

func (b *Breadth) ListEntityHeaders(ctx context.Context, q store.EntityQuery) iter.Seq2[store.EntityHeader, error] {
	b.record("ListEntityHeaders", len(q.IDs))
	return store.ListEntityHeaders(ctx, b.Store, q)
}

// Tx forwards the transaction, wrapping the view so reads inside it record
// against this recorder.
//
// The view delegates to the SAME [Breadth], rather than being a second
// Breadth sharing its maps: a fresh struct would carry a fresh zero-value
// mutex, so it would lock its own lock while writing the parent's maps, which
// synchronizes nothing. The race detector runs in CI and would eventually say
// so. [Counting.Tx] has that shape; do not copy it here.
func (b *Breadth) Tx(ctx context.Context, fn func(store.Store) error) error {
	return b.Store.Tx(ctx, func(view store.Store) error {
		return fn(&breadthView{parent: b, Store: view})
	})
}

// breadthView is the in-transaction face of a [Breadth]: it reads through the
// transaction's store view while recording against the parent recorder, so
// both halves stay under the parent's single mutex.
type breadthView struct {
	store.Store

	parent *Breadth
}

func (v *breadthView) ListEntities(ctx context.Context, q store.EntityQuery) iter.Seq2[*entity.Entity, error] {
	v.parent.record("ListEntities", len(q.IDs))
	return v.Store.ListEntities(ctx, q)
}

func (v *breadthView) ListEntitiesPage(ctx context.Context, q store.EntityQuery) (store.Page[*entity.Entity], error) {
	v.parent.record("ListEntitiesPage", len(q.IDs))
	return v.Store.ListEntitiesPage(ctx, q)
}

func (v *breadthView) ListRelations(ctx context.Context, q store.RelationQuery) iter.Seq2[*entity.Relation, error] {
	v.parent.record("ListRelations", len(q.EntityIDs))
	return v.Store.ListRelations(ctx, q)
}

func (v *breadthView) ListRelationsPage(ctx context.Context, q store.RelationQuery) (store.Page[*entity.Relation], error) {
	v.parent.record("ListRelationsPage", len(q.EntityIDs))
	return v.Store.ListRelationsPage(ctx, q)
}

func (v *breadthView) MatchingIDs(ctx context.Context, q store.GraphQuery, ids []string) (map[string]bool, error) {
	v.parent.record("MatchingIDs", len(ids))
	return v.Store.MatchingIDs(ctx, q, ids)
}

func (v *breadthView) ListEntityHeaders(ctx context.Context, q store.EntityQuery) iter.Seq2[store.EntityHeader, error] {
	v.parent.record("ListEntityHeaders", len(q.IDs))
	return store.ListEntityHeaders(ctx, v.Store, q)
}
