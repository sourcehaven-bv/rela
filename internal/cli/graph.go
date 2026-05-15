package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

var (
	graphOutput    string
	graphFormat    string
	graphDirection string
	graphTypes     []string
)

var graphCmd = &cobra.Command{
	Use:   "graph",
	Short: "Export graph to Graphviz DOT format",
	Long: `Exports the entity graph to Graphviz DOT format for visualization.

Examples:
  rela graph                          # Print DOT to stdout
  rela graph -o graph.dot             # Write to file
  rela graph -o graph.png -f png      # Render to PNG (requires Graphviz)
  rela graph --types requirement,decision`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Get entities and edges
		ctx := context.Background()
		svc := cliReadFromContext(cmd.Context())
		st := svc.Store()
		meta := svc.Meta()

		var entities []*entity.Entity
		for e, err := range st.ListEntities(ctx, store.EntityQuery{}) {
			if err != nil {
				return err
			}
			entities = append(entities, e)
		}

		var edges []*entity.Relation
		for r, err := range st.ListRelations(ctx, store.RelationQuery{}) {
			if err != nil {
				return err
			}
			edges = append(edges, r)
		}

		// Filter by types if specified
		if len(graphTypes) > 0 {
			typeSet := make(map[string]bool)
			for _, t := range graphTypes {
				// Resolve aliases and handle plurals
				t = strings.TrimSuffix(t, "s")
				resolved := meta.ResolveAlias(t)
				typeSet[resolved] = true
			}

			filtered := make([]*entity.Entity, 0)
			for _, e := range entities {
				if typeSet[e.Type] {
					filtered = append(filtered, e)
				}
			}
			entities = filtered

			// Also filter edges
			entityIDs := make(map[string]bool)
			for _, e := range entities {
				entityIDs[e.ID] = true
			}

			filteredEdges := make([]*entity.Relation, 0)
			for _, edge := range edges {
				if entityIDs[edge.From] && entityIDs[edge.To] {
					filteredEdges = append(filteredEdges, edge)
				}
			}
			edges = filteredEdges
		}

		// Generate DOT
		dot := generateDOT(meta, entities, edges)

		// Output handling
		if graphOutput == "" {
			// Print to stdout
			fmt.Println(dot)
			return nil
		}

		// Check if we need to render
		if graphFormat != "" && graphFormat != "dot" {
			// Use Graphviz to render
			return renderWithGraphviz(cmd.Context(), dot, graphOutput, graphFormat)
		}

		// Write DOT file
		return os.WriteFile(graphOutput, []byte(dot), 0644)
	},
}

func generateDOT(meta *metamodel.Metamodel, entities []*entity.Entity, edges []*entity.Relation) string {
	var sb strings.Builder

	direction := "TB"
	if graphDirection == "lr" {
		direction = "LR"
	}

	sb.WriteString("digraph architecture {\n")
	fmt.Fprintf(&sb, "  rankdir=%s;\n", direction)
	sb.WriteString("  node [shape=box, style=filled];\n")
	sb.WriteString("\n")

	// Group nodes by type
	typeGroups := make(map[string][]*entity.Entity)
	for _, e := range entities {
		typeGroups[e.Type] = append(typeGroups[e.Type], e)
	}

	// Write nodes grouped by type (as subgraphs for clustering).
	// DOT unquoted IDs must match [_A-Za-z][_A-Za-z0-9]*, so entity
	// types with hyphens (e.g. `review-response`) need sanitization
	// to keep the cluster ID valid.
	for entityType, group := range typeGroups {
		fmt.Fprintf(&sb, "  subgraph cluster_%s {\n", sanitizeDOTID(entityType))
		fmt.Fprintf(&sb, "    label=\"%ss\";\n", strings.ToUpper(entityType[:1])+entityType[1:])

		// Get color from metamodel
		color := "#FFFFFF"
		if def, ok := meta.GetEntityDef(entityType); ok && def.Color != "" {
			color = def.Color
		}

		for _, e := range group {
			label := escapeLabel(e.Title())
			if label == "" {
				label = e.ID
			} else {
				label = e.ID + "\\n" + label
			}
			fmt.Fprintf(&sb, "    \"%s\" [label=\"%s\", fillcolor=\"%s\"];\n",
				e.ID, label, color)
		}

		sb.WriteString("  }\n\n")
	}

	// Write edges
	for _, edge := range edges {
		fmt.Fprintf(&sb, "  \"%s\" -> \"%s\" [label=\"%s\"];\n",
			edge.From, edge.To, edge.Type)
	}

	sb.WriteString("}\n")

	return sb.String()
}

const maxLabelLen = 40

// sanitizeDOTID converts a string into a valid unquoted DOT identifier
// by replacing any character outside [A-Za-z0-9_] with '_'. Used for
// subgraph cluster IDs, where entity types containing hyphens would
// otherwise produce invalid DOT.
func sanitizeDOTID(s string) string {
	var sb strings.Builder
	sb.Grow(len(s))
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_':
			sb.WriteRune(r)
		default:
			sb.WriteByte('_')
		}
	}
	return sb.String()
}

func escapeLabel(s string) string {
	s = strings.ReplaceAll(s, "\"", "\\\"")
	s = strings.ReplaceAll(s, "\n", "\\n")
	// Truncate long labels
	if len(s) > maxLabelLen {
		s = s[:maxLabelLen-3] + "..."
	}
	return s
}

// coverage-ignore: requires external graphviz installation - tested manually
func renderWithGraphviz(ctx context.Context, dot, outputPath, format string) error {
	_, err := exec.LookPath("dot")
	if err != nil {
		return errors.New("graphviz 'dot' command not found; install Graphviz or use -f dot")
	}

	cmd := exec.CommandContext(ctx, "dot", "-T"+format, "-o", outputPath)
	cmd.Stdin = strings.NewReader(dot)
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to render graph: %w", err)
	}

	out.WriteSuccess("Rendered graph to %s", outputPath)
	return nil
}

func init() {
	graphCmd.Flags().StringVarP(&graphOutput, "output", "o", "", "Output file (stdout if not specified)")
	graphCmd.Flags().StringVarP(&graphFormat, "format", "f", "", "Output format (dot, png, svg, pdf)")
	graphCmd.Flags().StringVar(&graphDirection, "direction", "tb", "Graph direction (tb=top-bottom, lr=left-right)")
	graphCmd.Flags().StringSliceVar(&graphTypes, "types", nil, "Filter by entity types (comma-separated)")

	rootCmd.AddCommand(graphCmd)
}
