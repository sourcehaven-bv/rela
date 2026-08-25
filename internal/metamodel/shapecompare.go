package metamodel

import (
	"fmt"
	"slices"
	"strconv"
)

// ShapeTier classifies one schema-shape delta by its impact on stored data.
// The ordering is meaningful: a higher tier subsumes a lower one when a
// report is reduced to a single verdict.
type ShapeTier int

const (
	// TierAdditive deltas cannot invalidate stored content (new types, new
	// optional properties, new enum values, widenings, default-only edits).
	// A store may adopt the new shape silently.
	TierAdditive ShapeTier = iota
	// TierDrift deltas leave stored content stale but never broken:
	// deletions orphan data (GC territory), a new required property yields
	// soft warnings. A store adopts the new shape but the operator is told.
	TierDrift
	// TierMigration deltas mean stored values no longer fit the schema
	// (type/format changes, list flips, enum replacements, narrowings).
	// The store must not adopt the new shape until a migration runs.
	TierMigration
)

func (t ShapeTier) String() string {
	switch t {
	case TierAdditive:
		return "additive"
	case TierDrift:
		return "drift"
	case TierMigration:
		return "needs-migration"
	}
	return fmt.Sprintf("ShapeTier(%d)", int(t))
}

// ShapeDelta is one classified difference between two shape projections.
type ShapeDelta struct {
	Tier ShapeTier
	// Kind is a stable machine-readable delta kind (e.g. "property_removed",
	// "property_type_changed", "possible_property_rename"). The generator
	// switches on it.
	Kind string
	// Subject names what changed: "task", "task.status", "rel:implements",
	// "rel:implements.weight", or "type:status" for a named enum.
	Subject string
	// Detail is the human-readable explanation shown in gate notices.
	Detail string
	// Counterpart names the removed half of a possible-rename pair (the old
	// subject), for possible_property_rename / possible_entity_type_rename.
	Counterpart string
	// Removed/Added carry the value diff for enum_values_* kinds, so the
	// migration generator can draft a map_values stub without re-diffing.
	Removed []string
	Added   []string
}

// ShapeReport is the classified diff between two shape projections.
type ShapeReport struct {
	Deltas []ShapeDelta
}

// Tier reduces the report to a single verdict: the highest tier among the
// deltas. An empty report (identical shapes) is TierAdditive.
func (r ShapeReport) Tier() ShapeTier {
	verdict := TierAdditive
	for _, d := range r.Deltas {
		if d.Tier > verdict {
			verdict = d.Tier
		}
	}
	return verdict
}

// Compatible reports whether the target shape can be adopted without a
// migration (no needs-migration deltas).
func (r ShapeReport) Compatible() bool { return r.Tier() < TierMigration }

// ByTier returns the deltas of one tier, in report order.
func (r ShapeReport) ByTier(t ShapeTier) []ShapeDelta {
	var out []ShapeDelta
	for _, d := range r.Deltas {
		if d.Tier == t {
			out = append(out, d)
		}
	}
	return out
}

// CompareShapes classifies every difference between two shape projections.
// `from` is the shape the stored data conforms to; `to` is the live schema.
//
// The one inherent blind spot: a rename is indistinguishable from a
// delete+add. When a removed and an added property in the same entity type
// share type/list/format, the report carries an explicit
// possible_property_rename delta (drift tier) so the operator is told —
// but it cannot be more than a warning. Same for entity types of similar
// property shape.
func CompareShapes(from, to ShapeProjection) ShapeReport {
	var r ShapeReport

	compareEntityShapes(&r, from.Entities, to.Entities)
	compareRelationShapes(&r, from.Relations, to.Relations)
	compareNamedTypes(&r, from.Types, to.Types)

	return r
}

func compareEntityShapes(r *ShapeReport, from, to map[string]EntityShape) {
	var removedTypes []string
	for _, name := range sortedKeys(from) {
		if _, ok := to[name]; !ok {
			removedTypes = append(removedTypes, name)
			r.add(TierDrift, "entity_type_removed", name,
				fmt.Sprintf("entity type %q removed: existing entities become unknown to the schema (readable, but every write is rejected) until migrated or GC'd", name))
		}
	}
	for _, name := range sortedKeys(to) {
		if _, ok := from[name]; !ok {
			r.add(TierAdditive, "entity_type_added", name, fmt.Sprintf("entity type %q added", name))
			// A removed type whose property shape resembles this added one
			// may be a rename spelled as delete+add.
			for _, old := range removedTypes {
				if propertyShapesSimilar(from[old].Properties, to[name].Properties) {
					r.Deltas = append(r.Deltas, ShapeDelta{
						Tier: TierDrift, Kind: "possible_entity_type_rename", Subject: name, Counterpart: old,
						Detail: fmt.Sprintf("entity type %q removed and %q added with similar properties — if this is a rename, generate a migration (rela migrate gen) before the old entities are GC'd", old, name),
					})
				}
			}
		}
	}
	for _, name := range sortedKeys(from) {
		toShape, ok := to[name]
		if !ok {
			continue
		}
		compareProperties(r, name, from[name].Properties, toShape.Properties, nil)
	}
}

func compareRelationShapes(r *ShapeReport, from, to map[string]RelationShape) {
	for _, name := range sortedKeys(from) {
		if _, ok := to[name]; !ok {
			r.add(TierDrift, "relation_type_removed", "rel:"+name,
				fmt.Sprintf("relation type %q removed: existing relations become unknown to the schema until migrated or GC'd", name))
		}
	}
	for _, name := range sortedKeys(to) {
		if _, ok := from[name]; !ok {
			r.add(TierAdditive, "relation_type_added", "rel:"+name, fmt.Sprintf("relation type %q added", name))
		}
	}
	for _, name := range sortedKeys(from) {
		toRel, ok := to[name]
		if !ok {
			continue
		}
		fromRel := from[name]
		subject := "rel:" + name

		compareEndpointList(r, subject, "from", fromRel.From, toRel.From)
		compareEndpointList(r, subject, "to", fromRel.To, toRel.To)

		if fromRel.Symmetric != toRel.Symmetric {
			r.add(TierMigration, "relation_symmetry_changed", subject,
				fmt.Sprintf("relation %q symmetric flag changed %t → %t: the meaning of stored edges changes", name, fromRel.Symmetric, toRel.Symmetric))
		}
		compareCardinality(r, subject, "min_outgoing", fromRel.MinOutgoing, toRel.MinOutgoing, false)
		compareCardinality(r, subject, "max_outgoing", fromRel.MaxOutgoing, toRel.MaxOutgoing, true)
		compareCardinality(r, subject, "min_incoming", fromRel.MinIncoming, toRel.MinIncoming, false)
		compareCardinality(r, subject, "max_incoming", fromRel.MaxIncoming, toRel.MaxIncoming, true)

		if fromRel.Content && !toRel.Content {
			r.add(TierDrift, "relation_content_removed", subject,
				fmt.Sprintf("relation %q no longer supports body content: existing bodies are orphaned", name))
		} else if !fromRel.Content && toRel.Content {
			r.add(TierAdditive, "relation_content_added", subject, fmt.Sprintf("relation %q now supports body content", name))
		}

		compareProperties(r, name, fromRel.Properties, toRel.Properties, func(s string) string { return "rel:" + s })
	}
}

// compareEndpointList classifies a relation endpoint (from/to) change:
// pure widening is additive; any removal is a narrowing (stored relations
// may reference an endpoint type no longer allowed).
func compareEndpointList(r *ShapeReport, subject, side string, from, to []string) {
	if slices.Equal(from, to) {
		return
	}
	narrowed := false
	for _, t := range from {
		if !slices.Contains(to, t) {
			narrowed = true
			break
		}
	}
	if narrowed {
		r.add(TierMigration, "relation_endpoint_narrowed", subject,
			fmt.Sprintf("%s %s endpoints narrowed %v → %v: existing relations may reference disallowed types", subject, side, from, to))
		return
	}
	r.add(TierAdditive, "relation_endpoint_widened", subject,
		fmt.Sprintf("%s %s endpoints widened %v → %v", subject, side, from, to))
}

// compareCardinality classifies a min/max bound change using EFFECTIVE
// values: an absent min bound means 0, an absent max bound means unbounded.
// Loosening (higher max, lower min) is additive; a no-op like
// `min: unset -> 0` produces no delta at all; tightening means existing data
// may violate the bound.
func compareCardinality(r *ShapeReport, subject, bound string, from, to *int, isMax bool) {
	fromEff, toEff := effectiveBound(from, isMax), effectiveBound(to, isMax)
	if fromEff == toEff {
		return
	}
	loosened := (isMax && toEff > fromEff) || (!isMax && toEff < fromEff)
	if loosened {
		r.add(TierAdditive, "relation_cardinality_loosened", subject,
			fmt.Sprintf("%s %s loosened %s → %s", subject, bound, fmtIntPtr(from), fmtIntPtr(to)))
		return
	}
	r.add(TierMigration, "relation_cardinality_tightened", subject,
		fmt.Sprintf("%s %s tightened %s → %s: existing relations may violate the bound", subject, bound, fmtIntPtr(from), fmtIntPtr(to)))
}

// effectiveBound maps an absent bound to its semantic value: no minimum is 0,
// no maximum is unbounded.
func effectiveBound(p *int, isMax bool) int {
	if p != nil {
		return *p
	}
	if isMax {
		return int(^uint(0) >> 1) // effectively unbounded
	}
	return 0
}

func fmtIntPtr(p *int) string {
	if p == nil {
		return "unset"
	}
	return strconv.Itoa(*p)
}

// compareProperties diffs one property map. owner is the entity or relation
// type name; subjectFn optionally rewrites subjects (relations prefix "rel:").
func compareProperties(r *ShapeReport, owner string, from, to map[string]PropertyShape, subjectFn func(string) string) {
	subject := func(prop string) string {
		s := owner + "." + prop
		if subjectFn != nil {
			return subjectFn(s)
		}
		return s
	}

	var removed []string
	for _, pname := range sortedKeys(from) {
		if _, ok := to[pname]; !ok {
			removed = append(removed, pname)
			r.add(TierDrift, "property_removed", subject(pname),
				fmt.Sprintf("property %s removed: stored values are orphaned until migrated or GC'd", subject(pname)))
		}
	}
	for _, pname := range sortedKeys(to) {
		if _, ok := from[pname]; ok {
			continue
		}
		added := to[pname]
		if added.Required {
			r.add(TierDrift, "required_property_added", subject(pname),
				fmt.Sprintf("required property %s added: existing records lack it (soft warnings) until backfilled", subject(pname)))
		} else {
			r.add(TierAdditive, "property_added", subject(pname), fmt.Sprintf("property %s added", subject(pname)))
		}
		// Rename spelled as delete+add?
		for _, old := range removed {
			if samePropertyKernel(from[old], added) {
				r.Deltas = append(r.Deltas, ShapeDelta{
					Tier: TierDrift, Kind: "possible_property_rename", Subject: subject(pname), Counterpart: subject(old),
					Detail: fmt.Sprintf("property %s removed and %s added with the same shape — if this is a rename, generate a migration (rela migrate gen) before the old values are GC'd", subject(old), subject(pname)),
				})
			}
		}
	}
	for _, pname := range sortedKeys(from) {
		toProp, ok := to[pname]
		if !ok {
			continue
		}
		comparePropertyShape(r, subject(pname), from[pname], toProp)
	}
}

func comparePropertyShape(r *ShapeReport, subject string, from, to PropertyShape) {
	if from.Type != to.Type {
		r.add(TierMigration, "property_type_changed", subject,
			fmt.Sprintf("property %s type changed %q → %q: stored values must be converted", subject, from.Type, to.Type))
	}
	if from.Format != to.Format {
		r.add(TierMigration, "property_format_changed", subject,
			fmt.Sprintf("property %s format changed %q → %q: stored values must be converted", subject, from.Format, to.Format))
	}
	if from.List != to.List {
		r.add(TierMigration, "property_list_changed", subject,
			fmt.Sprintf("property %s list flag changed %t → %t: stored values must be restructured", subject, from.List, to.List))
	}
	if !from.Required && to.Required {
		r.add(TierDrift, "property_became_required", subject,
			fmt.Sprintf("property %s became required: existing records without it get soft warnings until backfilled", subject))
	} else if from.Required && !to.Required {
		r.add(TierAdditive, "property_became_optional", subject, fmt.Sprintf("property %s became optional", subject))
	}
	if from.Default != to.Default {
		// Defaults only affect future creates, never stored data (A7).
		r.add(TierAdditive, "property_default_changed", subject,
			fmt.Sprintf("property %s default changed %q → %q (future creates only)", subject, from.Default, to.Default))
	}
	if from.Computed != to.Computed {
		r.add(TierDrift, "property_computed_changed", subject,
			fmt.Sprintf("property %s computed expression changed: stored values must be recomputed", subject))
	}
	compareValueList(r, subject, from.Values, to.Values)
}

// compareValueList classifies an enum value-list change: additions widen,
// removals orphan stored values (drift), removals combined with additions
// look like value renames and need a map_values migration.
func compareValueList(r *ShapeReport, subject string, from, to []string) {
	if slices.Equal(from, to) {
		return
	}
	var removedVals, addedVals []string
	for _, v := range from {
		if !slices.Contains(to, v) {
			removedVals = append(removedVals, v)
		}
	}
	for _, v := range to {
		if !slices.Contains(from, v) {
			addedVals = append(addedVals, v)
		}
	}
	addValueDelta := func(tier ShapeTier, kind, detail string) {
		r.Deltas = append(r.Deltas, ShapeDelta{
			Tier: tier, Kind: kind, Subject: subject, Detail: detail,
			Removed: removedVals, Added: addedVals,
		})
	}
	switch {
	case len(removedVals) == 0 && len(addedVals) == 0:
		// Pure reorder: value identity unchanged, stored data unaffected.
		addValueDelta(TierAdditive, "enum_values_reordered", subject+" enum values reordered")
	case len(removedVals) == 0:
		addValueDelta(TierAdditive, "enum_values_added", fmt.Sprintf("%s enum values added: %v", subject, addedVals))
	case len(addedVals) == 0:
		addValueDelta(TierDrift, "enum_values_removed",
			fmt.Sprintf("%s enum values removed: %v — stored occurrences become invalid (soft warnings) until remapped or GC'd", subject, removedVals))
	default:
		addValueDelta(TierMigration, "enum_values_replaced",
			fmt.Sprintf("%s enum values replaced (removed %v, added %v): stored occurrences must be remapped (map_values)", subject, removedVals, addedVals))
	}
}

func compareNamedTypes(r *ShapeReport, from, to map[string][]string) {
	for _, name := range sortedKeys(from) {
		if _, ok := to[name]; !ok {
			// Impact on data shows up via the property-level deltas of any
			// property that referenced this type; the type's disappearance
			// itself orphans nothing.
			r.add(TierAdditive, "named_type_removed", "type:"+name, fmt.Sprintf("named type %q removed", name))
		}
	}
	for _, name := range sortedKeys(to) {
		if _, ok := from[name]; !ok {
			r.add(TierAdditive, "named_type_added", "type:"+name, fmt.Sprintf("named type %q added", name))
		}
	}
	for _, name := range sortedKeys(from) {
		if _, ok := to[name]; !ok {
			continue
		}
		compareValueList(r, "type:"+name, from[name], to[name])
	}
}

// samePropertyKernel reports whether two property shapes agree on the fields
// that make a delete+add pair look like a rename (type, list, format).
func samePropertyKernel(a, b PropertyShape) bool {
	return a.Type == b.Type && a.List == b.List && a.Format == b.Format && a.Computed == b.Computed
}

// propertyShapesSimilar reports whether two property maps share at least half
// of the larger map's property names with matching kernels — the heuristic
// for "a removed and an added entity type might be the same type renamed".
func propertyShapesSimilar(a, b map[string]PropertyShape) bool {
	larger := max(len(a), len(b))
	if larger == 0 {
		return true // two property-less types look alike by definition
	}
	shared := 0
	for name, ap := range a {
		if bp, ok := b[name]; ok && samePropertyKernel(ap, bp) {
			shared++
		}
	}
	return shared*2 >= larger
}

func (r *ShapeReport) add(tier ShapeTier, kind, subject, detail string) {
	r.Deltas = append(r.Deltas, ShapeDelta{Tier: tier, Kind: kind, Subject: subject, Detail: detail})
}
