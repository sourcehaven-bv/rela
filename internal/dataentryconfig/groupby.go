package dataentryconfig

import (
	"fmt"
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
//   - Buckets sorts a date or datetime property into relative sections
//     (overdue, today, tomorrow, and so on). The SPA computes them in the
//     reader's display time zone, because "today" is the reader's today.
type ListGroupBy struct {
	Property string      `yaml:"property" json:"property"`
	Groups   []ListGroup `yaml:"groups,omitempty" json:"groups,omitempty"`
	// Buckets names the bucketing scheme. Only "relative" exists today.
	Buckets string `yaml:"buckets,omitempty" json:"buckets,omitempty"`
	// Labels overrides bucket titles by bucket key (see DateBucketKeys). A
	// key left out keeps the SPA's English default.
	Labels map[string]string `yaml:"labels,omitempty" json:"labels,omitempty"`
	// MaxRows caps how many rows the grouped list loads. Zero means the
	// default, filled in by NormalizeListGroupBy.
	MaxRows int `yaml:"max_rows,omitempty" json:"max_rows,omitempty"`
}

// ListGroup restyles the section for one value of an enum group property.
// Color is a status color token from the component library (see
// ValidGroupColors).
type ListGroup struct {
	Value string `yaml:"value" json:"value"`
	Label string `yaml:"label,omitempty" json:"label,omitempty"`
	Color string `yaml:"color,omitempty" json:"color,omitempty"`
}

// listGroupByKeys are the keys the mapping form accepts. The rest of this
// file's config decodes leniently, but a typo here (`bucket: relative`)
// would silently produce a list grouped by distinct dates, which looks like
// a feature working badly rather than a key being ignored.
var listGroupByKeys = map[string]bool{
	"property": true, "groups": true, "buckets": true, "labels": true, "max_rows": true,
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
					value.Content[i].Line, key, strings.Join(sortedMapKeys(listGroupByKeys), ", "))
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
