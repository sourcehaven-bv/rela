// coverage-ignore: MCP tool handlers - tested via integration tests
package mcp

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/Sourcehaven-BV/rela/internal/sqldb"
)

func (s *Server) handleSQLQuery(
	ctx context.Context, request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	query, err := request.RequireString("query")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	result, err := sqldb.Query(ctx, s.ws.Graph(), s.ws.Meta(), query)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Format result as JSON
	rows := make([]map[string]interface{}, len(result.Rows))
	for i, row := range result.Rows {
		// Defensive check for column/row length mismatch
		if len(row) != len(result.Columns) {
			return mcp.NewToolResultError(fmt.Sprintf(
				"internal error: column count mismatch (%d columns, %d values)",
				len(result.Columns), len(row))), nil
		}
		rowMap := make(map[string]interface{})
		for j, col := range result.Columns {
			rowMap[col] = row[j]
		}
		rows[i] = rowMap
	}

	output := map[string]interface{}{
		"columns":   result.Columns,
		"rows":      rows,
		"row_count": len(result.Rows),
	}

	if result.Truncated {
		output["truncated"] = true
		output["max_rows"] = sqldb.MaxRows
	}

	text, err := marshalJSON(output)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(text), nil
}
