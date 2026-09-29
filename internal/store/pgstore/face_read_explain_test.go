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
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/pgstore"
	"github.com/Sourcehaven-BV/rela/internal/store/storeutil"
)

// Every per-face and family read is served by an index, under a generic plan
// (TKT-KQXVF7, migration 0018).
//
// The fixture mixes a faceless type (task: 5,000 rows at the implicit face)
// with a faced type (policy: 2,500 families at draft and published, no row
// at the implicit face). The 0014 partial indexes served only rows at the
// implicit face, so every policy read below scanned or sorted the table.
//
// Each plan is taken from PREPARE/EXECUTE under force_generic_plan. pgx
// prepares statements, and after five executions PostgreSQL plans them with
// the parameters unknown. A one-shot EXPLAIN substitutes the parameters and
// can find an index that the generic plan misses (see
// TestEnumOrderExplainUsesDerivedIndexUnderGenericPlan), which is exactly
// what happened to HighestID's `id LIKE $1`.
//
// Run with:
//
//	RELA_TEST_DATABASE_URL=... go test -tags postgres \
//	  -run=TestFaceReadExplain -v ./internal/store/pgstore/...
func TestFaceReadExplain(t *testing.T) {
	const (
		tasks    = 5000
		policies = 2500
	)
	pool := newScopedPool(t)
	s, err := pgstore.New(pool)
	require.NoError(t, err)
	ctx := context.Background()

	require.NoError(t, s.Tx(ctx, func(v store.Store) error {
		for i := range tasks {
			if createErr := v.CreateEntity(ctx, entity.New(fmt.Sprintf("TASK-%06d", i), "task")); createErr != nil {
				return createErr
			}
		}
		for i := range policies {
			for _, face := range []entity.Face{"draft", "published"} {
				e := entity.New(fmt.Sprintf("POL-%06d", i), "policy")
				e.Face = face
				if createErr := v.CreateEntity(ctx, e); createErr != nil {
					return createErr
				}
			}
		}
		return nil
	}))
	_, err = pool.Exec(ctx, "ANALYZE entities")
	require.NoError(t, err)

	published := store.NewWorldScope(map[string]store.TypeResolution{
		"policy": {Chain: []entity.Face{"published", "draft"}, Fallback: store.FallbackExclude},
	})
	family := make([]string, 50)
	for i := range family {
		family[i] = fmt.Sprintf("POL-%06d", i*37)
	}

	type shape struct {
		name  string
		sql   string
		args  []any
		index string
		// sortOK allows a full Sort node. Only the family read has one, over
		// the few rows of its 50 families. An Incremental Sort is always
		// allowed: the world page uses one to rank the faces of each family
		// while reading the index in id order.
		sortOK bool
	}
	page := func(name string, q store.EntityQuery, sortOK bool) shape {
		sqlText, args := pgstore.BuildEntityListSQLForTest(t, q)
		return shape{name, sqlText, args, "entities_type_id_face_idx", sortOK}
	}
	highestSQL, highestArgs := pgstore.BuildHighestIDSQLForTest("POL")
	familySQL, familyArgs := pgstore.BuildEntityListSQLForTest(t, store.EntityQuery{IDs: family, Faces: store.AllFaces()})

	for _, tc := range []shape{
		page("default-face page", store.EntityQuery{Type: "task", Limit: 100, Faces: store.InWorld(store.DefaultWorld())}, false),
		page("all-faces page", store.EntityQuery{Type: "policy", Faces: store.AllFaces(), Limit: 100}, false),
		page("world page", store.EntityQuery{Type: "policy", Faces: store.InWorld(published), Limit: 100}, false),
		page("world page after a cursor", store.EntityQuery{
			Type: "policy", Faces: store.InWorld(published), Limit: 100,
			Cursor: storeutil.EncodeCursor("POL-001200@published"),
		}, false),
		page("explicit-faces page", store.EntityQuery{
			Type: "policy", Faces: store.AllFaces(), FaceIn: []entity.Face{"published"}, Limit: 100,
		}, false),
		{"family read", familySQL, familyArgs, "entities_pkey", true},
		{"single row", pgstore.GetEntityStateSQLForTest, []any{"POL-000007", "published"}, "entities_pkey", false},
		{"highest id", highestSQL, highestArgs, "entities_pkey", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			plan := explainPreparedGeneric(t, pool, tc.sql, tc.args)
			t.Logf("sql: %s\nplan:\n%s", tc.sql, plan)

			require.Contains(t, plan, tc.index, "the read is not served by its index")
			require.NotContains(t, plan, "Seq Scan on entities", "the read scans the table")
			if !tc.sortOK {
				for line := range strings.SplitSeq(plan, "\n") {
					require.NotRegexp(t, `^(->\s+)?Sort\s+\(`, strings.TrimSpace(line),
						"the page sorts instead of walking the index:\n%s", plan)
				}
			}
		})
	}
}

// explainPreparedGeneric returns the generic plan PostgreSQL uses for sqlText
// once a prepared statement has run five times.
//
// PREPARE plus EXPLAIN EXECUTE under force_generic_plan is the only way to see
// that plan: a one-shot EXPLAIN plans with the parameter values in hand. The
// values are rendered as literals because EXPLAIN takes no bind parameters of
// its own; they are test constants, and the renderer handles only the
// argument types the store's entity builders emit.
func explainPreparedGeneric(t *testing.T, pool *pgxpool.Pool, sqlText string, args []any) string {
	t.Helper()
	ctx := context.Background()
	conn, err := pool.Acquire(ctx)
	require.NoError(t, err)
	defer conn.Release()

	_, err = conn.Exec(ctx, "SET plan_cache_mode = force_generic_plan")
	require.NoError(t, err)
	_, err = conn.Exec(ctx, "PREPARE rela_face_explain AS "+sqlText)
	require.NoError(t, err)
	defer func() {
		_, deallocErr := conn.Exec(ctx, "DEALLOCATE rela_face_explain")
		require.NoError(t, deallocErr)
		_, resetErr := conn.Exec(ctx, "RESET plan_cache_mode")
		require.NoError(t, resetErr)
	}()

	literals := make([]string, len(args))
	for i, a := range args {
		literals[i] = sqlLiteral(t, a)
	}
	stmt := "EXPLAIN EXECUTE rela_face_explain"
	if len(literals) > 0 {
		stmt += "(" + strings.Join(literals, ", ") + ")"
	}
	rows, err := conn.Query(ctx, stmt)
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

func sqlLiteral(t *testing.T, v any) string {
	t.Helper()
	quote := func(s string) string { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }
	switch x := v.(type) {
	case string:
		return quote(x)
	case entity.Face:
		return quote(string(x))
	case []string:
		quoted := make([]string, len(x))
		for i, s := range x {
			quoted[i] = quote(s)
		}
		return "ARRAY[" + strings.Join(quoted, ", ") + "]::text[]"
	default:
		t.Fatalf("sqlLiteral: unsupported argument type %T", v)
		return ""
	}
}

func TestBuildHighestIDSQLBounds(t *testing.T) {
	_, args := pgstore.BuildHighestIDSQLForTest("FEAT")
	require.Equal(t, []any{"FEAT-", "FEAT."}, args)
}
