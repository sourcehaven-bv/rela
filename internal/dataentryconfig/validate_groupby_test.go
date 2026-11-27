package dataentryconfig

import (
	"encoding/json"
	"maps"
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// groupByMetamodel has one property of each shape grouping cares about: an
// inline enum, a named enum type, a date, a datetime, a plain string and a
// list-valued enum.
func groupByMetamodel() *metamodel.Metamodel {
	return &metamodel.Metamodel{
		Version: "1.0",
		Types: map[string]metamodel.CustomType{
			"health": {Values: []string{"on_track", "at_risk", "late"}},
		},
		Entities: map[string]metamodel.EntityDef{
			"task": {
				Label:           "Task",
				IDPrefix:        "TASK-",
				DisplayProperty: "title",
				Properties: map[string]metamodel.PropertyDef{
					"title":  {Type: metamodel.PropertyTypeString},
					"status": {Type: metamodel.PropertyTypeEnum, Values: []string{"todo", "doing", "done"}},
					"health": {Type: "health"},
					"due":    {Type: metamodel.PropertyTypeDate},
					"at":     {Type: metamodel.PropertyTypeDatetime},
					"tags":   {Type: metamodel.PropertyTypeEnum, Values: []string{"a", "b"}, List: true},
				},
			},
		},
	}
}

func groupByErrors(g *ListGroupBy) []string {
	cfg := &Config{Lists: map[string]List{
		"tasks": {EntityType: "task", Columns: []ListColumn{{Property: "title"}}, GroupBy: g},
	}}
	return validateListsGroupBy(cfg, groupByMetamodel())
}

func TestValidateListGroupBy(t *testing.T) {
	tests := []struct {
		name    string
		groupBy *ListGroupBy
		wantErr string // empty: must validate clean
	}{
		{name: "absent", groupBy: nil},
		{name: "enum short form", groupBy: &ListGroupBy{Property: "status"}},
		{name: "plain string", groupBy: &ListGroupBy{Property: "title"}},
		{
			name: "enum with groups",
			groupBy: &ListGroupBy{Property: "status", MaxRows: 2000, Groups: []ListGroup{
				{Value: "doing", Label: "In progress", Color: "green"},
				{Value: "done", Color: "grey"},
			}},
		},
		{name: "named enum type", groupBy: &ListGroupBy{Property: "health", Groups: []ListGroup{{Value: "late", Color: "red"}}}},
		{
			name:    "date buckets with labels",
			groupBy: &ListGroupBy{Property: "due", Buckets: &ListBuckets{Scheme: "relative"}, Labels: map[string]string{"overdue": "Te laat", "today": "Vandaag"}},
		},
		{
			name:    "week buckets with labels",
			groupBy: &ListGroupBy{Property: "due", Buckets: &ListBuckets{Scheme: "relative_weeks"}, Labels: map[string]string{"this_week": "Deze week", "next_week": "Volgende week"}},
		},
		{
			name:    "week buckets reject next_7_days",
			groupBy: &ListGroupBy{Property: "due", Buckets: &ListBuckets{Scheme: "relative_weeks"}, Labels: map[string]string{"next_7_days": "x"}},
			wantErr: `labels has unknown bucket "next_7_days"`,
		},
		{name: "datetime buckets", groupBy: &ListGroupBy{Property: "at", Buckets: &ListBuckets{Scheme: "relative"}}},
		{name: "calendar months", groupBy: &ListGroupBy{Property: "due", Buckets: &ListBuckets{By: "month", Order: "desc"}}},
		{
			name: "near then far",
			groupBy: &ListGroupBy{Property: "due", Buckets: &ListBuckets{By: "day", For: "7d", Then: "month"},
				Labels: map[string]string{"overdue": "Te laat", "today": "Vandaag", "no_date": "Geen datum"}},
		},
		{name: "day window then later", groupBy: &ListGroupBy{Property: "at", Buckets: &ListBuckets{By: "day", For: "14d"}, Labels: map[string]string{"later": "Later"}}},
		{name: "calendar without by", groupBy: &ListGroupBy{Property: "due", Buckets: &ListBuckets{Order: "asc"}}, wantErr: "buckets.by is required"},
		{name: "unknown unit", groupBy: &ListGroupBy{Property: "due", Buckets: &ListBuckets{By: "quarter"}}, wantErr: `buckets.by "quarter" is not valid`},
		{name: "for without d", groupBy: &ListGroupBy{Property: "due", Buckets: &ListBuckets{By: "day", For: "7"}}, wantErr: `buckets.for "7" is not valid`},
		{name: "for with a sign", groupBy: &ListGroupBy{Property: "due", Buckets: &ListBuckets{By: "day", For: "+7d"}}, wantErr: `buckets.for "+7d" is not valid`},
		{name: "empty scheme", groupBy: &ListGroupBy{Property: "due", Buckets: &ListBuckets{Scheme: `""`}}, wantErr: `buckets "\"\"" is not valid`},
		{
			name:    "tomorrow outside a one-day window",
			groupBy: &ListGroupBy{Property: "due", Buckets: &ListBuckets{By: "day", For: "1d"}, Labels: map[string]string{"tomorrow": "Morgen"}},
			wantErr: `unknown bucket "tomorrow"`,
		},
		{name: "for zero", groupBy: &ListGroupBy{Property: "due", Buckets: &ListBuckets{By: "day", For: "0d"}}, wantErr: `buckets.for "0d" is not valid`},
		{name: "for too long", groupBy: &ListGroupBy{Property: "due", Buckets: &ListBuckets{By: "day", For: "367d"}}, wantErr: `buckets.for "367d" is not valid`},
		{name: "for on months", groupBy: &ListGroupBy{Property: "due", Buckets: &ListBuckets{By: "month", For: "7d"}}, wantErr: "buckets.for needs by: day"},
		{name: "then without for", groupBy: &ListGroupBy{Property: "due", Buckets: &ListBuckets{By: "day", Then: "month"}}, wantErr: "buckets.then needs for"},
		{name: "then day", groupBy: &ListGroupBy{Property: "due", Buckets: &ListBuckets{By: "day", For: "7d", Then: "day"}}, wantErr: `buckets.then "day" is not valid`},
		{name: "desc with for", groupBy: &ListGroupBy{Property: "due", Buckets: &ListBuckets{By: "day", For: "7d", Order: "desc"}}, wantErr: "order desc cannot take for"},
		{name: "unknown order", groupBy: &ListGroupBy{Property: "due", Buckets: &ListBuckets{By: "day", Order: "up"}}, wantErr: `buckets.order "up" is not valid`},
		{
			name:    "label the shape has no section for",
			groupBy: &ListGroupBy{Property: "due", Buckets: &ListBuckets{By: "month"}, Labels: map[string]string{"today": "Vandaag"}},
			wantErr: `unknown bucket "today" (valid: no_date)`,
		},
		{name: "calendar buckets on an enum", groupBy: &ListGroupBy{Property: "status", Buckets: &ListBuckets{By: "day"}}, wantErr: "buckets needs a date or datetime property"},
		{name: "missing property", groupBy: &ListGroupBy{}, wantErr: "property, relation or groups with conditions is required"},
		{
			name: "computed groups",
			groupBy: &ListGroupBy{Groups: []ListGroup{
				{ID: "doing", Condition: "entity.status == 'doing'", Label: "Doing", Color: "green"},
				{ID: "rest"},
			}},
		},
		{
			name:    "computed fallback not last",
			groupBy: &ListGroupBy{Groups: []ListGroup{{ID: "rest"}, {ID: "doing", Condition: "true"}}},
			wantErr: "only the last group may be the fallback",
		},
		{
			name:    "computed duplicate id",
			groupBy: &ListGroupBy{Groups: []ListGroup{{ID: "a", Condition: "true"}, {ID: "a", Condition: "true"}}},
			wantErr: "duplicates groups[0]",
		},
		{
			name:    "computed bad id",
			groupBy: &ListGroupBy{Groups: []ListGroup{{ID: "A#b", Condition: "true"}}},
			wantErr: "must match",
		},
		{
			name:    "computed group with value",
			groupBy: &ListGroupBy{Groups: []ListGroup{{ID: "a", Value: "doing", Condition: "true"}}},
			wantErr: "value needs property",
		},
		{
			name:    "computed with buckets",
			groupBy: &ListGroupBy{Buckets: &ListBuckets{Scheme: "relative"}, Groups: []ListGroup{{ID: "a", Condition: "true"}}},
			wantErr: "buckets needs a date property",
		},
		{
			name:    "condition on an enum group",
			groupBy: &ListGroupBy{Property: "status", Groups: []ListGroup{{Value: "doing", Condition: "true"}}},
			wantErr: "a computed grouping has no property or relation",
		},
		{name: "unknown property", groupBy: &ListGroupBy{Property: "nope"}, wantErr: `property "nope" not in metamodel`},
		{name: "list property", groupBy: &ListGroupBy{Property: "tags"}, wantErr: "is a list"},
		{
			name:    "group value outside the enum",
			groupBy: &ListGroupBy{Property: "status", Groups: []ListGroup{{Value: "blocked"}}},
			wantErr: `groups[0] value "blocked" is not valid`,
		},
		{
			name:    "duplicate group value",
			groupBy: &ListGroupBy{Property: "status", Groups: []ListGroup{{Value: "todo"}, {Value: "todo"}}},
			wantErr: "groups[1] duplicates groups[0]",
		},
		{
			name:    "unknown color",
			groupBy: &ListGroupBy{Property: "status", Groups: []ListGroup{{Value: "todo", Color: "pink"}}},
			wantErr: `unknown color "pink"`,
		},
		{
			name:    "groups on a non-enum property",
			groupBy: &ListGroupBy{Property: "title", Groups: []ListGroup{{Value: "x"}}},
			wantErr: "groups needs an enum property",
		},
		{name: "max_rows negative", groupBy: &ListGroupBy{Property: "status", MaxRows: -1}, wantErr: "max_rows -1 is out of range"},
		{name: "max_rows above ceiling", groupBy: &ListGroupBy{Property: "status", MaxRows: 2001}, wantErr: "max_rows 2001 is out of range"},
		{name: "buckets on an enum", groupBy: &ListGroupBy{Property: "status", Buckets: &ListBuckets{Scheme: "relative"}}, wantErr: "buckets needs a date or datetime property"},
		{name: "unknown bucket scheme", groupBy: &ListGroupBy{Property: "due", Buckets: &ListBuckets{Scheme: "weekly"}}, wantErr: `buckets "weekly" is not valid`},
		{
			name:    "unknown label key",
			groupBy: &ListGroupBy{Property: "due", Buckets: &ListBuckets{Scheme: "relative"}, Labels: map[string]string{"yesterday": "Gisteren"}},
			wantErr: `unknown bucket "yesterday"`,
		},
		{
			name:    "labels without buckets",
			groupBy: &ListGroupBy{Property: "due", Labels: map[string]string{"today": "Vandaag"}},
			wantErr: "labels only apply with buckets",
		},
		{
			name:    "groups and buckets together",
			groupBy: &ListGroupBy{Property: "due", Buckets: &ListBuckets{Scheme: "relative"}, Groups: []ListGroup{{Value: "x"}}},
			wantErr: "groups and buckets cannot both be set",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			errs := groupByErrors(tc.groupBy)
			if tc.wantErr == "" {
				assert.Empty(t, errs)
				return
			}
			require.NotEmpty(t, errs)
			assert.Contains(t, strings.Join(errs, "\n"), tc.wantErr)
		})
	}
}

func TestListGroupBy_UnmarshalYAML(t *testing.T) {
	tests := []struct {
		name    string
		src     string
		want    ListGroupBy
		wantErr string
	}{
		{name: "short form", src: "group_by: status", want: ListGroupBy{Property: "status"}},
		{
			name: "long form",
			src: "group_by:\n  property: status\n  max_rows: 100\n" +
				"  groups:\n    - {value: doing, label: In progress, color: green}\n",
			want: ListGroupBy{Property: "status", MaxRows: 100, Groups: []ListGroup{
				{Value: "doing", Label: "In progress", Color: "green"},
			}},
		},
		{
			name: "buckets",
			src:  "group_by: {property: due, buckets: relative, labels: {today: Vandaag}}",
			want: ListGroupBy{Property: "due", Buckets: &ListBuckets{Scheme: "relative"}, Labels: map[string]string{"today": "Vandaag"}},
		},
		{
			name: "calendar buckets",
			src:  "group_by: {property: due, buckets: {by: day, for: 7d, then: month}}",
			want: ListGroupBy{Property: "due", Buckets: &ListBuckets{By: "day", For: "7d", Then: "month"}},
		},
		{name: "empty buckets scalar", src: "group_by: {property: due, buckets: ''}", want: ListGroupBy{Property: "due", Buckets: &ListBuckets{Scheme: `""`}}},
		{name: "unknown buckets key", src: "group_by: {property: due, buckets: {by: day, until: 7d}}", wantErr: `unknown key "until"`},
		{name: "buckets sequence", src: "group_by: {property: due, buckets: [day]}", wantErr: "must be \"relative\" or a mapping"},
		// A typo in the mapping must not fall back to grouping by distinct
		// values, which would look like the feature misbehaving.
		{name: "unknown key", src: "group_by: {property: due, bucket: relative}", wantErr: `unknown key "bucket"`},
		{name: "sequence", src: "group_by: [status]", wantErr: "must be a property name or a mapping"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var list List
			err := yaml.Unmarshal([]byte(tc.src), &list)
			if tc.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErr)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, list.GroupBy)
			assert.Equal(t, tc.want, *list.GroupBy)
		})
	}
}

func TestNormalizeListGroupBy(t *testing.T) {
	cfg := &Config{Lists: map[string]List{
		"flat":    {EntityType: "task"},
		"default": {EntityType: "task", GroupBy: &ListGroupBy{Property: "status"}},
		"set":     {EntityType: "task", GroupBy: &ListGroupBy{Property: "status", MaxRows: 40}},
	}}
	NormalizeListGroupBy(cfg)
	assert.Nil(t, cfg.Lists["flat"].GroupBy)
	assert.Equal(t, defaultGroupMaxRows, cfg.Lists["default"].GroupBy.MaxRows)
	assert.Equal(t, 40, cfg.Lists["set"].GroupBy.MaxRows)
}

// TestColorNamesMatchSPA pins ColorNames to the SPA's NAMED_COLORS, which
// paints every surface that takes a bare color name. A name only one side
// knows either validates and renders as no color, or is never accepted.
func TestColorNamesMatchSPA(t *testing.T) {
	src, err := os.ReadFile("../../frontend/src/utils/styleColors.ts")
	require.NoError(t, err)
	block := regexp.MustCompile(`(?s)(?:export )?const NAMED_COLORS[^=]*= \{(.*?)\n\}`).FindSubmatch(src)
	require.NotNil(t, block, "NAMED_COLORS not found")
	var spa []string
	for _, m := range regexp.MustCompile(`(?m)^\s*([a-z]+):`).FindAllSubmatch(block[1], -1) {
		spa = append(spa, string(m[1]))
	}
	slices.Sort(spa)
	assert.Equal(t, slices.Sorted(maps.Keys(ColorNames)), spa)
}

// TestDateBucketKeysMatchSPA pins DateBucketKeys to the SPA's bucket order,
// which is where the keys are used. A key only one side knows is a label
// that validates and never shows, or a bucket that cannot be relabelled.
func TestDateBucketKeysMatchSPA(t *testing.T) {
	src, err := os.ReadFile("../../frontend/src/utils/dateBuckets.ts")
	require.NoError(t, err)
	block := regexp.MustCompile(`(?s)export const BUCKET_ORDER[^=]*= \[(.*?)\]`).FindSubmatch(src)
	require.NotNil(t, block, "BUCKET_ORDER not found")
	var spa []string
	for _, m := range regexp.MustCompile(`'([a-z_0-9]+)'`).FindAllSubmatch(block[1], -1) {
		spa = append(spa, string(m[1]))
	}
	assert.Equal(t, DateBucketKeys, spa)
}

// TestListBuckets_JSONKeepsTheConfigForm pins the wire to the YAML shape: a
// string for the scheme, the mapping's own keys for the calendar form.
func TestListBuckets_JSONKeepsTheConfigForm(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   ListBuckets
		want string
	}{
		{name: "scheme", in: ListBuckets{Scheme: "relative"}, want: `"relative"`},
		{name: "calendar", in: ListBuckets{By: "day", For: "7d", Then: "month"}, want: `{"by":"day","for":"7d","then":"month"}`},
		{name: "desc", in: ListBuckets{By: "month", Order: "desc"}, want: `{"by":"month","order":"desc"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := json.Marshal(tc.in)
			require.NoError(t, err)
			assert.JSONEq(t, tc.want, string(got))
			var back ListBuckets
			require.NoError(t, json.Unmarshal(got, &back))
			assert.Equal(t, tc.in, back)
			y, err := yaml.Marshal(tc.in)
			require.NoError(t, err)
			var fromYAML ListBuckets
			require.NoError(t, yaml.Unmarshal(y, &fromYAML))
			assert.Equal(t, tc.in, fromYAML)
		})
	}
}
