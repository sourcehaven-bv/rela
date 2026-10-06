package dataentryconfig

import (
	"fmt"
	"slices"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// ValidGroupColors is the allowlist of section colors a list group may use.
// It mirrors the component library's StatusColor type
// (frontend/packages/rela-components/src/types/index.ts), which is what the
// SPA hands the section heading. A token rather than a CSS value, for the
// reason ValidCalendarColors gives.
var ValidGroupColors = map[string]bool{
	"green": true,
	"amber": true,
	"red":   true,
	"grey":  true,
	"blue":  true,
}

// DateBucketKeys are the bucket keys `labels:` may override, in the order the
// sections appear. The SPA's utils/dateBuckets.ts holds the same set.
var DateBucketKeys = []string{"overdue", "today", "tomorrow", "next_7_days", "later", "no_date"}

// bucketsRelative is the one bucketing scheme.
const bucketsRelative = "relative"

// Row caps for a grouped list. The default matches the size a reader can
// still scan in sections; the ceiling keeps one page load bounded, since a
// grouped list is fetched whole rather than paged.
const (
	defaultGroupMaxRows = 500
	maxGroupMaxRows     = 2000
)

// validateListsGroupBy checks every list's `group_by:`. A list whose entity
// type is unknown is skipped here; validateLists already reports it.
func validateListsGroupBy(cfg *Config, meta *metamodel.Metamodel) []string {
	var errs []string
	for listID, list := range cfg.Lists {
		entDef, ok := meta.GetEntityDef(list.EntityType)
		if !ok {
			continue
		}
		errs = append(errs, validateListGroupBy(listID, list.EntityType, list.GroupBy, entDef, meta)...)
	}
	return errs
}

// validateListGroupBy checks a list's `group_by:` against the list's entity
// type. Nil is accepted and yields nothing: the list is flat.
func validateListGroupBy(
	listID, entityType string, g *ListGroupBy, entDef *metamodel.EntityDef, meta *metamodel.Metamodel,
) []string {
	if g == nil {
		return nil
	}
	prefix := fmt.Sprintf("list %q: group_by", listID)
	var errs []string

	if g.MaxRows < 0 || g.MaxRows > maxGroupMaxRows {
		errs = append(errs, fmt.Sprintf("%s: max_rows %d is out of range (1..%d)", prefix, g.MaxRows, maxGroupMaxRows))
	}
	if len(g.Groups) > 0 && g.Buckets != "" {
		errs = append(errs, prefix+": groups and buckets cannot both be set "+
			"(groups restyles enum values, buckets sorts dates into relative sections)")
	}
	if g.Buckets != "" && g.Buckets != bucketsRelative {
		errs = append(errs, fmt.Sprintf("%s: buckets %q is not valid (valid: %s)", prefix, g.Buckets, bucketsRelative))
	}
	if len(g.Labels) > 0 && g.Buckets == "" {
		errs = append(errs, prefix+": labels only apply with buckets")
	}
	for _, key := range sortedMapKeys(g.Labels) {
		if !slices.Contains(DateBucketKeys, key) {
			errs = append(errs, fmt.Sprintf("%s: labels has unknown bucket %q (valid: %s)",
				prefix, key, strings.Join(DateBucketKeys, ", ")))
		}
	}

	if g.Relation != "" {
		if g.Property != "" {
			errs = append(errs, prefix+": property and relation are mutually exclusive")
		}
		if len(g.Groups) > 0 || g.Buckets != "" {
			errs = append(errs, prefix+": groups and buckets do not apply to a relation")
		}
		return append(errs, validateRelationColumns(prefix, entityType, g.Relation, g.OfferedBy, g.OrderBy, meta)...)
	}
	if g.OfferedBy != "" || g.OrderBy != "" {
		errs = append(errs, prefix+": offered_by and order_by need relation")
	}
	if g.Property == "" {
		return append(errs, prefix+": property or relation is required")
	}
	propDef, ok := entDef.Properties[g.Property]
	if !ok {
		return append(errs, fmt.Sprintf("%s: property %q not in metamodel for entity %q",
			prefix, g.Property, entityType))
	}
	// A row with two values would belong to two sections. Showing it twice
	// double-counts every section total; showing it once picks a section
	// arbitrarily. Neither is a grouping.
	if propDef.List {
		errs = append(errs, fmt.Sprintf("%s: property %q is a list; grouping needs a single-valued property",
			prefix, g.Property))
	}

	if g.Buckets != "" && propDef.Type != metamodel.PropertyTypeDate && propDef.Type != metamodel.PropertyTypeDatetime {
		errs = append(errs, fmt.Sprintf("%s: buckets needs a date or datetime property, %q is %s",
			prefix, g.Property, propDef.Type))
	}
	errs = append(errs, validateListGroups(prefix, g, propDef, meta)...)
	return errs
}

// validateListGroups checks `groups:` entries: each value must be one the
// enum declares, named once, with a known color.
func validateListGroups(
	prefix string, g *ListGroupBy, propDef metamodel.PropertyDef, meta *metamodel.Metamodel,
) []string {
	if len(g.Groups) == 0 {
		return nil
	}
	values := GetValidEnumValues(propDef, meta)
	if len(values) == 0 {
		return []string{fmt.Sprintf("%s: groups needs an enum property, %q is %s", prefix, g.Property, propDef.Type)}
	}
	valid := make(map[string]bool, len(values))
	for _, v := range values {
		valid[v] = true
	}
	var errs []string
	seen := map[string]int{}
	for i, grp := range g.Groups {
		if !valid[grp.Value] {
			errs = append(errs, fmt.Sprintf("%s: groups[%d] value %q is not valid for %q (valid: %s)",
				prefix, i, grp.Value, g.Property, strings.Join(values, ", ")))
		}
		if j, dup := seen[grp.Value]; dup {
			errs = append(errs, fmt.Sprintf("%s: groups[%d] duplicates groups[%d] (%q)", prefix, i, j, grp.Value))
		} else {
			seen[grp.Value] = i
		}
		if grp.Color != "" && !ValidGroupColors[grp.Color] {
			errs = append(errs, fmt.Sprintf("%s: groups[%d] has unknown color %q (valid: %s)",
				prefix, i, grp.Color, strings.Join(sortedMapKeys(ValidGroupColors), ", ")))
		}
	}
	return errs
}

// NormalizeListGroupBy fills in the row cap after load, so the wire value is
// never zero and the SPA does not hold a second copy of the default.
func NormalizeListGroupBy(cfg *Config) {
	for id, list := range cfg.Lists {
		if list.GroupBy == nil || list.GroupBy.MaxRows != 0 {
			continue
		}
		g := *list.GroupBy
		g.MaxRows = defaultGroupMaxRows
		list.GroupBy = &g
		cfg.Lists[id] = list
	}
}
