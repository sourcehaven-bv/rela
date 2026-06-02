package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// GraphCmd exports the entity graph to Graphviz DOT format.
type GraphCmd struct {
	OutFile   string   `name:"file" help:"Output file (stdout if not specified)."`
	Format    string   `short:"f" help:"Output format (dot, png, svg, pdf)."`
	Direction string   `default:"tb" help:"Graph direction (tb=top-bottom, lr=left-right)."`
	Types     []string `help:"Filter by entity types (comma-separated)."`
}

// Run dispatches `rela graph`.
func (c *GraphCmd) Run(ctx context.Context, svc *cliServices) error {
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

	if len(c.Types) > 0 {
		typeSet := make(map[string]bool)
		for _, t := range c.Types {
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

	dot := generateDOT(meta, entities, edges, c.Direction)

	if c.OutFile == "" {
		fmt.Println(dot)
		return nil
	}

	if c.Format != "" && c.Format != "dot" {
		return renderWithGraphviz(ctx, dot, c.OutFile, c.Format)
	}

	return os.WriteFile(c.OutFile, []byte(dot), 0644)
}

func generateDOT(
	meta *metamodel.Metamodel,
	entities []*entity.Entity,
	edges []*entity.Relation,
	direction string,
) string {
	var sb strings.Builder

	dir := "TB"
	if direction == "lr" {
		dir = "LR"
	}

	sb.WriteString("digraph architecture {\n")
	fmt.Fprintf(&sb, "  rankdir=%s;\n", dir)
	sb.WriteString("  node [shape=box, style=filled];\n")
	sb.WriteString("\n")

	typeGroups := make(map[string][]*entity.Entity)
	for _, e := range entities {
		typeGroups[e.Type] = append(typeGroups[e.Type], e)
	}

	for entityType, group := range typeGroups {
		fmt.Fprintf(&sb, "  subgraph cluster_%s {\n", sanitizeDOTID(entityType))
		fmt.Fprintf(&sb, "    label=\"%ss\";\n", strings.ToUpper(entityType[:1])+entityType[1:])

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

	for _, edge := range edges {
		fmt.Fprintf(&sb, "  \"%s\" -> \"%s\" [label=\"%s\"];\n",
			edge.From, edge.To, edge.Type)
	}

	sb.WriteString("}\n")
	return sb.String()
}

const maxLabelLen = 40

// sanitizeDOTID returns a valid DOT identifier — replaces characters
// outside [A-Za-z0-9_] with '_'.
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
	if len(s) > maxLabelLen {
		s = s[:maxLabelLen-3] + "..."
	}
	return s
}

// coverage-ignore: requires external graphviz installation
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
