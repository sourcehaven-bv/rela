package dataentryconfig

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/filter"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// defaultSankeyMaxNodes caps one response, like [Gantt.MaxNodes].
const defaultSankeyMaxNodes = 2000

// Sankey defines a flow diagram over typed entities: one column per entity
// type (or group of types), and the relations between neighboring columns
// drawn as ribbons. Every link counts 1.
//
// A dedicated endpoint serves it, because the diagram needs the whole gated
// node set and the edges among them in one answer. The layout (ordering,
// heights, ribbons) is computed in the SPA.
type Sankey struct {
	// Title is the heading shown above the diagram. Defaults to the config key.
	Title string `yaml:"title" json:"title"`
	// Header and Footer are admin-authored markdown above and below the diagram.
	Header string `yaml:"header,omitempty" json:"header,omitempty"`
	Footer string `yaml:"footer,omitempty" json:"footer,omitempty"`
	// Columns lists the entity types of each column, left to right. Types that
	// share a column stack top to bottom in the listed order. A type appears
	// once. Required, at least two columns.
	Columns [][]string `yaml:"columns" json:"columns"`
	// Edges lists the relation types drawn as ribbons. A relation counts when
	// both ends are in the diagram, whichever way it points; the diagram
	// always runs it from the lower column to the higher one. Required.
	Edges []string `yaml:"edges" json:"edges"`
	// Types maps an entity type in Columns to how its nodes look and order.
	// A type absent here still renders: titled by its display property, in the
	// order the diagram's crossing minimiser finds.
	Types map[string]SankeyType `yaml:"types,omitempty" json:"types,omitempty"`
	// MaxNodes caps the nodes of one response (default 2000). Exceeding it sets
	// the response's truncated flag rather than failing.
	MaxNodes int `yaml:"max_nodes,omitempty" json:"max_nodes"`
}

// SankeyType configures the nodes of one entity type.
type SankeyType struct {
	// Where restricts which entities of the type join, in the internal/filter
	// language, all ANDed. A clause that does not parse is a load error.
	Where []string `yaml:"where,omitempty" json:"where,omitempty"`
	// Label names the property used as the node label. Defaults to the type's
	// display property.
	Label string `yaml:"label,omitempty" json:"label,omitempty"`
	// Order pins the type's nodes to the ascending value of this property
	// (compared as text, then by id), instead of the crossing minimiser. A node
	// without the property sorts first; text order puts "1.10" before "1.2".
	// "id" orders by entity id.
	Order string `yaml:"order,omitempty" json:"order,omitempty"`
	// Color is a named palette token (see [ValidCalendarColors]) for the
	// type's nodes.
	Color string `yaml:"color,omitempty" json:"color,omitempty"`
	// ColorBy names a property whose value picks the node's tone. The
	// response carries the value; the SPA maps equal values to equal tones.
	ColorBy string `yaml:"color_by,omitempty" json:"color_by,omitempty"`
	// Pips shows the entities related to each node as small dots beside it.
	// Nil: no pips.
	Pips *SankeyPips `yaml:"pips,omitempty" json:"pips,omitempty"`
}

// SankeyPips configures the dots drawn for a node's related entities, for
// example the evidence of a control. They are not diagram nodes: they carry no
// ribbon and do not count against max_nodes.
type SankeyPips struct {
	// Relation is the relation type that links a node to its pips, in either
	// direction. Required.
	Relation string `yaml:"relation" json:"relation"`
	// Type is the entity type of the pips. Required; it differs from the
	// node's own type.
	Type string `yaml:"type" json:"type"`
	// Label names the pip type's property shown on hover. Defaults to its
	// display property.
	Label string `yaml:"label,omitempty" json:"label,omitempty"`
	// ColorBy names a property of the pip type whose value picks the dot's tone.
	ColorBy string `yaml:"color_by,omitempty" json:"color_by,omitempty"`
	// Max is the number of dots drawn per node (default 8, at most 50). The
	// rest are reported as a count.
	Max int `yaml:"max,omitempty" json:"max"`
}

const (
	defaultSankeyPipsMax = 8
	maxSankeyPipsMax     = 50
)

// validateSankeys checks each sankey against the metamodel, at config load.
func validateSankeys(cfg *Config, meta *metamodel.Metamodel) []string {
	errs := make([]string, 0, len(cfg.Sankeys))
	for _, id := range slices.Sorted(maps.Keys(cfg.Sankeys)) {
		errs = append(errs, validateSankey(id, cfg.Sankeys[id], meta)...)
	}
	return errs
}

func validateSankey(id string, s Sankey, meta *metamodel.Metamodel) []string {
	var errs []string
	prefix := fmt.Sprintf("sankey %q", id)
	if len(s.Columns) < 2 {
		errs = append(errs, prefix+": 'columns' needs at least two columns")
	}
	if s.MaxNodes < 0 {
		errs = append(errs, prefix+": max_nodes must not be negative")
	}
	inColumns := map[string]bool{}
	for c, col := range s.Columns {
		if len(col) == 0 {
			errs = append(errs, fmt.Sprintf("%s: columns[%d] is empty", prefix, c))
		}
		for _, typeName := range col {
			if _, ok := meta.GetEntityDef(typeName); !ok {
				errs = append(errs, fmt.Sprintf("%s: columns[%d] references unknown entity type %q", prefix, c, typeName))
			}
			if inColumns[typeName] {
				errs = append(errs, fmt.Sprintf("%s: entity type %q appears twice in columns", prefix, typeName))
			}
			inColumns[typeName] = true
		}
	}
	if len(s.Edges) == 0 {
		errs = append(errs, prefix+": 'edges' must list at least one relation type")
	}
	seen := map[string]bool{}
	for i, rel := range s.Edges {
		if _, ok := meta.GetRelationDef(rel); !ok {
			errs = append(errs, fmt.Sprintf("%s: edges[%d] references unknown relation type %q", prefix, i, rel))
		}
		if seen[rel] {
			errs = append(errs, fmt.Sprintf("%s: edges[%d] duplicates relation type %q", prefix, i, rel))
		}
		seen[rel] = true
	}
	for _, typeName := range slices.Sorted(maps.Keys(s.Types)) {
		if !inColumns[typeName] {
			errs = append(errs, fmt.Sprintf("%s: types.%s is not in columns", prefix, typeName))
			continue
		}
		errs = append(errs, validateSankeyType(prefix, typeName, s.Types[typeName], meta)...)
	}
	return errs
}

func validateSankeyType(prefix, typeName string, t SankeyType, meta *metamodel.Metamodel) []string {
	var errs []string
	prefix = fmt.Sprintf("%s: types.%s", prefix, typeName)
	entDef, ok := meta.GetEntityDef(typeName)
	if !ok {
		return nil // reported for columns already
	}
	for _, p := range []struct{ key, prop string }{{"label", t.Label}, {"color_by", t.ColorBy}} {
		if p.prop == "" {
			continue
		}
		if _, ok := entDef.Properties[p.prop]; !ok {
			errs = append(errs, fmt.Sprintf("%s: %s property %q not in metamodel for entity %q",
				prefix, p.key, p.prop, typeName))
		}
	}
	if t.Order != "" && t.Order != "id" {
		if _, ok := entDef.Properties[t.Order]; !ok {
			errs = append(errs, fmt.Sprintf("%s: order property %q not in metamodel for entity %q",
				prefix, t.Order, typeName))
		}
	}
	if t.Label == "" && entDef.GetPrimaryProperty() == "" {
		errs = append(errs, prefix+": 'label' omitted and the entity type has no display property")
	}
	// A clause that fails to parse is an error, not a skip: dropping a filter
	// widens the diagram, which is the wrong failure direction.
	for j, clause := range t.Where {
		f, err := filter.Parse(clause)
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s: where[%d] %q: %v", prefix, j, clause, err))
			continue
		}
		if entity.IsEntityPropertyKey(f.Property) {
			if _, ok := entDef.Properties[f.Property]; !ok {
				errs = append(errs, fmt.Sprintf("%s: where[%d] references unknown property %q", prefix, j, f.Property))
			}
		}
	}
	errs = append(errs, validateCalendarColor(t.Color, prefix)...)
	errs = append(errs, validateSankeyPips(prefix, typeName, t.Pips, meta)...)
	return errs
}

func validateSankeyPips(prefix, typeName string, p *SankeyPips, meta *metamodel.Metamodel) []string {
	if p == nil {
		return nil
	}
	prefix += ".pips"
	var errs []string
	relDef, relOK := meta.GetRelationDef(p.Relation)
	if !relOK {
		errs = append(errs, fmt.Sprintf("%s: relation %q is not in the metamodel", prefix, p.Relation))
	}
	pipDef, typeOK := meta.GetEntityDef(p.Type)
	if !typeOK {
		errs = append(errs, fmt.Sprintf("%s: type %q is not in the metamodel", prefix, p.Type))
	}
	if p.Type == typeName {
		errs = append(errs, prefix+": type must differ from the node's own type")
	}
	if relOK && typeOK && !relationJoins(relDef, typeName, p.Type) {
		errs = append(errs, fmt.Sprintf("%s: relation %q does not join %q and %q", prefix, p.Relation, typeName, p.Type))
	}
	if p.Max < 0 || p.Max > maxSankeyPipsMax {
		errs = append(errs, fmt.Sprintf("%s: max must be between 0 and %d", prefix, maxSankeyPipsMax))
	}
	if typeOK {
		for _, q := range []struct{ key, prop string }{{"label", p.Label}, {"color_by", p.ColorBy}} {
			if q.prop == "" {
				continue
			}
			if _, ok := pipDef.Properties[q.prop]; !ok {
				errs = append(errs, fmt.Sprintf("%s: %s property %q not in metamodel for entity %q",
					prefix, q.key, q.prop, p.Type))
			}
		}
		if p.Label == "" && pipDef.GetPrimaryProperty() == "" {
			errs = append(errs, prefix+": 'label' omitted and the entity type has no display property")
		}
	}
	return errs
}

// relationJoins reports whether rel connects types a and b in either direction.
func relationJoins(rel *metamodel.RelationDef, a, b string) bool {
	return (slices.Contains(rel.From, a) && slices.Contains(rel.To, b)) ||
		(slices.Contains(rel.From, b) && slices.Contains(rel.To, a))
}

// NormalizeSankeys fills in defaults after load so the wire value is never
// empty and the SPA does not re-implement them.
func NormalizeSankeys(cfg *Config) {
	for id, s := range cfg.Sankeys {
		if s.MaxNodes == 0 {
			s.MaxNodes = defaultSankeyMaxNodes
		}
		if strings.TrimSpace(s.Title) == "" {
			s.Title = id
		}
		for name, t := range s.Types {
			if t.Pips != nil && t.Pips.Max == 0 {
				pips := *t.Pips
				pips.Max = defaultSankeyPipsMax
				t.Pips = &pips
				s.Types[name] = t
			}
		}
		cfg.Sankeys[id] = s
	}
}
