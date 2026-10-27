package storetest

import (
	"context"
	"iter"
	"maps"
	"sort"
	"strings"
	"sync"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// Counting decorates a store.Store and counts every read call by method
// name, so a test can pin how many store round-trips an operation costs —
// the backend-agnostic twin of the SQL accounting a postgres request gets
// through store.QueryStats (TKT-1U8XYN). An N+1 in a handler is N store
// calls whatever the backend, which is why this counts calls, not SQL.
//
// Reads are counted; writes pass through uncounted (a test seeds through
// the same handle, and seeding is not the thing under measurement). The
// optional capabilities a consumer type-asserts are forwarded: the header
// readers, the matched counter and the mention index (counted), soft delete
// (a write, so uncounted), and the transaction view (its reads counted too).
//
// Its exported methods are the store read surface it decorates plus the
// optional capabilities above, plus Unwrap for store.Decorator, so the count
// moves with store.Store rather than with this type.
//
//plimsoll:max-exported-methods=24
type Counting struct {
	store.Store

	// counts is shared by pointer with every transaction view, so a read
	// inside Tx takes the same lock as a read outside it.
	counts *callCounts
}

// callCounts is the counter state of a [Counting] and its transaction views.
type callCounts struct {
	mu    sync.Mutex
	calls map[string]int
}

// NewCounting wraps s.
func NewCounting(s store.Store) *Counting {
	return &Counting{Store: s, counts: &callCounts{calls: map[string]int{}}}
}

func (c *Counting) hit(method string) {
	c.counts.mu.Lock()
	c.counts.calls[method]++
	c.counts.mu.Unlock()
}

// Reset clears the counters.
func (c *Counting) Reset() {
	c.counts.mu.Lock()
	c.counts.calls = map[string]int{}
	c.counts.mu.Unlock()
}

// Reads returns the total number of read calls since the last Reset.
func (c *Counting) Reads() int {
	c.counts.mu.Lock()
	defer c.counts.mu.Unlock()
	n := 0
	for _, v := range c.counts.calls {
		n += v
	}
	return n
}

// Calls returns a copy of the per-method counters.
func (c *Counting) Calls() map[string]int {
	c.counts.mu.Lock()
	defer c.counts.mu.Unlock()
	out := make(map[string]int, len(c.counts.calls))
	maps.Copy(out, c.counts.calls)
	return out
}

// String renders the counters as "Method=n Method=n", sorted, for test
// failure messages.
func (c *Counting) String() string {
	calls := c.Calls()
	keys := make([]string, 0, len(calls))
	for k := range calls {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for i, k := range keys {
		if i > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(k)
		b.WriteByte('=')
		b.WriteString(itoa(calls[k]))
	}
	return b.String()
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var d []byte
	for n > 0 {
		d = append([]byte{byte('0' + n%10)}, d...)
		n /= 10
	}
	return string(d)
}

func (c *Counting) GetEntity(ctx context.Context, ref entity.Ref) (*entity.Entity, error) {
	c.hit("GetEntity")
	return c.Store.GetEntity(ctx, ref)
}

func (c *Counting) ListEntities(ctx context.Context, q store.EntityQuery) iter.Seq2[*entity.Entity, error] {
	c.hit("ListEntities")
	return c.Store.ListEntities(ctx, q)
}

func (c *Counting) ListEntitiesPage(ctx context.Context, q store.EntityQuery) (store.Page[*entity.Entity], error) {
	c.hit("ListEntitiesPage")
	return c.Store.ListEntitiesPage(ctx, q)
}

func (c *Counting) CountEntities(ctx context.Context, q store.EntityQuery) (int, error) {
	c.hit("CountEntities")
	return c.Store.CountEntities(ctx, q)
}

func (c *Counting) HighestID(ctx context.Context, prefix string) (int, error) {
	c.hit("HighestID")
	return c.Store.HighestID(ctx, prefix)
}

func (c *Counting) GetRelation(ctx context.Context, k entity.RelationKey) (*entity.Relation, error) {
	c.hit("GetRelation")
	return c.Store.GetRelation(ctx, k)
}

func (c *Counting) ListRelations(ctx context.Context, q store.RelationQuery) iter.Seq2[*entity.Relation, error] {
	c.hit("ListRelations")
	return c.Store.ListRelations(ctx, q)
}

func (c *Counting) ListRelationsPage(ctx context.Context, q store.RelationQuery) (store.Page[*entity.Relation], error) {
	c.hit("ListRelationsPage")
	return c.Store.ListRelationsPage(ctx, q)
}

func (c *Counting) CountRelations(ctx context.Context, q store.RelationQuery) (int, error) {
	c.hit("CountRelations")
	return c.Store.CountRelations(ctx, q)
}

func (c *Counting) GraphQuery(ctx context.Context, q store.GraphQuery) iter.Seq2[*entity.Entity, error] {
	c.hit("GraphQuery")
	return c.Store.GraphQuery(ctx, q)
}

func (c *Counting) GraphCount(ctx context.Context, q store.GraphQuery) (matched, total int, err error) {
	c.hit("GraphCount")
	return c.Store.GraphCount(ctx, q)
}

func (c *Counting) MatchingFaces(
	ctx context.Context, q store.GraphQuery, ids []string,
) (map[string][]entity.Face, error) {
	c.hit("MatchingFaces")
	return c.Store.MatchingFaces(ctx, q, ids)
}

// GraphQueryHeaders forwards to the wrapped store's native header query, or
// to the generic projection of GraphQuery — either way one read.
func (c *Counting) GraphQueryHeaders(ctx context.Context, q store.GraphQuery) iter.Seq2[store.EntityHeader, error] {
	c.hit("GraphQueryHeaders")
	return store.GraphQueryHeaders(ctx, c.Store, q)
}

// CountMatched forwards to the wrapped store's matched counter, or to
// GraphCount's matched half — either way one read.
func (c *Counting) CountMatched(ctx context.Context, q store.GraphQuery) (int, error) {
	c.hit("CountMatched")
	return store.CountMatched(ctx, c.Store, q)
}

// Rollup forwards to the wrapped store's native roll-up, counted as one
// read. A store without one runs the generic fallback through this
// decorator, so each of the fallback's own reads is counted.
func (c *Counting) Rollup(ctx context.Context, q store.RollupQuery) (map[string]store.RollupResult, error) {
	if r, ok := c.Store.(store.Rollupper); ok {
		c.hit("Rollup")
		return r.Rollup(ctx, q)
	}
	return store.RollupGeneric(ctx, c, q)
}

// ListEntityHeaders forwards to the wrapped store's header reader, or to
// the generic projection when it has none — either way it is counted as
// one read.
func (c *Counting) ListEntityHeaders(ctx context.Context, q store.EntityQuery) iter.Seq2[store.EntityHeader, error] {
	c.hit("ListEntityHeaders")
	return store.ListEntityHeaders(ctx, c.Store, q)
}

// MentionSources forwards to the wrapped store's mention index, counted as
// one read. A store without one fails the call: no index means no answer,
// not an empty one.
func (c *Counting) MentionSources(
	ctx context.Context, target string, after entity.Ref, limit int,
) ([]entity.Ref, error) {
	c.hit("MentionSources")
	return mentionSourcesOf(ctx, c.Store, target, after, limit)
}

// Unwrap returns the wrapped store, so a lookup keyed on the backend type
// (the database handle piles and state share) still finds it; see
// [store.Decorator].
func (c *Counting) Unwrap() store.Store { return c.Store }

// SoftDelete forwards the wrapped store's soft-delete capability. See
// [forwardedSoftDeleter] for a store without one.
func (c *Counting) SoftDelete() store.SoftDeleter {
	return forwardedSoftDeleter(c.Store)
}

// Tx forwards the transaction, wrapping the view so reads inside it count.
// The view shares the parent's counter state, lock included.
func (c *Counting) Tx(ctx context.Context, fn func(store.Store) error) error {
	return c.Store.Tx(ctx, func(view store.Store) error {
		return fn(&Counting{Store: view, counts: c.counts})
	})
}
