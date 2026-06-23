package pgstore_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/pgstore"
)

// BenchmarkGraphQuery measures the SQL-native impl's wallclock at
// 1k and 10k tickets with a group-shaped predicate (typical ACL
// read-filter shape):
//
//   - alice → member-of → engineering
//   - engineering → owns → every Nth ticket
//
// With composite indexes the recursive CTE collapses to a single
// round-trip; without them PostgreSQL falls back to seq scans on
// `relations` for the rel_type filter at each CTE iteration. The
// pre-pushdown naive impl (now removed from pgstore) was 1 + N
// round-trips per request.
//
// Run with: just bench-postgres, or:
//
//	RELA_TEST_DATABASE_URL=... go test -tags postgres -bench=. \
//	  -run=^$ ./internal/store/pgstore/...
func BenchmarkGraphQuery(b *testing.B) {
	for _, n := range []int{1_000, 10_000} {
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			s := benchSetup(b, n)
			q := store.GraphQuery{
				EntityType: "ticket",
				HasInbound: &store.RelationPredicate{
					Endpoints:      []string{"alice"},
					OfTypes:        []string{"owns"},
					InheritThrough: []string{"member-of"},
					Depth:          5,
				},
			}
			b.ResetTimer()
			for range b.N {
				count := 0
				for _, err := range s.GraphQuery(context.Background(), q) {
					if err != nil {
						b.Fatal(err)
					}
					count++
				}
				if count == 0 {
					b.Fatal("query returned zero entities; setup is broken")
				}
			}
		})
	}
}

// benchSetup seeds n tickets, one team, one principal, and a
// member-of + owns chain. Every 10th ticket is owned by the team,
// so the predicate matches ~n/10 entities.
func benchSetup(b *testing.B, n int) store.Store {
	b.Helper()
	pool := newScopedPool(b)
	s, err := pgstore.New(pool)
	if err != nil {
		b.Fatalf("New: %v", err)
	}
	ctx := context.Background()
	mustCreate := func(e *entity.Entity) {
		if err := s.CreateEntity(ctx, e); err != nil {
			b.Fatalf("create %s: %v", e.ID, err)
		}
	}
	mustRel := func(from, relType, to string) {
		if _, err := s.CreateRelation(ctx, from, relType, to, nil); err != nil {
			b.Fatalf("relation %s --%s--> %s: %v", from, relType, to, err)
		}
	}

	mustCreate(entity.New("alice", "person"))
	mustCreate(entity.New("engineering", "team"))
	mustRel("alice", "member-of", "engineering")
	for i := range n {
		id := fmt.Sprintf("TKT-%06d", i)
		mustCreate(entity.New(id, "ticket"))
		if i%10 == 0 {
			mustRel("engineering", "owns", id)
		}
	}
	return s
}
