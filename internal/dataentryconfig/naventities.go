package dataentryconfig

import (
	"fmt"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// EffectiveNavSort is the order an `entities:` navigation entry lists its
// links in: the entry's own `sort:`, else the entity type's `default_sort`,
// else nil (the list endpoint's id order).
//
// One helper because two consumers must agree on it: the sidebar wire, which
// tells the SPA what to request, and the derived-index planner, which builds
// the index for that request. Resolving the fallback in only one of them
// derives an index for a query nobody sends.
//
// Nil: meta accepted — an unknown type (or no metamodel) has no default_sort,
// so the entry's own sort is the whole answer.
func EffectiveNavSort(nav NavigationEntry, meta *metamodel.Metamodel) []SortSpec {
	if len(nav.Sort) > 0 {
		return nav.Sort
	}
	if meta == nil {
		return nil
	}
	def, ok := meta.GetEntityDef(nav.Entities)
	if !ok {
		return nil
	}
	return def.DefaultSort
}

// SortParam renders sort specs in the list endpoint's `sort=` grammar:
// comma-separated properties, a leading `-` for descending.
func SortParam(specs []SortSpec) string {
	parts := make([]string, 0, len(specs))
	for _, s := range specs {
		if s.Property == "" {
			continue
		}
		if s.IsDescending() {
			parts = append(parts, "-"+s.Property)
			continue
		}
		parts = append(parts, s.Property)
	}
	return strings.Join(parts, ",")
}

// navEntryDestinations reports which destination kinds an entry sets, for the
// exclusivity rule on `entities:`.
func navEntryDestinations(nav NavigationEntry) []string {
	var kinds []string
	for _, k := range []struct {
		name string
		set  bool
	}{
		{"list", nav.List != ""},
		{"dashboard", nav.Dashboard},
		{"kanban", nav.Kanban != ""},
		{"calendar", nav.Calendar != ""},
		{"gantt", nav.Gantt != ""},
		{"search", nav.Search},
		{"settings", nav.Settings},
		{"action", nav.Action != ""},
		{"document", nav.Document != ""},
		{"group", nav.IsGroup()},
	} {
		if k.set {
			kinds = append(kinds, k.name)
		}
	}
	return kinds
}

// validateNavEntitiesShape checks the metamodel-independent rules for an
// `entities:` entry, and that its companion keys appear nowhere else.
//
// The exclusivity check covers `entities:` only. Other kinds have never been
// checked for it (TKT-VKM63H tracks the general case); an `entities:` entry
// combined with another kind would render one and silently drop the other.
func validateNavEntitiesShape(nav NavigationEntry) []string {
	if nav.Entities == "" {
		var errs []string
		name := nav.Label
		if name == "" {
			name = nav.Group
		}
		if nav.QueryScope != "" {
			errs = append(errs, fmt.Sprintf(
				"navigation %q: query_scope is only valid on an entities: entry", name))
		}
		if len(nav.Sort) > 0 {
			errs = append(errs, fmt.Sprintf(
				"navigation %q: sort is only valid on an entities: entry", name))
		}
		return errs
	}

	var errs []string
	if kinds := navEntryDestinations(nav); len(kinds) > 0 {
		errs = append(errs, fmt.Sprintf(
			"navigation entities %q: cannot be combined with %s (an entry has exactly one kind)",
			nav.Entities, strings.Join(kinds, ", ")))
	}
	if nav.Label != "" {
		errs = append(errs, fmt.Sprintf(
			"navigation entities %q: label is not allowed (each link is labeled with its entity's display name; "+
				"use the enclosing group's title as the heading)", nav.Entities))
	}
	return errs
}

// validateNavEntitiesPlacement rejects an `entities:` entry at the top level.
// Its links need a heading to read as a set, and the group title is that
// heading.
func validateNavEntitiesPlacement(top []NavigationEntry) []string {
	var errs []string
	for _, nav := range top {
		if nav.Entities != "" && !nav.IsGroup() {
			errs = append(errs, fmt.Sprintf(
				"navigation entities %q: must be inside a group (the group title is the heading for its links)",
				nav.Entities))
		}
	}
	return errs
}

// validateNavEntities checks every `entities:` entry against the metamodel:
// the type exists and the sort names real properties. Scope references are
// checked by [validateQueryScopes] with the other views.
func validateNavEntities(cfg *Config, meta *metamodel.Metamodel) []string {
	if cfg == nil || meta == nil {
		return nil
	}
	var errs []string
	for _, nav := range NavEntitiesEntries(cfg.Navigation) {
		def, ok := meta.GetEntityDef(nav.Entities)
		if !ok {
			errs = append(errs, fmt.Sprintf(
				"navigation entities %q: unknown entity type %q", nav.Entities, nav.Entities))
			continue
		}
		errs = append(errs, validateSortSpecs(
			fmt.Sprintf("navigation entities %q", nav.Entities), def, nav.Sort)...)
	}
	return errs
}

// NavEntitiesEntries returns every `entities:` entry, at any depth the
// navigation allows. Exported for the derived-index planner, which must see
// exactly the entries the sidebar serves.
func NavEntitiesEntries(navs []NavigationEntry) []NavigationEntry {
	var out []NavigationEntry
	for _, nav := range navs {
		if nav.Entities != "" {
			out = append(out, nav)
		}
		out = append(out, NavEntitiesEntries(nav.Items)...)
	}
	return out
}

// validateSortSpecs is the sort rule shared by lists and navigation entries:
// a known direction, and a property the type declares or one of the virtual
// `id` / `modified`.
func validateSortSpecs(context string, def *metamodel.EntityDef, specs []SortSpec) []string {
	var errs []string
	for i, s := range specs {
		if !validSortDirections[s.Direction] {
			errs = append(errs, fmt.Sprintf(
				"%s: sort[%d] has invalid direction %q (valid: asc, desc)",
				context, i, s.Direction))
		}
		if s.Property != "" && s.Property != "id" && s.Property != "modified" {
			if _, ok := def.Properties[s.Property]; !ok {
				errs = append(errs, fmt.Sprintf(
					"%s: sort[%d] references unknown property %q",
					context, i, s.Property))
			}
		}
	}
	return errs
}
