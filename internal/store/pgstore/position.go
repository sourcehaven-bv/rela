package pgstore

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/Sourcehaven-BV/rela/internal/store"
)

var _ store.PositionQueryer = (*Store)(nil)

// GraphPosition implements store.PositionQueryer in ONE statement
// (TKT-U9DYW4): the finished graph select — world resolution included — is
// the inner relation, and window functions over q's ordering number its rows
// and name each row's neighbors. Only the row for id leaves the database, so
// scope navigation no longer transfers and sorts a whole type to answer with
// three ids.
//
// The window orders by exactly what buildGraphQuerySQLSelect pages by, so a
// list page and the position inside it cannot disagree.
func (s *Store) GraphPosition(
	ctx context.Context, q store.GraphQuery, id string,
) (store.Position, bool, error) {
	if err := checkGraphQueryScope(q); err != nil {
		return store.Position{}, false, err
	}
	sqlText, args := buildGraphPositionSQL(q, id)

	var (
		pos                store.Position
		prevID, prevType   *string
		nextID, nextType   *string
		rowNumber, matched int
	)
	err := s.db.QueryRow(ctx, sqlText, args...).Scan(&rowNumber, &matched, &prevID, &prevType, &nextID, &nextType)
	if errors.Is(err, pgx.ErrNoRows) {
		return store.Position{}, false, nil
	}
	if err != nil {
		return store.Position{}, false, fmt.Errorf("pgstore: graph position: %w", err)
	}
	pos.Index, pos.Total = rowNumber, matched
	if prevID != nil && prevType != nil {
		pos.Prev = &store.EntityRef{ID: *prevID, Type: *prevType}
	}
	if nextID != nil && nextType != nil {
		pos.Next = &store.EntityRef{ID: *nextID, Type: *nextType}
	}
	return pos, true, nil
}

func buildGraphPositionSQL(q store.GraphQuery, id string) (sqlText string, args []any) {
	q.Limit, q.Offset = 0, 0
	inner, args := buildGraphQuerySQLSelect(q, graphSelectHeaders)

	var window strings.Builder
	for _, spec := range q.OrderBy {
		args = append(args, spec.Property)
		dir := " ASC"
		if spec.Descending {
			dir = " DESC"
		}
		window.WriteString("(s.properties ->> $" + strconv.Itoa(len(args)) + `) COLLATE "C"` + dir + ", ")
	}
	// Byte-wise, like every id ordering here: the id column is declared
	// COLLATE "C" (guarded by TestListOrderIsByteWise), which the window
	// inherits exactly as the list's own ORDER BY does.
	window.WriteString("s.id ASC")

	args = append(args, id)
	return "SELECT rn, total, prev_id, prev_type, next_id, next_type FROM (" +
		"SELECT s.id, row_number() OVER w AS rn, count(*) OVER () AS total," +
		" lag(s.id) OVER w AS prev_id, lag(s.type) OVER w AS prev_type," +
		" lead(s.id) OVER w AS next_id, lead(s.type) OVER w AS next_type" +
		" FROM (" + inner + ") s WINDOW w AS (ORDER BY " + window.String() + ")" +
		") p WHERE p.id = $" + strconv.Itoa(len(args)), args
}
