package rollup

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// A roll-up over roll-up (design section 10) is an along or a via with
// condition whose node condition reads roll-ups of the type it judges. The
// dependency graph has one node per (type, roll-up property) and an edge
// from an outer roll-up to each inner roll-up it reads. [Compile] refuses a
// cycle in it, a roll-up reading itself included, and a chain longer than
// one edge: one level of nesting only.

// Ref names one roll-up property of one entity type.
type Ref struct {
	Type, Property string
}

func (r Ref) String() string { return r.Type + "." + r.Property }

// rollupGraph is the dependency graph of a compiled set: the inner
// roll-ups each roll-up reads.
type rollupGraph map[Ref][]Ref

// buildGraph collects the edges of every compiled column.
func buildGraph(byType map[string][]Column) rollupGraph {
	g := rollupGraph{}
	for typ, cols := range byType {
		for _, col := range cols {
			if inner := col.Inner(); len(inner) > 0 {
				g[Ref{Type: typ, Property: col.Property}] = inner
			}
		}
	}
	return g
}

// Inner is the roll-ups the column's node conditions read, sorted by type
// then property. Empty for a roll-up that reads none.
func (c Column) Inner() []Ref {
	if c.Along == nil {
		return nil
	}
	var out []Ref
	for _, typ := range slices.Sorted(maps.Keys(c.Along.Nodes)) {
		for _, p := range c.Along.Nodes[typ].Rollups {
			out = append(out, Ref{Type: typ, Property: p})
		}
	}
	return out
}

// problems reports each cycle and each chain deeper than one level, once,
// as the message of the outer roll-up that starts it.
func (g rollupGraph) problems() []string {
	var out []string
	inCycle := map[Ref]bool{}
	for _, start := range g.sortedNodes() {
		if cycle := g.cycleFrom(start); cycle != nil && !inCycle[start] {
			for _, r := range cycle {
				inCycle[r] = true
			}
			out = append(out, fmt.Sprintf("entity %q property %q: rollup reads itself through a cycle: %s",
				start.Type, start.Property, chainText(cycle)))
		}
	}
	for _, outer := range g.sortedNodes() {
		for _, inner := range g[outer] {
			deeper := g[inner]
			if inCycle[outer] || inCycle[inner] || len(deeper) == 0 {
				continue
			}
			out = append(out, fmt.Sprintf(
				"entity %q property %q: rollup nests roll-ups more than one level deep: %s",
				outer.Type, outer.Property, chainText([]Ref{outer, inner, deeper[0]})))
		}
	}
	return out
}

// cycleFrom returns a path from start back to start, or nil.
func (g rollupGraph) cycleFrom(start Ref) []Ref {
	seen := map[Ref]bool{}
	var walk func(at Ref, path []Ref) []Ref
	walk = func(at Ref, path []Ref) []Ref {
		for _, next := range g[at] {
			if next == start {
				return append(slices.Clone(path), start)
			}
			if seen[next] {
				continue
			}
			seen[next] = true
			if found := walk(next, append(path, next)); found != nil {
				return found
			}
		}
		return nil
	}
	return walk(start, []Ref{start})
}

func (g rollupGraph) sortedNodes() []Ref {
	return slices.SortedFunc(maps.Keys(g), func(a, b Ref) int {
		return strings.Compare(a.String(), b.String())
	})
}

func chainText(chain []Ref) string {
	parts := make([]string, len(chain))
	for i, r := range chain {
		parts[i] = r.String()
	}
	return strings.Join(parts, " -> ")
}

// columnTypes is every entity type whose rows or edges col reads directly:
// the source type, each type a walk can reach or a path passes, and both
// end types of every relation walked. An inner roll-up's types are added
// by [Set.ReadTypes].
func columnTypes(meta *metamodel.Metamodel, typeName string, col Column) []string {
	types := []string{typeName}
	addRel := func(rel string) {
		if def, ok := meta.GetRelationDef(rel); ok {
			types = append(types, def.From...)
			types = append(types, def.To...)
		}
	}
	switch {
	case col.Along != nil:
		for _, seg := range col.Along.Segments {
			types = append(types, seg.Reaches...)
			addRel(seg.Relation)
		}
	case col.Siblings != nil:
		types = append(types, col.Siblings.Parents...)
		types = append(types, col.Siblings.Members...)
		addRel(col.Siblings.Relation)
	case col.Relation != "":
		for _, h := range col.Path {
			types = append(types, h.Target)
			addRel(h.Relation)
		}
		types = append(types, col.Target)
		addRel(col.Relation)
	}
	slices.Sort(types)
	return slices.Compact(types)
}

// ReadTypes is every entity type a change to which can change a roll-up of
// entityType, through its nested roll-ups too, sorted. A live view of
// entityType that shows its roll-ups refreshes on a change to any of them
// (design decision Q8); a change to an edge reaches the view as a change
// to the relation's end types. Empty when entityType has no roll-ups.
//
// Nil: accepted; a nil Set has no roll-ups.
func (s *Set) ReadTypes(entityType string) []string {
	if s == nil {
		return nil
	}
	var out []string
	done := map[Ref]bool{}
	var add func(typ string, col Column)
	add = func(typ string, col Column) {
		ref := Ref{Type: typ, Property: col.Property}
		if done[ref] {
			return
		}
		done[ref] = true
		out = append(out, col.Types...)
		for _, in := range col.Inner() {
			if inner, ok := s.column(in); ok {
				add(in.Type, inner)
			}
		}
	}
	for _, col := range s.For(entityType) {
		add(entityType, col)
	}
	slices.Sort(out)
	return slices.Compact(out)
}

// column looks up one compiled roll-up.
func (s *Set) column(r Ref) (Column, bool) {
	for _, c := range s.For(r.Type) {
		if c.Property == r.Property {
			return c, true
		}
	}
	return Column{}, false
}
