package dataentryconfig

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
)

// ListGroupBy splits a list into sections by the value of one property.
//
// Written either as a bare property name (`group_by: status`) or as a
// mapping. The short form is the long form with only Property set, so the
// wire always carries the mapping and the SPA reads one shape.
//
// A grouped list is not paged. The SPA loads every row up to MaxRows,
// sorted by the group property first, and splits them into sections on the
// client. Paging a grouped list would cut a section in half at a page
// boundary, and a reader cannot tell a half section from a whole one.
//
// Groups and Buckets are mutually exclusive, because they answer the same
// question (which section does a row go in) in two ways:
//   - Groups names sections for the values of an enum property. It only
//     restyles: every declared value still gets its section, in the enum's
//     declared order, whether or not Groups mentions it.
//   - Buckets sorts a date or datetime property into date sections: relative
//     ones (overdue, today, tomorrow, and so on) or calendar ones (one per
//     day, week, month or year). The SPA computes them in the reader's
//     display time zone, because "today" is the reader's today.
//
// With neither Property nor Relation, the list is grouped by condition
// (computed groups, TKT-QS024L0): each entry in Groups has an ID and a
// Condition, and the server puts each row in the first group whose condition
// it matches. One group without a condition may come last; it takes every
// row the others did not. See ListGroup.Condition.
type ListGroupBy struct {
	Property string `yaml:"property,omitempty" json:"property,omitempty"`
	// Relation groups on a single-valued relation instead of a property:
	// one section per target (TKT-JO8PN3). OfferedBy and OrderBy pick and
	// order the sections exactly as on a kanban's `columns_from`.
	Relation  string `yaml:"relation,omitempty" json:"relation,omitempty"`
	OfferedBy string `yaml:"offered_by,omitempty" json:"offered_by,omitempty"`
	OrderBy   string `yaml:"order_by,omitempty" json:"order_by,omitempty"`
	// StyleFrom colors each relation section by an enum property of its
	// target, as on a kanban's `columns_from`.
	StyleFrom string      `yaml:"style_from,omitempty" json:"style_from,omitempty"`
	Groups    []ListGroup `yaml:"groups,omitempty" json:"groups,omitempty"`
	// Buckets sorts a date property into date sections. See [ListBuckets].
	//
	// Nil: accepted, means the property's values are the sections.
	Buckets *ListBuckets `yaml:"buckets,omitempty" json:"buckets,omitempty"`
	// Labels overrides bucket titles by bucket key (see DateBucketKeys). A
	// key left out keeps the SPA's English default.
	Labels map[string]string `yaml:"labels,omitempty" json:"labels,omitempty"`
	// MaxRows caps how many rows the grouped list loads. Zero means the
	// default, filled in by NormalizeListGroupBy.
	MaxRows int `yaml:"max_rows,omitempty" json:"max_rows,omitempty"`
}

// ListGroup restyles the section for one value of an enum group property,
// or, in a computed grouping, declares one section by ID and Condition.
// Color is a color name from [ColorNames].
type ListGroup struct {
	Value string `yaml:"value,omitempty" json:"value,omitempty"`
	// ID names a computed group. It is what the server stamps on each row
	// (`_group`), so it is an allowlisted slug.
	ID string `yaml:"id,omitempty" json:"id,omitempty"`
	// Condition is a predicate over the row, compiled at load by
	// conditionlint (kind "list_groups") and evaluated on the served page
	// after row-gating and redaction. today() and now() are UTC, as in
	// every other condition.
	Condition string `yaml:"condition,omitempty" json:"condition,omitempty"`
	Label     string `yaml:"label,omitempty" json:"label,omitempty"`
	Color     string `yaml:"color,omitempty" json:"color,omitempty"`
}

// Computed reports whether the grouping is by condition rather than by a
// property or relation value.
func (g *ListGroupBy) Computed() bool {
	return g != nil && g.Property == "" && g.Relation == ""
}

// ListGroupConditionID is the compiled-condition key of a computed group.
// A group id never contains "#", so the list id is the part before the last
// one.
func ListGroupConditionID(listID, groupID string) string {
	return listID + "#" + groupID
}

// ListGroupConditionList returns the list id of a key built by
// [ListGroupConditionID].
func ListGroupConditionList(key string) string {
	if i := strings.LastIndex(key, "#"); i >= 0 {
		return key[:i]
	}
	return key
}

// listGroupByKeys are the keys the mapping form accepts. The rest of this
// file's config decodes leniently, but a typo here (`bucket: relative`)
// would silently produce a list grouped by distinct dates, which looks like
// a feature working badly rather than a key being ignored.
var listGroupByKeys = map[string]bool{
	"property": true, "groups": true, "buckets": true, "labels": true, "max_rows": true,
	"relation": true, "offered_by": true, "order_by": true, "style_from": true,
}

// UnmarshalYAML accepts the short form (a property name) and the mapping.
func (g *ListGroupBy) UnmarshalYAML(value *yaml.Node) error {
	switch value.Kind {
	case yaml.ScalarNode:
		*g = ListGroupBy{Property: strings.TrimSpace(value.Value)}
		return nil
	case yaml.MappingNode:
		for i := 0; i+1 < len(value.Content); i += 2 {
			key := value.Content[i].Value
			if !listGroupByKeys[key] {
				return fmt.Errorf("invalid group_by at line %d: unknown key %q (valid: %s)",
					value.Content[i].Line, key, strings.Join(slices.Sorted(maps.Keys(listGroupByKeys)), ", "))
			}
		}
		// Alias to avoid recursing into this method.
		type rawListGroupBy ListGroupBy
		var raw rawListGroupBy
		if err := value.Decode(&raw); err != nil {
			return fmt.Errorf("invalid group_by: %w", err)
		}
		*g = ListGroupBy(raw)
		return nil
	default:
		return fmt.Errorf("invalid group_by at line %d: got %s, must be a property name or a mapping",
			value.Line, yamlKindName(value.Kind))
	}
}
