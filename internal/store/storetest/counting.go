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

// Counting decorates a store.Store and counts every read call by method
// name, so a test can pin how many store round-trips an operation costs —
// the backend-agnostic twin of the SQL accounting a postgres request gets
// through store.QueryStats (TKT-1U8XYN). An N+1 in a handler is N store
// calls whatever the backend, which is why this counts calls, not SQL.
//
// Reads are counted; writes pass through uncounted (a test seeds through
// the same handle, and seeding is not the thing under measurement). The
// optional capabilities a consumer type-asserts are forwarded: the header
// reader (counted), and the transaction view (its reads counted too).
type Counting struct {
	store.Store

	mu    sync.Mutex
	calls map[string]int
}

// NewCounting wraps s.
func NewCounting(s store.Store) *Counting {
	return &Counting{Store: s, calls: map[string]int{}}
}

func (c *Counting) hit(method string) {
	c.mu.Lock()
	c.calls[method]++
	c.mu.Unlock()
}

// Reset clears the counters.
func (c *Counting) Reset() {
	c.mu.Lock()
	c.calls = map[string]int{}
	c.mu.Unlock()
}

// Reads returns the total number of read calls since the last Reset.
func (c *Counting) Reads() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	n := 0
	for _, v := range c.calls {
		n += v
	}
	return n
}

// Calls returns a copy of the per-method counters.
func (c *Counting) Calls() map[string]int {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make(map[string]int, len(c.calls))
	for k, v := range c.calls {
		out[k] = v
	}
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

func (c *Counting) GetEntity(ctx context.Context, id string) (*entity.Entity, error) {
	c.hit("GetEntity")
	return c.Store.GetEntity(ctx, id)
}

func (c *Counting) GetEntityState(ctx context.Context, id string, face entity.Face) (*entity.Entity, error) {
	c.hit("GetEntityState")
	return c.Store.GetEntityState(ctx, id, face)
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

func (c *Counting) PropertyValues(ctx context.Context, property string, limit int) ([]string, error) {
	c.hit("PropertyValues")
	return c.Store.PropertyValues(ctx, property, limit)
}

func (c *Counting) GetRelation(ctx context.Context, from, relType, to string) (*entity.Relation, error) {
	c.hit("GetRelation")
	return c.Store.GetRelation(ctx, from, relType, to)
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

func (c *Counting) MatchingIDs(ctx context.Context, q store.GraphQuery, ids []string) (map[string]bool, error) {
	c.hit("MatchingIDs")
	return c.Store.MatchingIDs(ctx, q, ids)
}

// ListEntityHeaders forwards to the wrapped store's header reader, or to
// the generic projection when it has none — either way it is counted as
// one read.
func (c *Counting) ListEntityHeaders(ctx context.Context, q store.EntityQuery) iter.Seq2[store.EntityHeader, error] {
	c.hit("ListEntityHeaders")
	return store.ListEntityHeaders(ctx, c.Store, q)
}

// Tx forwards the transaction, wrapping the view so reads inside it count.
func (c *Counting) Tx(ctx context.Context, fn func(store.Store) error) error {
	return c.Store.Tx(ctx, func(view store.Store) error {
		return fn(&Counting{Store: view, calls: c.calls})
	})
}
