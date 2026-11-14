//go:build postgres

package pgstore_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/search"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/pgstore"
)

// A search for a word most rows contain reads rows in title-distance order
// from the search_title index and stops at the limit; a search for a rare
// word reads its few matches through the trigram index on search_text
// (TKT-M58EQ1W).
//
// Before, every search ranked all of its matches and then sorted them, so a
// common word cost a scan of most of the table: 2.8 s at 96k entities on the
// perf seed. Both search builders are checked, in the default world and in a
// world that resolves faces, because the world shape is what a project with
// faces runs for every search.
//
// The plans are the ones PostgreSQL makes with the needle in hand, which is
// what the store asks for (searchQueryMode).
//
// Run with:
//
//	RELA_TEST_DATABASE_URL=... go test -tags postgres \
//	  -run=TestSearchExplain -v ./internal/store/pgstore/...
func TestSearchExplain(t *testing.T) {
	const policies = 3000
	pool := newScopedPool(t)
	s, err := pgstore.New(pool)
	require.NoError(t, err)
	ctx := context.Background()
	require.NoError(t, pgstore.SyncSearchTitles(ctx, pool, pgstore.SearchTitles{"policy": "title", "note": "title"}))

	words := []string{"gateway", "policy", "review", "budget", "access", "network", "vendor", "audit"}
	require.NoError(t, s.Tx(ctx, func(v store.Store) error {
		for i := range policies {
			// Policies have faces; notes have the default one, which the
			// default world reads.
			for _, face := range []entity.Face{"", "draft", "published"} {
				e := entity.New(fmt.Sprintf("POL-%06d", i), "policy")
				if face == "" {
					e = entity.New(fmt.Sprintf("NOTE-%06d", i), "note")
				}
				e.Face = face
				e.SetString("title", words[i%len(words)]+" "+words[(i/len(words))%len(words)]+" "+string(face))
				if i%1000 == 7 {
					e.SetString("title", "zebraquartz "+string(face))
				}
				// A body, as real entities have: without one the table is a
				// few pages and a sequential scan is always cheapest. Every
				// body carries "prose", the common word.
				e.Content = strings.Repeat(words[(i+3)%len(words)]+" notes and more prose. ", 12)
				if createErr := v.CreateEntity(ctx, e); createErr != nil {
					return createErr
				}
			}
		}
		return nil
	}))
	// VACUUM, not only ANALYZE: it also moves the fresh rows out of the GIN
	// index's pending list, as autovacuum does on a live database. The
	// planner prices a full pending list as a scan of its own.
	_, err = pool.Exec(ctx, "VACUUM ANALYZE entities")
	require.NoError(t, err)

	worlds := map[string]store.WorldScope{
		"default": store.TrivialScope(),
		"faces": store.NewWorldScope(map[string]store.TypeResolution{
			"policy": {Chain: []entity.Face{"published", "draft"}, Fallback: store.FallbackExclude},
		}),
	}
	for worldName, world := range worlds {
		builders := map[string]func(string) (string, []any){
			"gated": func(needle string) (string, []any) {
				return pgstore.BuildVisibleSearchSQLForTest(t, search.Query{Text: needle, World: world},
					map[string]search.TypeScope{"policy": {AllowAll: true}, "note": {AllowAll: true}}, 20)
			},
			"ungated": func(needle string) (string, []any) {
				return pgstore.BuildSearchSQLForTest(needle, 20, world)
			},
		}
		for _, tc := range []struct {
			needle string
			index  string
		}{
			{"prose", "entities_search_title_trgm_idx"},
			{"zebraquartz", "entities_search_trgm_idx"},
		} {
			for name, build := range builders {
				t.Run(worldName+"/"+name+"/"+tc.needle, func(t *testing.T) {
					sqlText, args := build(tc.needle)
					plan := explainPerExecution(t, pool, sqlText, args)
					t.Logf("plan:\n%s", plan)
					require.Contains(t, plan, tc.index, "the search does not read %s", tc.index)
					require.NotContains(t, plan, "Seq Scan on entities", "the search scans the table")
				})
			}
		}
	}
}

// explainPerExecution returns the plan PostgreSQL makes for sqlText with its
// arguments bound, without a cached prepared statement, as the store runs a
// search.
func explainPerExecution(t *testing.T, pool *pgxpool.Pool, sqlText string, args []any) string {
	t.Helper()
	rows, err := pool.Query(context.Background(), "EXPLAIN "+sqlText,
		append([]any{pgx.QueryExecModeExec}, args...)...)
	require.NoError(t, err)
	defer rows.Close()
	var lines []string
	for rows.Next() {
		var line string
		require.NoError(t, rows.Scan(&line))
		lines = append(lines, line)
	}
	require.NoError(t, rows.Err())
	return strings.Join(lines, "\n")
}
