package pgstore_test

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/reconcile"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/pgstore"
	"github.com/Sourcehaven-BV/rela/internal/store/storetest"
)

func storedElements(t *testing.T, s store.Store) []storetest.StoredElement {
	t.Helper()
	els, err := pgstore.StoredElementsForTest(t.Context(), s.(*pgstore.Store))
	require.NoError(t, err)
	return els
}

// seedElementRows stores FEAT-1 and FEAT-2 and a relation between them.
func seedElementRows(t *testing.T, s store.Store) {
	t.Helper()
	ctx := t.Context()
	e := entity.New("FEAT-1", "feature")
	e.SetString("title", "v1")
	require.NoError(t, s.CreateEntity(ctx, e))
	require.NoError(t, s.CreateEntity(ctx, entity.New("FEAT-2", "feature")))
	key := entity.RelationKey{From: "FEAT-1", Type: "depends-on", To: "FEAT-2"}
	_, err := s.CreateRelation(ctx, key, &store.RelationData{Content: "v1"})
	require.NoError(t, err)
}

// TestElementRawWriteClearedAndHealed pins migration 0042's safety net: a
// statement that changes a hashed column and not the element clears the
// element, and the version sweep fills it again. The content_hash triggers
// and the element triggers do not clear each other's column.
func TestElementRawWriteClearedAndHealed(t *testing.T) {
	pool := newScopedPool(t)
	s, err := pgstore.New(pool)
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	ctx := context.Background()
	seedElementRows(t, s)
	storetest.RequireElementsMatch(t, s, storedElements, "create")

	elementNull := func(q string) bool {
		t.Helper()
		var h *string
		require.NoError(t, pool.QueryRow(ctx, q).Scan(&h))
		return h == nil
	}
	const entityElement = `SELECT element_hash FROM entities WHERE id = 'FEAT-1'`
	const relationElement = `SELECT element_hash FROM relations WHERE from_id = 'FEAT-1'`

	// content_hash moves without touching the element.
	_, err = pool.Exec(ctx, `UPDATE entities SET content_hash = 'h' WHERE id = 'FEAT-1'`)
	require.NoError(t, err)
	require.False(t, elementNull(entityElement), "a content_hash write must keep the element")

	_, err = pool.Exec(ctx, `UPDATE entities SET properties = '{"title":"raw"}' WHERE id = 'FEAT-1'`)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE relations SET content = 'raw' WHERE from_id = 'FEAT-1'`)
	require.NoError(t, err)
	require.True(t, elementNull(entityElement), "a raw property change must clear the entity element")
	require.True(t, elementNull(relationElement), "a raw content change must clear the relation element")

	// The heal writes only element columns: content_hash stays cleared by
	// the raw write (0020) and set where it was set.
	_, err = pool.Exec(ctx, `UPDATE entities SET content_hash = 'h2' WHERE id = 'FEAT-1'`)
	require.NoError(t, err)
	require.NoError(t, s.SweepNow(ctx, stubProvider{}, store.SweepConfig{
		Idle: time.Nanosecond, MaxStaleness: time.Hour, Batch: 10,
	}))
	var contentHash *string
	require.NoError(t, pool.QueryRow(ctx, `SELECT content_hash FROM entities WHERE id = 'FEAT-1'`).Scan(&contentHash))
	require.NotNil(t, contentHash, "the heal must not clear content_hash")
	storetest.RequireElementsMatch(t, s, storedElements, "heal")
}

// TestElementJSONBEqualWrite pins that a properties write jsonb treats as
// equal (1 and 1.0) keeps an element that still matches the row.
func TestElementJSONBEqualWrite(t *testing.T) {
	pool := newScopedPool(t)
	s, err := pgstore.New(pool)
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	ctx := context.Background()

	e := entity.New("FEAT-1", "feature")
	e.Properties["n"] = 1
	require.NoError(t, s.CreateEntity(ctx, e))
	_, err = pool.Exec(ctx, `UPDATE entities SET properties = '{"n": 1.0}' WHERE id = 'FEAT-1'`)
	require.NoError(t, err)
	storetest.RequireElementsMatch(t, s, storedElements, "jsonb-equal write")
}

// TestElementHealSkipsRowChangedAfterRead pins the xmin guard on the heal: a
// row changed between the heal's read and its write keeps NULL, so a stale
// element is never stored. The next heal fills it from the new content.
func TestElementHealSkipsRowChangedAfterRead(t *testing.T) {
	pool := newScopedPool(t)
	s, err := pgstore.New(pool)
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	ctx := context.Background()
	seedElementRows(t, s)

	_, err = pool.Exec(ctx, `UPDATE entities SET element_hash = NULL WHERE id = 'FEAT-1'`)
	require.NoError(t, err)
	read, stored, err := pgstore.HealEntitiesWithWriteForTest(ctx, s, func() {
		_, werr := pool.Exec(ctx, `UPDATE entities SET content = 'changed' WHERE id = 'FEAT-1'`)
		require.NoError(t, werr)
	})
	require.NoError(t, err)
	require.Equal(t, 1, read)
	require.Zero(t, stored, "the heal must not store an element for a row that changed after its read")

	more, err := pgstore.HealElementsForTest(ctx, s, 10)
	require.NoError(t, err)
	require.False(t, more)
	storetest.RequireElementsMatch(t, s, storedElements, "second heal")
}

// TestElementHealSkipsLockedRow pins that the heal does not wait on a row
// another transaction holds: it fills the rows it can lock and leaves the
// held row NULL for a later pass. Waiting could deadlock with a writer that
// locks many rows in another order.
func TestElementHealSkipsLockedRow(t *testing.T) {
	pool := newScopedPool(t)
	s, err := pgstore.New(pool)
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	ctx := context.Background()
	seedElementRows(t, s)
	_, err = pool.Exec(ctx, `UPDATE entities SET element_hash = NULL`)
	require.NoError(t, err)

	holder, err := pool.Begin(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { _ = holder.Rollback(ctx) })
	_, err = holder.Exec(ctx, `SELECT 1 FROM entities WHERE id = 'FEAT-1' FOR UPDATE`)
	require.NoError(t, err)

	healCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	_, err = pgstore.HealElementsForTest(healCtx, s, 10)
	require.NoError(t, err, "the heal must not wait on a locked row")
	var unhashed []string
	rows, err := pool.Query(ctx, `SELECT id FROM entities WHERE element_hash IS NULL`)
	require.NoError(t, err)
	for rows.Next() {
		var id string
		require.NoError(t, rows.Scan(&id))
		unhashed = append(unhashed, id)
	}
	require.NoError(t, rows.Err())
	require.Equal(t, []string{"FEAT-1"}, unhashed, "only the locked row stays NULL")

	require.NoError(t, holder.Rollback(ctx))
	_, err = pgstore.HealElementsForTest(ctx, s, 10)
	require.NoError(t, err)
	storetest.RequireElementsMatch(t, s, storedElements, "heal after release")
}

// TestElementAddressOrder pins the address expression against
// [reconcile.EntityAddress] and the database order against Go's bytewise
// order, with non-ASCII and punctuation that a linguistic collation would
// reorder.
func TestElementAddressOrder(t *testing.T) {
	pool := newScopedPool(t)
	s, err := pgstore.New(pool)
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	ctx := context.Background()

	var want []string
	for _, row := range []struct {
		id, typ string
		face    entity.Face
	}{
		{"A-1", "zeta", ""}, {"a-2", "Zeta", "draft"}, {"B_2", "Äpfel", ""}, {"b-3", "apfel", ""},
		{"C-3", "émoji", ""}, {"c4", "e", "approved"}, {"D-4", "a-b", ""}, {"d-5", "a_b", ""},
	} {
		e := entity.New(row.id, row.typ)
		e.Face = row.face
		require.NoError(t, s.CreateEntity(ctx, e))
		want = append(want, reconcile.EntityAddress(row.typ, e.Ref()))
	}
	slices.Sort(want)
	got, err := pgstore.SortedAddressesForTest(ctx, s)
	require.NoError(t, err)
	require.Equal(t, want, got)
	storetest.RequireElementsMatch(t, s, storedElements, "create")
}

// TestMigration0042_Elements pins migration 0042 on a database that has rows:
// they read NULL after it, the heal fills them, and a second run is harmless.
func TestMigration0042_Elements(t *testing.T) {
	pool := newScopedPool(t)
	s, err := pgstore.New(pool)
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	ctx := context.Background()
	seedElementRows(t, s)
	for _, stmt := range []string{
		`DROP TRIGGER entities_clear_element ON entities`,
		`DROP TRIGGER relations_clear_element ON relations`,
		`DROP FUNCTION rela_clear_element()`,
		`ALTER TABLE entities DROP COLUMN element_hash, DROP COLUMN element_fp0, DROP COLUMN element_fp1,
			DROP COLUMN element_fp2, DROP COLUMN element_fp3`,
		`ALTER TABLE relations DROP COLUMN element_hash, DROP COLUMN element_fp0, DROP COLUMN element_fp1,
			DROP COLUMN element_fp2, DROP COLUMN element_fp3`,
		`UPDATE schema_version SET version = 41`,
	} {
		_, err = pool.Exec(ctx, stmt)
		require.NoError(t, err, stmt)
	}

	require.NoError(t, pgstore.Migrate(ctx, pool))
	_, err = pool.Exec(ctx, `UPDATE schema_version SET version = 41`)
	require.NoError(t, err)
	require.NoError(t, pgstore.Migrate(ctx, pool), "a second run of 0042 is harmless")
	var unhashed int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM entities WHERE element_hash IS NULL`).Scan(&unhashed))
	require.Equal(t, 2, unhashed, "no backfill in the migration")

	_, err = pgstore.HealElementsForTest(ctx, s, 10)
	require.NoError(t, err)
	storetest.RequireElementsMatch(t, s, storedElements, "heal after migration")
}
