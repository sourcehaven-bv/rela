//go:build postgres

package pgstore_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/search"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/pgstore"
)

// A world-scoped search reads the trigram index when the needle has a
// trigram, and keeps the plain shape when it has none (TKT-T2I7P2).
//
// Before, the text predicate ran only after DISTINCT ON had resolved every
// family of the world, so the index was never used and every search cost a
// scan of the whole world. The matched-ids CTE (textHitsCTE) collects the
// candidate ids through the index first.
//
// Both plans are checked: the custom plan pgx gets for its first executions,
// and the generic plan PostgreSQL may switch a cached statement to.
//
// Run with:
//
//	RELA_TEST_DATABASE_URL=... go test -tags postgres \
//	  -run=TestWorldSearchExplain -v ./internal/store/pgstore/...
func TestWorldSearchExplain(t *testing.T) {
	const policies = 3000
	pool := newScopedPool(t)
	s, err := pgstore.New(pool)
	require.NoError(t, err)
	ctx := context.Background()

	words := []string{"gateway", "policy", "review", "budget", "access", "network", "vendor", "audit"}
	require.NoError(t, s.Tx(ctx, func(v store.Store) error {
		for i := range policies {
			for _, face := range []entity.Face{"draft", "published"} {
				e := entity.New(fmt.Sprintf("POL-%06d", i), "policy")
				e.Face = face
				e.SetString("title", words[i%len(words)]+" "+words[(i/len(words))%len(words)]+" "+string(face))
				if i%1000 == 7 {
					e.SetString("title", "zebraquartz "+string(face))
				}
				// A body, as real entities have: without one the table is a
				// few pages and a sequential scan is always cheapest.
				e.Content = strings.Repeat(words[(i+3)%len(words)]+" notes and more prose. ", 40)
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

	world := store.NewWorldScope(map[string]store.TypeResolution{
		"policy": {Chain: []entity.Face{"published", "draft"}, Fallback: store.FallbackExclude},
	})
	titles := pgstore.SearchTitles{"policy": "title"}
	gated := func(needle string) (string, []any) {
		return pgstore.BuildVisibleSearchSQLForTest(t, search.Query{Text: needle, World: world},
			map[string]search.TypeScope{"policy": {AllowAll: true}}, titles, 500)
	}
	ungated := func(needle string) (string, []any) {
		return pgstore.BuildSearchSQLForTest(needle, 500, world, titles)
	}

	for _, tc := range []struct {
		needle  string
		indexed bool
	}{
		{"zebraquartz", true},
		{"ze", false},
		{"a b", false},
	} {
		for name, build := range map[string]func(string) (string, []any){"gated": gated, "ungated": ungated} {
			t.Run(name+"/"+tc.needle, func(t *testing.T) {
				sqlText, args := build(tc.needle)
				require.Equal(t, tc.indexed, strings.Contains(sqlText, "text_hits"),
					"the matched-ids CTE is used exactly when the needle has a trigram:\n%s", sqlText)
				if !tc.indexed {
					return
				}
				for mode, plan := range map[string]string{
					"custom":  explainCustom(t, pool, sqlText, args),
					"generic": explainPreparedGeneric(t, pool, sqlText, args),
				} {
					t.Logf("%s plan:\n%s", mode, plan)
					require.Contains(t, plan, "entities_search_trgm_idx",
						"%s plan does not read the trigram index", mode)
					// The generic plan estimates the CTE at a few rows, so a
					// semi-join against it runs as a nested loop, quadratic
					// in the matches (4.5 s for 9.5k on the perf seed).
					require.NotContains(t, plan, "Semi Join",
						"%s plan joins the CTE as a semi-join", mode)
				}
			})
		}
	}
}

// explainCustom returns the plan for sqlText with its arguments bound, which
// is what PostgreSQL uses for a statement's first executions.
func explainCustom(t *testing.T, pool *pgxpool.Pool, sqlText string, args []any) string {
	t.Helper()
	rows, err := pool.Query(context.Background(), "EXPLAIN "+sqlText, args...)
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
