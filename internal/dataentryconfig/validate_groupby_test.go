package dataentryconfig

import (
	"os"
	"regexp"
	"sort"
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
			groupBy: &ListGroupBy{Property: "due", Buckets: "relative", Labels: map[string]string{"overdue": "Te laat", "today": "Vandaag"}},
		},
		{name: "datetime buckets", groupBy: &ListGroupBy{Property: "at", Buckets: "relative"}},
		{name: "missing property", groupBy: &ListGroupBy{}, wantErr: "property or relation is required"},
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
			groupBy: &ListGroupBy{Property: "status", Groups: []ListGroup{{Value: "todo", Color: "violet"}}},
			wantErr: `unknown color "violet"`,
		},
		{
			name:    "groups on a non-enum property",
			groupBy: &ListGroupBy{Property: "title", Groups: []ListGroup{{Value: "x"}}},
			wantErr: "groups needs an enum property",
		},
		{name: "max_rows negative", groupBy: &ListGroupBy{Property: "status", MaxRows: -1}, wantErr: "max_rows -1 is out of range"},
		{name: "max_rows above ceiling", groupBy: &ListGroupBy{Property: "status", MaxRows: 2001}, wantErr: "max_rows 2001 is out of range"},
		{name: "buckets on an enum", groupBy: &ListGroupBy{Property: "status", Buckets: "relative"}, wantErr: "buckets needs a date or datetime property"},
		{name: "unknown bucket scheme", groupBy: &ListGroupBy{Property: "due", Buckets: "weekly"}, wantErr: `buckets "weekly" is not valid`},
		{
			name:    "unknown label key",
			groupBy: &ListGroupBy{Property: "due", Buckets: "relative", Labels: map[string]string{"yesterday": "Gisteren"}},
			wantErr: `unknown bucket "yesterday"`,
		},
		{
			name:    "labels without buckets",
			groupBy: &ListGroupBy{Property: "due", Labels: map[string]string{"today": "Vandaag"}},
			wantErr: "labels only apply with buckets",
		},
		{
			name:    "groups and buckets together",
			groupBy: &ListGroupBy{Property: "due", Buckets: "relative", Groups: []ListGroup{{Value: "x"}}},
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
			want: ListGroupBy{Property: "due", Buckets: "relative", Labels: map[string]string{"today": "Vandaag"}},
		},
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

// TestGroupColorsMatchLibrary pins ValidGroupColors to the component
// library's StatusColor type. The SPA passes the configured color straight to
// the section heading, so a token the library does not know renders as no
// color at all, silently.
func TestGroupColorsMatchLibrary(t *testing.T) {
	src, err := os.ReadFile("../../frontend/packages/rela-components/src/types/index.ts")
	require.NoError(t, err)
	line := regexp.MustCompile(`export type StatusColor = ([^\n]+)`).FindSubmatch(src)
	require.NotNil(t, line, "StatusColor type not found")
	var library []string
	for _, m := range regexp.MustCompile(`'([a-z]+)'`).FindAllSubmatch(line[1], -1) {
		library = append(library, string(m[1]))
	}
	sort.Strings(library)
	assert.Equal(t, sortedMapKeys(ValidGroupColors), library)
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
