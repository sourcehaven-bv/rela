package transform

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// RelationGroup is one block of already-resolved, already-authorized relations
// for the entity being rendered: a display label and the visible neighbor
// titles. The caller (which owns the ACL read + neighbor gate) resolves these;
// the renderer only formats them. Keeping resolution outside the renderer is
// what lets the built-in entity renderer stay free of store/ACL dependencies.
type RelationGroup struct {
	Label     string
	Neighbors []string
}

// EntityRenderer renders a single entity to markdown: an H1 title, a property
// table (in metamodel-defined order), the relation groups, then the body.
//
// It performs no store read and no ACL decision — it operates on the entity and
// relation groups it is given, which the caller has already authorized. This is
// the built-in [Renderer] used by CLI export and by data-entry when no per-type
// Lua render override is configured.
type EntityRenderer struct {
	Entity    *entity.Entity
	Meta      *metamodel.Metamodel
	Relations []RelationGroup
}

// Render implements [Renderer].
func (er EntityRenderer) Render(_ context.Context) ([]byte, error) {
	if er.Entity == nil {
		return nil, errors.New("transform: EntityRenderer requires an entity")
	}
	var b strings.Builder

	title := er.title()
	fmt.Fprintf(&b, "# %s\n\n", escapeInline(title))

	if props := er.propertyRows(); len(props) > 0 {
		b.WriteString("| Property | Value |\n| --- | --- |\n")
		for _, row := range props {
			fmt.Fprintf(&b, "| %s | %s |\n", escapeCell(row[0]), escapeCell(row[1]))
		}
		b.WriteString("\n")
	}

	for _, g := range er.Relations {
		if len(g.Neighbors) == 0 {
			continue
		}
		fmt.Fprintf(&b, "## %s\n\n", escapeInline(g.Label))
		for _, n := range g.Neighbors {
			fmt.Fprintf(&b, "- %s\n", escapeInline(n))
		}
		b.WriteString("\n")
	}

	if body := strings.TrimSpace(er.Entity.Content); body != "" {
		b.WriteString(body)
		b.WriteString("\n")
	}

	return []byte(b.String()), nil
}

// title returns the entity's display title, preferring the metamodel's
// DisplayTitle (which honors display_property). DisplayTitle returns the ID as
// its fallback sentinel; when that happens we still try the entity's own `title`
// property before settling on the ID, so an entity whose type doesn't declare a
// required title property (but carries one) still renders a human title.
func (er EntityRenderer) title() string {
	if er.Meta != nil {
		if t := er.Meta.DisplayTitle(er.Entity.ID, er.Entity.Type, er.Entity.Properties); t != "" && t != er.Entity.ID {
			return t
		}
	}
	if t := er.Entity.Title(); t != "" {
		return t
	}
	return er.Entity.ID
}

// propertyRows returns [name, value] pairs in metamodel order (falling back to
// the entity's own property map order for anything the metamodel doesn't list).
// The title property is omitted — it is already the H1.
func (er EntityRenderer) propertyRows() [][2]string {
	seen := make(map[string]bool)
	var rows [][2]string

	add := func(name string) {
		if name == "" || seen[name] || name == "title" {
			return
		}
		seen[name] = true
		v, ok := er.Entity.Properties[name]
		if !ok {
			return
		}
		rows = append(rows, [2]string{name, formatValue(v)})
	}

	if er.Meta != nil {
		if def, ok := er.Meta.GetEntityDef(er.Entity.Type); ok {
			for _, name := range def.GetPropertyOrder() {
				add(name)
			}
		}
	}
	// Any properties not covered by the metamodel order, appended stably.
	for name := range er.Entity.Properties {
		add(name)
	}
	return rows
}

// formatValue renders a property value as a single-line cell string. Lists are
// comma-joined; other scalars use their default Go formatting.
func formatValue(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case []any:
		parts := make([]string, 0, len(t))
		for _, e := range t {
			parts = append(parts, formatValue(e))
		}
		return strings.Join(parts, ", ")
	case []string:
		return strings.Join(t, ", ")
	default:
		return fmt.Sprintf("%v", t)
	}
}

// escapeInline neutralizes characters that would break inline markdown or inject
// structure (newlines collapse to spaces; pipe/backslash escaped).
func escapeInline(s string) string {
	s = strings.ReplaceAll(s, "\r\n", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "|", "\\|")
	return s
}

// escapeCell is escapeInline for use inside a table cell (same rules; the pipe
// escape is the load-bearing one so a value can't add columns).
func escapeCell(s string) string { return escapeInline(s) }
