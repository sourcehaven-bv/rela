// Package sqldb provides a MySQL-compatible SQL interface to rela graphs.
package sqldb

import (
	"context"
	"errors"
	"fmt"
	"io"

	sqle "github.com/dolthub/go-mysql-server"
	"github.com/dolthub/go-mysql-server/memory"
	"github.com/dolthub/go-mysql-server/sql"

	"github.com/Sourcehaven-BV/rela/internal/graph"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// MaxRows is the maximum number of rows returned by a query.
// Use LIMIT clause in SQL to get more specific results.
const MaxRows = 10000

// QueryResult holds the result of a SQL query.
type QueryResult struct {
	Columns   []string
	Rows      [][]interface{}
	Truncated bool // true if result was truncated due to MaxRows limit
}

// Query executes a SQL query against the rela graph and returns results.
// The provided context is used for cancellation.
func Query(ctx context.Context, g *graph.Graph, meta *metamodel.Metamodel, query string) (*QueryResult, error) {
	// Create the database
	db := NewDatabase("rela", g, meta)

	// Create provider and engine
	provider := memory.NewDBProvider(db)
	engine := sqle.NewDefault(provider)

	// Create a SQL context from the provided context
	sqlCtx := sql.NewContext(ctx)
	sqlCtx.SetCurrentDatabase("rela")

	// Execute the query
	schema, rowIter, _, err := engine.Query(sqlCtx, query)
	if err != nil {
		return nil, fmt.Errorf("query error: %w", err)
	}
	defer rowIter.Close(sqlCtx)

	// Extract column names
	result := &QueryResult{
		Columns: make([]string, len(schema)),
	}
	for i, col := range schema {
		result.Columns[i] = col.Name
	}

	// Collect rows with limit
	for {
		row, err := rowIter.Next(sqlCtx)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("row iteration error: %w", err)
		}

		// Enforce row limit
		if len(result.Rows) >= MaxRows {
			result.Truncated = true
			break
		}

		// Copy row values
		rowData := make([]interface{}, len(row))
		copy(rowData, row)
		result.Rows = append(result.Rows, rowData)
	}

	return result, nil
}
