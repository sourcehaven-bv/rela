package sqlitestore_test

import (
	"context"
	"path/filepath"
	"slices"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	rec "github.com/Sourcehaven-BV/rela/internal/reconcile"
	"github.com/Sourcehaven-BV/rela/internal/sqlitedb"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/sqlitestore"
	"github.com/Sourcehaven-BV/rela/internal/store/storetest"
)

func storedElements(t *testing.T, s store.Store) []storetest.StoredElement {
	t.Helper()
	els, err := sqlitestore.StoredElementsForTest(t.Context(), s.(*sqlitestore.Store))
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
	_, err := s.CreateRelation(ctx, entity.RelationKey{From: "FEAT-1", Type: "depends-on", To: "FEAT-2"},
		&store.RelationData{Content: "v1"})
	require.NoError(t, err)
}

// TestElementRawWriteClearedAndHealed pins elementDDL's safety net: a
// statement that changes a hashed column and not the element clears the
// element, and the version sweep fills it again. The content_hash triggers
// and the element triggers do not clear each other's column.
func TestElementRawWriteClearedAndHealed(t *testing.T) {
	s, db := openWithDB(t, filepath.Join(t.TempDir(), "e.db"))
	raw := db.DB()
	ctx := context.Background()
	seedElementRows(t, s)
	storetest.RequireElementsMatch(t, s, storedElements, "create")

	elementNull := func(q string) bool {
		t.Helper()
		var h *string
		require.NoError(t, raw.QueryRow(q).Scan(&h))
		return h == nil
	}
	const entityElement = `SELECT element_hash FROM entities WHERE id = 'FEAT-1'`
	const relationElement = `SELECT element_hash FROM relations WHERE from_id = 'FEAT-1'`

	_, err := raw.Exec(`UPDATE entities SET content_hash = 'h' WHERE id = 'FEAT-1'`)
	require.NoError(t, err)
	require.False(t, elementNull(entityElement), "a content_hash write must keep the element")

	_, err = raw.Exec(`UPDATE entities SET properties = '{"title":"raw"}' WHERE id = 'FEAT-1'`)
	require.NoError(t, err)
	_, err = raw.Exec(`UPDATE relations SET content = 'raw' WHERE from_id = 'FEAT-1'`)
	require.NoError(t, err)
	require.True(t, elementNull(entityElement), "a raw property change must clear the entity element")
	require.True(t, elementNull(relationElement), "a raw content change must clear the relation element")

	_, err = raw.Exec(`UPDATE entities SET content_hash = 'h2' WHERE id = 'FEAT-1'`)
	require.NoError(t, err)
	more, err := sqlitestore.HealElementsForTest(ctx, s, 10)
	require.NoError(t, err)
	require.False(t, more)
	var contentHash *string
	require.NoError(t, raw.QueryRow(`SELECT content_hash FROM entities WHERE id = 'FEAT-1'`).Scan(&contentHash))
	require.NotNil(t, contentHash, "the heal must not clear content_hash")
	storetest.RequireElementsMatch(t, s, storedElements, "heal")

	// The sweep tick runs the heal too.
	_, err = raw.Exec(`UPDATE entities SET content = 'raw again' WHERE id = 'FEAT-2'`)
	require.NoError(t, err)
	require.NoError(t, s.SweepNow(ctx, fixedProjection{}, immediateSweep(10)))
	storetest.RequireElementsMatch(t, s, storedElements, "sweep")
}

// TestElementHealBatches pins that a heal fills at most a batch per table and
// reports a full batch with progress, so the sweep runs again at once.
func TestElementHealBatches(t *testing.T) {
	s, db := openWithDB(t, filepath.Join(t.TempDir(), "e.db"))
	ctx := context.Background()
	for _, id := range []string{"FEAT-1", "FEAT-2", "FEAT-3"} {
		require.NoError(t, s.CreateEntity(ctx, entity.New(id, "feature")))
	}
	_, err := db.DB().Exec(`UPDATE entities SET element_hash = NULL`)
	require.NoError(t, err)

	more, err := sqlitestore.HealElementsForTest(ctx, s, 2)
	require.NoError(t, err)
	require.True(t, more, "a full batch with progress asks for another pass")
	more, err = sqlitestore.HealElementsForTest(ctx, s, 2)
	require.NoError(t, err)
	require.False(t, more)
	storetest.RequireElementsMatch(t, s, storedElements, "heal")
}

// TestElementHealSkipsUndecodableRow pins that a row whose element cannot be
// computed stays NULL without stopping the heal: the rows behind it in key
// order are still filled.
func TestElementHealSkipsUndecodableRow(t *testing.T) {
	s, db := openWithDB(t, filepath.Join(t.TempDir(), "e.db"))
	raw := db.DB()
	ctx := context.Background()
	seedElementRows(t, s)
	_, err := raw.Exec(`UPDATE entities SET element_hash = NULL`)
	require.NoError(t, err)
	_, err = raw.Exec(`UPDATE entities SET properties = 'not json' WHERE id = 'FEAT-1'`)
	require.NoError(t, err)

	_, err = sqlitestore.HealElementsForTest(ctx, s, 10)
	require.NoError(t, err)
	var unhashed []string
	rows, err := raw.Query(`SELECT id FROM entities WHERE element_hash IS NULL`)
	require.NoError(t, err)
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var id string
		require.NoError(t, rows.Scan(&id))
		unhashed = append(unhashed, id)
	}
	require.NoError(t, rows.Err())
	require.Equal(t, []string{"FEAT-1"}, unhashed, "only the undecodable row stays NULL")
}

// TestElementAddressOrder pins the address expression against
// [reconcile.EntityAddress] and the database order against Go's bytewise
// order, with non-ASCII and punctuation, and that a range sum over addresses
// reads the covering index.
func TestElementAddressOrder(t *testing.T) {
	s := open(t)
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
		want = append(want, rec.EntityAddress(row.typ, e.Ref()))
	}
	slices.Sort(want)
	got, err := sqlitestore.SortedAddressesForTest(ctx, s)
	require.NoError(t, err)
	require.Equal(t, want, got)
	storetest.RequireElementsMatch(t, s, storedElements, "create")

	plan, err := sqlitestore.ExplainEntityRangeForTest(ctx, s)
	require.NoError(t, err)
	require.Contains(t, plan, "USING COVERING INDEX entities_element_idx")
	plan, err = sqlitestore.ExplainRelationRangeForTest(ctx, s)
	require.NoError(t, err)
	require.Contains(t, plan, "USING COVERING INDEX relations_element_idx")
}

// TestMigrateV33ToV34Elements pins the v34 rung: a v33 database gains the
// element columns, triggers and indexes, its rows read NULL, and the heal
// fills them.
func TestMigrateV33ToV34Elements(t *testing.T) {
	path := filepath.Join(t.TempDir(), "v33.db")
	s, db := openWithDB(t, path)
	seedElementRows(t, s)
	for _, stmt := range []string{
		`DROP TRIGGER entities_clear_element`, `DROP TRIGGER relations_clear_element`,
		`DROP INDEX entities_element_idx`, `DROP INDEX relations_element_idx`,
		`DROP INDEX entities_elementless_idx`, `DROP INDEX relations_elementless_idx`,
	} {
		_, err := db.DB().Exec(stmt)
		require.NoError(t, err, stmt)
	}
	for _, table := range []string{"entities", "relations"} {
		for _, col := range []string{"element_hash", "element_fp0", "element_fp1", "element_fp2", "element_fp3"} {
			_, err := db.DB().Exec(`ALTER TABLE ` + table + ` DROP COLUMN ` + col)
			require.NoError(t, err)
		}
	}
	_, err := db.DB().Exec(`PRAGMA user_version = 33`)
	require.NoError(t, err)
	require.NoError(t, db.Close())

	s, db = openWithDB(t, path)
	var version int
	require.NoError(t, db.DB().QueryRow(`PRAGMA user_version`).Scan(&version))
	require.Equal(t, sqlitedb.SchemaVersion(), version)
	var unhashed int
	require.NoError(t, db.DB().QueryRow(`SELECT count(*) FROM entities WHERE element_hash IS NULL`).Scan(&unhashed))
	require.Equal(t, 2, unhashed, "no backfill at open")

	_, err = sqlitestore.HealElementsForTest(t.Context(), s, 10)
	require.NoError(t, err)
	storetest.RequireElementsMatch(t, s, storedElements, "heal after migration")

	_, err = db.DB().Exec(`UPDATE entities SET content = 'raw' WHERE id = 'FEAT-1'`)
	require.NoError(t, err)
	require.NoError(t, db.DB().QueryRow(`SELECT count(*) FROM entities WHERE element_hash IS NULL`).Scan(&unhashed))
	require.Equal(t, 1, unhashed, "the migrated database has the clearing trigger")
}
