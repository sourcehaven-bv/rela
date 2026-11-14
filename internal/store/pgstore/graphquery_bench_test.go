package pgstore_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

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
			s, _ := benchSetup(b, n)
			q := store.GraphQuery{
				EntityType: "ticket",
				HasInbound: &store.RelationPredicate{
					Endpoints:      []string{"alice"},
					OfTypes:        []string{"owns"},
					InheritThrough: []string{"member-of"},
					Depth:          5,
				},
				Faces: store.InWorld(store.TrivialScope()),
			}
			// b.Loop runs this closure once per -count, so the seed above
			// is not repeated for every b.N probe.
			for b.Loop() {
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
// so the predicate matches ~n/10 entities. The seed runs in one
// transaction: 10k autocommitted inserts take most of a run, and the
// setup reruns for every -count.
func benchSetup(b *testing.B, n int) (store.Store, *pgxpool.Pool) {
	b.Helper()
	pool := newScopedPool(b)
	s, err := pgstore.New(pool)
	if err != nil {
		b.Fatalf("New: %v", err)
	}
	ctx := context.Background()
	if err := s.Tx(ctx, func(tx store.Store) error { return benchSeed(ctx, tx, n) }); err != nil {
		b.Fatalf("seed: %v", err)
	}
	return s, pool
}

func benchSeed(ctx context.Context, tx store.Store, n int) error {
	rel := func(from, relType, to string) error {
		_, err := tx.CreateRelation(ctx, entity.RelationKey{From: from, Type: relType, To: to}, nil)
		return err
	}
	if err := tx.CreateEntity(ctx, entity.New("alice", "person")); err != nil {
		return err
	}
	if err := tx.CreateEntity(ctx, entity.New("engineering", "team")); err != nil {
		return err
	}
	if err := rel("alice", "member-of", "engineering"); err != nil {
		return err
	}
	for i := range n {
		id := fmt.Sprintf("TKT-%06d", i)
		e := entity.New(id, "ticket")
		// Searchable text for BenchmarkSearchBackend: a quarter of the
		// tickets mention "login".
		e.SetString("title", fmt.Sprintf("ticket %d: %s in widget %d",
			i, []string{"login failure", "render glitch", "sync stall", "crash report"}[i%4], i%37))
		if err := tx.CreateEntity(ctx, e); err != nil {
			return fmt.Errorf("create %s: %w", id, err)
		}
		if i%10 == 0 {
			if err := rel("engineering", "owns", id); err != nil {
				return err
			}
		}
	}
	return nil
}
