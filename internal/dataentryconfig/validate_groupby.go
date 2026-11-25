package dataentryconfig

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// DateBucketKeys are the bucket keys `labels:` may override, in the order the
// sections appear. The SPA's utils/dateBuckets.ts holds the same set.
var DateBucketKeys = []string{"overdue", "today", "tomorrow", "next_7_days", "later", "no_date"}

// WeekBucketKeys are the bucket keys of the relative_weeks scheme, in order.
var WeekBucketKeys = []string{"overdue", "today", "tomorrow", "this_week", "next_week", "later", "no_date"}

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
	if len(g.Groups) > 0 && g.Buckets != nil {
		errs = append(errs, prefix+": groups and buckets cannot both be set "+
			"(groups restyles enum values, buckets sorts dates into date sections)")
	}
	if len(g.Labels) > 0 && g.Buckets == nil {
		errs = append(errs, prefix+": labels only apply with buckets")
	}
	if g.Buckets != nil {
		errs = append(errs, validateListBuckets(prefix, g)...)
	}

	if g.Relation != "" {
		if g.Property != "" {
			errs = append(errs, prefix+": property and relation are mutually exclusive")
		}
		if len(g.Groups) > 0 || g.Buckets != nil {
			errs = append(errs, prefix+": groups and buckets do not apply to a relation")
		}
		return append(errs, validateRelationColumns(prefix, entityType, relationColumnsSource{
			Relation: g.Relation, OfferedBy: g.OfferedBy, OrderBy: g.OrderBy, StyleFrom: g.StyleFrom,
		}, meta)...)
	}
	if g.OfferedBy != "" || g.OrderBy != "" || g.StyleFrom != "" {
		errs = append(errs, prefix+": offered_by, order_by and style_from need relation")
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

	isDate := propDef.Type == metamodel.PropertyTypeDate || propDef.Type == metamodel.PropertyTypeDatetime
	if g.Buckets != nil && !isDate {
		errs = append(errs, fmt.Sprintf("%s: buckets needs a date or datetime property, %q is %s",
			prefix, g.Property, propDef.Type))
	}
	errs = append(errs, validateListGroups(prefix, g, propDef, meta)...)
	return errs
}

// validateListBuckets checks `buckets:` and the `labels:` keys it allows.
func validateListBuckets(prefix string, g *ListGroupBy) []string {
	b := g.Buckets
	var errs []string
	if !b.Calendar() && b.Scheme != BucketsRelative && b.Scheme != BucketsRelativeWeeks {
		errs = append(errs, fmt.Sprintf("%s: buckets %q is not valid (valid: %s, %s, or a mapping with by)",
			prefix, b.Scheme, BucketsRelative, BucketsRelativeWeeks))
	}
	if b.Calendar() {
		errs = append(errs, validateCalendarBuckets(prefix, *b)...)
	}
	allowed := BucketLabelKeys(*b)
	for _, key := range slices.Sorted(maps.Keys(g.Labels)) {
		if !slices.Contains(allowed, key) {
			errs = append(errs, fmt.Sprintf("%s: labels has unknown bucket %q (valid: %s)",
				prefix, key, strings.Join(allowed, ", ")))
		}
	}
	return errs
}

// validateCalendarBuckets checks the mapping form of `buckets:`.
func validateCalendarBuckets(prefix string, b ListBuckets) []string {
	var errs []string
	units := strings.Join(BucketUnits, ", ")
	switch {
	case b.By == "":
		errs = append(errs, prefix+": buckets.by is required (valid: "+units+")")
	case !slices.Contains(BucketUnits, b.By):
		errs = append(errs, fmt.Sprintf("%s: buckets.by %q is not valid (valid: %s)", prefix, b.By, units))
	}
	if _, ok := b.ForDays(); !ok {
		errs = append(errs, fmt.Sprintf("%s: buckets.for %q is not valid (a number of days from 1 to %d, such as 7d)",
			prefix, b.For, maxBucketForDays))
	}
	if b.For != "" && b.By != BucketUnitDay {
		errs = append(errs, prefix+": buckets.for needs by: day")
	}
	if b.Then != "" {
		if b.For == "" {
			errs = append(errs, prefix+": buckets.then needs for")
		}
		if !slices.Contains(BucketUnits[1:], b.Then) {
			errs = append(errs, fmt.Sprintf("%s: buckets.then %q is not valid (valid: %s)",
				prefix, b.Then, strings.Join(BucketUnits[1:], ", ")))
		}
	}
	switch b.Order {
	case "", BucketOrderAsc:
	case BucketOrderDesc:
		if b.For != "" {
			errs = append(errs, prefix+": buckets.order desc cannot take for (the window runs forward from today)")
		}
	default:
		errs = append(errs, fmt.Sprintf("%s: buckets.order %q is not valid (valid: %s, %s)",
			prefix, b.Order, BucketOrderAsc, BucketOrderDesc))
	}
	return errs
}

// BucketLabelKeys returns the section keys `labels:` may rename for b, in
// section order. The relative scheme has its fixed buckets. The calendar
// form names only its fixed sections; the per-day, per-week and per-month
// sections take titles from the calendar.
func BucketLabelKeys(b ListBuckets) []string {
	if b.Scheme == BucketsRelativeWeeks {
		return WeekBucketKeys
	}
	if !b.Calendar() {
		return DateBucketKeys
	}
	var keys []string
	if b.For != "" {
		keys = append(keys, "overdue")
	}
	if b.By == BucketUnitDay {
		keys = append(keys, "today")
		// A one-day window holds only today, so tomorrow has no section.
		if days, _ := b.ForDays(); days != 1 {
			keys = append(keys, "tomorrow")
		}
	}
	if b.For != "" && b.Then == "" {
		keys = append(keys, "later")
	}
	return append(keys, "no_date")
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
		if grp.Color != "" {
			if _, err := ParseNamedColor(grp.Color); err != nil {
				errs = append(errs, fmt.Sprintf("%s: groups[%d]: %v", prefix, i, err))
			}
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
