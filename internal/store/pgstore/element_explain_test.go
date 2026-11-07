//go:build postgres

package pgstore_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/pgstore"
)

// TestElementRangeSumUsesIndex pins that a range sum over addresses reads
// entities_element_idx and relations_element_idx under a generic plan, so
// step 3 does not scan a table per range.
func TestElementRangeSumUsesIndex(t *testing.T) {
	pool := newScopedPool(t)
	s, err := pgstore.New(pool)
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	ctx := context.Background()
	require.NoError(t, s.Tx(ctx, func(v store.Store) error {
		const n = 3000
		for i := range n {
			if createErr := v.CreateEntity(ctx, entity.New(fmt.Sprintf("TASK-%06d", i), "task")); createErr != nil {
				return createErr
			}
		}
		for i := range n - 1 {
			key := entity.RelationKey{
				From: fmt.Sprintf("TASK-%06d", i), Type: "depends-on", To: fmt.Sprintf("TASK-%06d", i+1),
			}
			if _, createErr := v.CreateRelation(ctx, key, &store.RelationData{}); createErr != nil {
				return createErr
			}
		}
		return nil
	}))
	_, err = pool.Exec(ctx, "VACUUM ANALYZE entities, relations")
	require.NoError(t, err)

	plan := explainPreparedGeneric(t, pool, pgstore.EntityRangeSumSQLForTest,
		[]any{"task/TASK-000100", "task/TASK-000200"})
	require.Contains(t, plan, "entities_element_idx", plan)
	plan = explainPreparedGeneric(t, pool, pgstore.RelationRangeSumSQLForTest,
		[]any{"TASK-000100--depends-on--", "TASK-000200--depends-on--"})
	require.Contains(t, plan, "relations_element_idx", plan)
}
