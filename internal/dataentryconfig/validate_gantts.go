package dataentryconfig

import (
	"fmt"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/filter"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// validGanttMultiParent are the accepted multi-parent policies. "duplicate" is
// deliberately absent — see the [Gantt.MultiParent] doc for why it was cut.
var validGanttMultiParent = map[string]bool{"first": true, "error": true}

// validGanttOnCycle are the accepted cycle policies.
var validGanttOnCycle = map[string]bool{"error": true, "prune": true}

// Defaults applied at load so the wire value is never empty and the SPA does
// not have to re-implement them. MaxDepth matches the view-traversal cap
// (maxRecursionDepth in internal/dataentry) on purpose: the two walks bound
// the same graph, and different defaults would make one surface show subtrees
// the other silently cannot reach.
const (
	defaultGanttMultiParent  = "first"
	defaultGanttOnCycle      = "error"
	defaultGanttDefaultDepth = 2
	defaultGanttMaxDepth     = 10
	defaultGanttMaxNodes     = 2000
)

// validateGantts checks each gantt against the metamodel. Errors name the
// gantt and source type so an author can pinpoint the problem, and surface at
// config load rather than on first render.
func validateGantts(cfg *Config, meta *metamodel.Metamodel) []string {
	var errs []string

	for ganttID, g := range cfg.Gantts {
		errs = append(errs, validateGanttShell(ganttID, g, meta)...)

		if len(g.Sources) == 0 {
			errs = append(errs, fmt.Sprintf("gantt %q: must declare at least one source", ganttID))
			continue
		}
		for _, typeName := range sortedMapKeys(g.Sources) {
			errs = append(errs, validateGanttSource(ganttID, typeName, g.Sources[typeName], meta)...)
		}
		errs = append(errs, validateGanttFilterControls(ganttID, g, meta)...)
		errs = append(errs, validateGanttTooltip(ganttID, g, meta)...)
	}
	return errs
}

// validateGanttTooltip checks the hover-card fields. Same best-effort rule as
// calendar chips — a property must resolve on at least one source type —
// plus the property-only restriction the [GanttTooltip] doc explains.
func validateGanttTooltip(ganttID string, g Gantt, meta *metamodel.Metamodel) []string {
	var errs []string
	for i, f := range g.Tooltip.Fields {
		prefix := fmt.Sprintf("gantt %q: tooltip.fields[%d]", ganttID, i)
		if f.Relation != "" {
			errs = append(errs, prefix+": relation fields are not supported on gantt tooltips — "+
				"neighbor titles would bypass row-gating (use a property field)")
			continue
		}
		if f.Property == "" {
			errs = append(errs, prefix+": must specify a property")
			continue
		}
		found := false
		for typeName := range g.Sources {
			if entDef, ok := meta.GetEntityDef(typeName); ok {
				if _, ok := entDef.Properties[f.Property]; ok {
					found = true
					break
				}
			}
		}
		if !found {
			errs = append(errs, fmt.Sprintf(
				"%s: property %q not present on any source type", prefix, f.Property))
		}
	}
	return errs
}

// validateGanttShell checks the gantt-level settings that do not depend on a
// source: the hierarchy relation set, policies and caps.
func validateGanttShell(ganttID string, g Gantt, meta *metamodel.Metamodel) []string {
	var errs []string
	prefix := fmt.Sprintf("gantt %q", ganttID)

	if len(g.Hierarchy) == 0 {
		errs = append(errs, prefix+": 'hierarchy' must list at least one relation type")
	}
	seen := map[string]int{}
	for i, rel := range g.Hierarchy {
		if _, ok := meta.GetRelationDef(rel); !ok {
			errs = append(errs, fmt.Sprintf("%s: hierarchy[%d] references unknown relation type %q",
				prefix, i, rel))
		}
		// A duplicate is dead config at best; at worst it changes which
		// parent "sorts first" for multi-parent detection. Refuse rather
		// than depend on the runtime happening to tolerate it.
		if j, dup := seen[rel]; dup {
			errs = append(errs, fmt.Sprintf("%s: hierarchy[%d] duplicates hierarchy[%d] (%q)",
				prefix, i, j, rel))
		} else {
			seen[rel] = i
		}
	}

	// "duplicate" gets its own message: it is a value an author might
	// reasonably expect, so "not valid" alone would read as a typo check.
	if g.MultiParent == "duplicate" {
		errs = append(errs, prefix+": multi_parent \"duplicate\" is not supported — "+
			"rendering one node under two parents double-counts every roll-up above it "+
			"(valid: first, error)")
	} else if g.MultiParent != "" && !validGanttMultiParent[g.MultiParent] {
		errs = append(errs, fmt.Sprintf("%s: multi_parent %q is not valid (valid: %s)",
			prefix, g.MultiParent, strings.Join(sortedMapKeys(validGanttMultiParent), ", ")))
	}
	if g.OnCycle != "" && !validGanttOnCycle[g.OnCycle] {
		errs = append(errs, fmt.Sprintf("%s: on_cycle %q is not valid (valid: %s)",
			prefix, g.OnCycle, strings.Join(sortedMapKeys(validGanttOnCycle), ", ")))
	}

	if g.DefaultDepth < 0 {
		errs = append(errs, prefix+": default_depth must not be negative")
	}
	if g.MaxDepth < 0 {
		errs = append(errs, prefix+": max_depth must not be negative")
	}
	if g.MaxNodes < 0 {
		errs = append(errs, prefix+": max_nodes must not be negative")
	}

	// Compare the EFFECTIVE values (defaults applied), or `default_depth: 20`
	// with an unset max_depth would load clean and then silently never show
	// more than 10 levels — the author's stated intent unachievable with no
	// signal.
	effDefault, effMax := g.DefaultDepth, g.MaxDepth
	if effDefault == 0 {
		effDefault = defaultGanttDefaultDepth
	}
	if effMax == 0 {
		effMax = defaultGanttMaxDepth
	}
	if effDefault > effMax {
		errs = append(errs, fmt.Sprintf(
			"%s: default_depth (%d) exceeds max_depth (%d) — levels past max_depth are never sent",
			prefix, effDefault, effMax))
	}
	return errs
}

// validateGanttSource checks one type's date-role mapping against the
// metamodel.
func validateGanttSource(ganttID, typeName string, src GanttSource, meta *metamodel.Metamodel) []string {
	var errs []string
	prefix := fmt.Sprintf("gantt %q: source %q", ganttID, typeName)

	entDef, ok := meta.GetEntityDef(typeName)
	if !ok {
		return append(errs, fmt.Sprintf("%s: unknown entity type %q", prefix, typeName))
	}

	for _, role := range []struct{ key, prop string }{
		{"start", src.Start},
		{"end", src.End},
		{"committed", src.Committed},
	} {
		if role.prop == "" {
			continue
		}
		def, ok := entDef.Properties[role.prop]
		if !ok {
			errs = append(errs, fmt.Sprintf("%s: %s property %q not in metamodel for entity %q",
				prefix, role.key, role.prop, typeName))
			continue
		}
		errs = append(errs, validateGanttDateProperty(prefix, role.key, role.prop, def)...)
	}

	if src.Label != "" {
		if _, ok := entDef.Properties[src.Label]; !ok {
			errs = append(errs, fmt.Sprintf("%s: label property %q not in metamodel for entity %q",
				prefix, src.Label, typeName))
		}
	} else if entDef.GetPrimaryProperty() == "" {
		errs = append(errs, fmt.Sprintf(
			"%s: 'label' omitted and entity %q has no display property to fall back to",
			prefix, typeName))
	}

	// Where: each clause must parse and reference a real property. A clause
	// that fails to parse is an ERROR, not a skip — dropping a filter widens
	// the view, which is the wrong failure direction. (The view-traverse
	// pipeline currently skips unparseable clauses; that is a bug to not
	// replicate, not a precedent.)
	for j, clause := range src.Where {
		f, err := filter.Parse(clause)
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s: where[%d] %q: %v", prefix, j, clause, err))
			continue
		}
		if entity.IsEntityPropertyKey(f.Property) {
			if _, ok := entDef.Properties[f.Property]; !ok {
				errs = append(errs, fmt.Sprintf("%s: where[%d] references unknown property %q",
					prefix, j, f.Property))
			}
		}
	}

	errs = append(errs, validateCalendarColor(src.Color, prefix)...)

	return errs
}

// validateGanttDateProperty checks that a property named as a gantt date role
// is usable as one: right type and single-valued. Unlike a calendar there is
// no format restriction — the gantt never writes a date back, so any parseable
// format the store holds is fine.
func validateGanttDateProperty(prefix, key, name string, def metamodel.PropertyDef) []string {
	var errs []string

	if !isFeedDateType(def.Type) {
		return append(errs, fmt.Sprintf("%s: %s property %q must be date- or datetime-typed, is %q",
			prefix, key, name, def.Type))
	}
	// A multi-valued date has no single position on a time axis.
	if def.List {
		errs = append(errs, fmt.Sprintf(
			"%s: %s property %q is a list; a gantt needs a single date per role",
			prefix, key, name))
	}
	return errs
}

// validateGanttFilterControls checks interactive filters against the sources'
// types. A control must resolve on at least one source type — sources are
// heterogeneous, so requiring every control to exist on every type would make
// a mixed gantt nearly unconfigurable — but one that resolves nowhere is dead
// config. (The calendar omits this validation entirely; that gap is not worth
// replicating.)
func validateGanttFilterControls(ganttID string, g Gantt, meta *metamodel.Metamodel) []string {
	var errs []string

	for i, fc := range g.FilterControls {
		if fc.Property == "" && fc.Relation == "" {
			errs = append(errs, fmt.Sprintf(
				"gantt %q: filter_controls[%d] must specify either property or relation",
				ganttID, i))
			continue
		}
		if fc.Property != "" {
			found := false
			for typeName := range g.Sources {
				if entDef, ok := meta.GetEntityDef(typeName); ok {
					if _, ok := entDef.Properties[fc.Property]; ok {
						found = true
						break
					}
				}
			}
			if !found {
				errs = append(errs, fmt.Sprintf(
					"gantt %q: filter_controls[%d] references property %q not present on any source type",
					ganttID, i, fc.Property))
			}
		}
		if fc.Relation != "" {
			if _, ok := meta.GetRelationDef(fc.Relation); !ok {
				errs = append(errs, fmt.Sprintf(
					"gantt %q: filter_controls[%d] references unknown relation %q",
					ganttID, i, fc.Relation))
			}
		}
	}
	return errs
}

// NormalizeGantts fills in gantt defaults after load so the wire value is
// never empty and the SPA has one source of truth for them.
func NormalizeGantts(cfg *Config) {
	for id, g := range cfg.Gantts {
		if g.MultiParent == "" {
			g.MultiParent = defaultGanttMultiParent
		}
		if g.OnCycle == "" {
			g.OnCycle = defaultGanttOnCycle
		}
		if g.DefaultDepth == 0 {
			g.DefaultDepth = defaultGanttDefaultDepth
		}
		if g.MaxDepth == 0 {
			g.MaxDepth = defaultGanttMaxDepth
		}
		if g.MaxNodes == 0 {
			g.MaxNodes = defaultGanttMaxNodes
		}
		cfg.Gantts[id] = g
	}
}
