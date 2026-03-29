package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Sourcehaven-BV/rela/internal/sqldb"
)

const maxColumnWidth = 50

var queryCmd = &cobra.Command{
	Use:   "query <sql>",
	Short: "Execute a SQL query against the rela graph",
	Long: `Executes a SQL query against the rela graph in-process.

Entity types become tables with pluralized names (e.g., 'documents', 'components').
Relation types become tables with their relation name (e.g., 'implements', 'affects').

Examples:
  rela query "SELECT id, title FROM documents"
  rela query "SELECT * FROM components WHERE status = 'active'"
  rela query "SELECT f.id, r.title FROM functions f JOIN implements i ON f.id = i.from_id JOIN requirements r ON i.to_id = r.id"
  rela query "SELECT COUNT(*) FROM functions"`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		query := args[0]

		result, err := sqldb.Query(cmd.Context(), ws.Graph(), meta, query)
		if err != nil {
			return err
		}

		// Output based on format
		switch outputFormat {
		case "json":
			return outputQueryJSON(result)
		default:
			return outputQueryTable(result)
		}
	},
}

func outputQueryJSON(result *sqldb.QueryResult) error {
	// Convert to array of maps
	rows := make([]map[string]interface{}, len(result.Rows))
	for i, row := range result.Rows {
		rowMap := make(map[string]interface{})
		for j, col := range result.Columns {
			// Convert []byte to string for JSON
			if b, ok := row[j].([]byte); ok {
				rowMap[col] = string(b)
			} else {
				rowMap[col] = row[j]
			}
		}
		rows[i] = rowMap
	}

	data, err := json.MarshalIndent(rows, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}

func outputQueryTable(result *sqldb.QueryResult) error {
	if len(result.Rows) == 0 {
		fmt.Println("(no results)")
		return nil
	}

	// Calculate column widths
	widths := make([]int, len(result.Columns))
	for i, col := range result.Columns {
		widths[i] = len(col)
	}
	for _, row := range result.Rows {
		for i, val := range row {
			s := formatQueryValue(val)
			if len(s) > widths[i] {
				widths[i] = len(s)
			}
		}
	}

	// Cap widths at maxColumnWidth chars
	for i := range widths {
		if widths[i] > maxColumnWidth {
			widths[i] = maxColumnWidth
		}
	}

	// Print header
	var header strings.Builder
	var sep strings.Builder
	for i, col := range result.Columns {
		if i > 0 {
			header.WriteString(" | ")
			sep.WriteString("-+-")
		}
		header.WriteString(padRight(col, widths[i]))
		sep.WriteString(strings.Repeat("-", widths[i]))
	}
	fmt.Println(header.String())
	fmt.Println(sep.String())

	// Print rows
	for _, row := range result.Rows {
		var line strings.Builder
		for i, val := range row {
			if i > 0 {
				line.WriteString(" | ")
			}
			s := formatQueryValue(val)
			if len(s) > widths[i] {
				s = s[:widths[i]-3] + "..."
			}
			line.WriteString(padRight(s, widths[i]))
		}
		fmt.Println(line.String())
	}

	if result.Truncated {
		fmt.Printf("\n(%d rows, truncated at %d - use LIMIT clause for specific results)\n",
			len(result.Rows), sqldb.MaxRows)
	} else {
		fmt.Printf("\n(%d rows)\n", len(result.Rows))
	}
	return nil
}

func formatQueryValue(val interface{}) string {
	if val == nil {
		return "NULL"
	}
	if b, ok := val.([]byte); ok {
		return string(b)
	}
	return fmt.Sprintf("%v", val)
}

func padRight(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(s))
}

func init() {
	rootCmd.AddCommand(queryCmd)
}
