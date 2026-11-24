package dataentryconfig

import (
	"encoding/json"
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// ListBuckets is a list's `group_by.buckets:`. It has two forms:
//
//   - The scalar `relative` sets Scheme: sections by how far away a date is
//     (overdue, today, tomorrow, next 7 days, later).
//   - A mapping sets By, For, Then and Order: calendar sections, one per
//     day, week, month or year. `{by: day, for: 7d, then: month}` gives the
//     next seven days one by one, then one section per month.
//
// The wire carries the form the YAML used, so the SPA reads the same keys the
// operator wrote. Exactly one form is set; validateListBuckets enforces it.
type ListBuckets struct {
	// Scheme is the scalar form. Only [BucketsRelative] is valid.
	Scheme string
	// By is the calendar unit of each section: one of BucketUnits.
	By string
	// For is the near window of a `by: day` list, written `<n>d`. Each day
	// from today to today + n - 1 gets a section; earlier dates go in
	// overdue, later ones in Then sections, or in later without Then.
	For string
	// Then is the unit of the sections after the For window. It must be
	// coarser than day.
	Then string
	// Order is asc (the default) or desc, for newest first.
	Order string
}

// BucketsRelative is the scalar bucketing scheme.
const BucketsRelative = "relative"

// Calendar bucket units, finest first.
const (
	BucketUnitDay   = "day"
	BucketUnitWeek  = "week"
	BucketUnitMonth = "month"
	BucketUnitYear  = "year"
)

// BucketUnits are the valid By and Then units, finest first.
var BucketUnits = []string{BucketUnitDay, BucketUnitWeek, BucketUnitMonth, BucketUnitYear}

// Bucket orders.
const (
	BucketOrderAsc  = "asc"
	BucketOrderDesc = "desc"
)

// maxBucketForDays bounds For: a year of day sections is already more than a
// reader scans.
const maxBucketForDays = 366

// forPattern is the form of [ListBuckets.For].
var forPattern = regexp.MustCompile(`^\d+d$`)

// listBucketsKeys are the keys the mapping form accepts. A typo must fail,
// for the reason listGroupByKeys gives.
var listBucketsKeys = map[string]bool{"by": true, "for": true, "then": true, "order": true}

// listBucketsMapping is the mapping form as it is decoded and encoded.
type listBucketsMapping struct {
	By    string `yaml:"by,omitempty" json:"by,omitempty"`
	For   string `yaml:"for,omitempty" json:"for,omitempty"`
	Then  string `yaml:"then,omitempty" json:"then,omitempty"`
	Order string `yaml:"order,omitempty" json:"order,omitempty"`
}

// UnmarshalYAML accepts the scalar scheme and the calendar mapping.
func (b *ListBuckets) UnmarshalYAML(value *yaml.Node) error {
	switch value.Kind {
	case yaml.ScalarNode:
		*b = ListBuckets{Scheme: scalarScheme(value.Value)}
		return nil
	case yaml.MappingNode:
		for i := 0; i+1 < len(value.Content); i += 2 {
			if key := value.Content[i].Value; !listBucketsKeys[key] {
				return fmt.Errorf("invalid buckets at line %d: unknown key %q (valid: %s)",
					value.Content[i].Line, key, strings.Join(slices.Sorted(maps.Keys(listBucketsKeys)), ", "))
			}
		}
		var m listBucketsMapping
		if err := value.Decode(&m); err != nil {
			return fmt.Errorf("invalid buckets: %w", err)
		}
		*b = ListBuckets{By: m.By, For: m.For, Then: m.Then, Order: m.Order}
		return nil
	default:
		return fmt.Errorf("invalid buckets at line %d: got %s, must be %q or a mapping",
			value.Line, yamlKindName(value.Kind), BucketsRelative)
	}
}

// MarshalYAML writes the form the config used.
func (b ListBuckets) MarshalYAML() (any, error) {
	if b.Scheme != "" {
		return b.Scheme, nil
	}
	return b.mapping(), nil
}

// MarshalJSON writes the form the config used: a string or a mapping.
func (b ListBuckets) MarshalJSON() ([]byte, error) {
	if b.Scheme != "" {
		return json.Marshal(b.Scheme)
	}
	return json.Marshal(b.mapping())
}

// UnmarshalJSON reads either form, so a config round-trips through JSON.
func (b *ListBuckets) UnmarshalJSON(data []byte) error {
	var scheme string
	if err := json.Unmarshal(data, &scheme); err == nil {
		*b = ListBuckets{Scheme: scalarScheme(scheme)}
		return nil
	}
	var m listBucketsMapping
	if err := json.Unmarshal(data, &m); err != nil {
		return fmt.Errorf("invalid buckets: %w", err)
	}
	*b = ListBuckets{By: m.By, For: m.For, Then: m.Then, Order: m.Order}
	return nil
}

// emptyScheme stands for a scalar written empty, so it reads as an invalid
// scheme rather than as a mapping without by.
const emptyScheme = `""`

func scalarScheme(v string) string {
	if v = strings.TrimSpace(v); v != "" {
		return v
	}
	return emptyScheme
}

func (b ListBuckets) mapping() listBucketsMapping {
	return listBucketsMapping{By: b.By, For: b.For, Then: b.Then, Order: b.Order}
}

// Calendar reports whether b is the mapping form.
func (b *ListBuckets) Calendar() bool {
	return b != nil && b.Scheme == ""
}

// Descending reports whether the sections run newest first.
func (b *ListBuckets) Descending() bool {
	return b.Calendar() && b.Order == BucketOrderDesc
}

// ForDays returns the length of the For window in days, or 0 when For is
// unset. ok is false for a value that is not `<n>d` with n in range.
func (b ListBuckets) ForDays() (days int, ok bool) {
	if b.For == "" {
		return 0, true
	}
	// The same form the SPA parses (/^\d+d$/): no sign, no space.
	if !forPattern.MatchString(b.For) {
		return 0, false
	}
	n, err := strconv.Atoi(strings.TrimSuffix(b.For, "d"))
	if err != nil || n < 1 || n > maxBucketForDays {
		return 0, false
	}
	return n, true
}
