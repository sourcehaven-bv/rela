// Package sqldb provides a MySQL-compatible SQL interface to rela graphs.
package sqldb

import (
	"context"
	"fmt"

	sqle "github.com/dolthub/go-mysql-server"
	"github.com/dolthub/go-mysql-server/memory"
	"github.com/dolthub/go-mysql-server/sql"

	"github.com/Sourcehaven-BV/rela/internal/graph"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// QueryResult holds the result of a SQL query.
type QueryResult struct {
	Columns []string
	Rows    [][]interface{}
}

// Query executes a SQL query against the rela graph and returns results.
func Query(g *graph.Graph, meta *metamodel.Metamodel, query string) (*QueryResult, error) {
	// Create the database
	db := NewDatabase("rela", g, meta)

	// Create provider and engine
	provider := memory.NewDBProvider(db)
	engine := sqle.NewDefault(provider)

	// Create a context
	ctx := sql.NewContext(context.Background())
	ctx.SetCurrentDatabase("rela")

	// Execute the query
	schema, rowIter, _, err := engine.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query error: %w", err)
	}
	defer rowIter.Close(ctx)

	// Extract column names
	result := &QueryResult{
		Columns: make([]string, len(schema)),
	}
	for i, col := range schema {
		result.Columns[i] = col.Name
	}

	// Collect rows
	for {
		row, err := rowIter.Next(ctx)
		if err != nil {
			break // EOF or error
		}

		// Copy row values
		rowData := make([]interface{}, len(row))
		copy(rowData, row)
		result.Rows = append(result.Rows, rowData)
	}

	return result, nil
}
