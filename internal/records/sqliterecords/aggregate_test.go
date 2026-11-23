package sqliterecords_test

import (
	"context"
	"path/filepath"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/records"
	"github.com/Sourcehaven-BV/rela/internal/records/recordstest"
	"github.com/Sourcehaven-BV/rela/internal/records/sqlrecords"
	"github.com/Sourcehaven-BV/rela/internal/sqlexec"
	"github.com/Sourcehaven-BV/rela/internal/sqlexec/sqliteexec"
	"github.com/Sourcehaven-BV/rela/internal/sqlitedb"
)

// countingDB counts the statements that reach the database.
type countingDB struct {
	sqlexec.DB
	queries atomic.Int64
}

func (c *countingDB) Query(ctx context.Context, q string, args ...any) (sqlexec.Rows, error) {
	c.queries.Add(1)
	return c.DB.Query(ctx, q, args...)
}

// TestAggregateQueryBudget pins the cost of an aggregate: one statement,
// however many owners and rows, and no row cap.
func TestAggregateQueryBudget(t *testing.T) {
	ctx := context.Background()
	db, err := sqlitedb.Open(ctx, sqlitedb.Options{Path: filepath.Join(t.TempDir(), "rela.db")})
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	h, err := sqliteexec.New(db.DB())
	require.NoError(t, err)
	cdb := &countingDB{DB: h}
	s, err := sqlrecords.New(cdb, "test")
	require.NoError(t, err)

	var rows []records.Record
	var owners []string
	for i := range 40 {
		owner := "O" + strconv.Itoa(i)
		owners = append(owners, owner)
		for j := range 5 {
			rows = append(rows, recordstest.Row(owner+"-"+strconv.Itoa(j), "meter", owner,
				time.Duration(j)*time.Hour, map[string]any{"kwh": "1.5"}))
		}
	}
	require.NoError(t, s.Append(ctx, records.AppendRequest{Records: rows}))

	for _, field := range []string{"", "kwh"} {
		cdb.queries.Store(0)
		pts, err := s.Aggregate(ctx, records.AggregateQuery{Type: "meter", Owners: owners, Field: field,
			Bucket: records.BucketDay, PerOwner: true}, -1)
		require.NoError(t, err)
		require.Len(t, pts, 40)
		require.EqualValues(t, 1, cdb.queries.Load(), "field %q", field)
	}
}
