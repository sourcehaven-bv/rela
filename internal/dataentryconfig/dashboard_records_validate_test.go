package dataentryconfig

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// Trend charts over records (TKT-RK810V7).

func recordsCardMetamodel() *metamodel.Metamodel {
	meta := testMetamodel()
	meta.Records = map[string]metamodel.RecordTypeDef{
		"measurement": {Owner: "ticket", Fields: map[string]metamodel.PropertyDef{
			"value": {Type: metamodel.PropertyTypeDecimal},
			"note":  {Type: metamodel.PropertyTypeString},
		}},
	}
	return meta
}

func validateRecordsCardYAML(t *testing.T, card string) (*Config, error) {
	t.Helper()
	src := `
version: "1.0"
lists:
  all: {entity_type: ticket}
dashboard:
  cards:
    - ` + card + `
`
	var cfg Config
	require.NoError(t, yaml.Unmarshal([]byte(src), &cfg))
	return &cfg, ValidateConfig([]byte(src), &cfg, recordsCardMetamodel())
}

func TestDashboardCard_RecordsLoads(t *testing.T) {
	cfg, err := validateRecordsCardYAML(t, `{title: Coverage, display: line, width: wide,
       records: {type: measurement, field: value, owners: [MET-1, MET-2], bucket: week, range: 365d, stat: avg}}`)
	require.NoError(t, err)
	r := cfg.Dashboard.Cards[0].Records
	require.NotNil(t, r)
	assert.Equal(t, DashboardRecords{Type: "measurement", Field: "value", Owners: []string{"MET-1", "MET-2"},
		Bucket: "week", Range: "365d", Stat: "avg"}, *r)

	_, err = validateRecordsCardYAML(t, `{title: Rows, display: line,
       records: {type: measurement, owners: [MET-1], stat: count}}`)
	require.NoError(t, err, "a count needs no field")
}

func TestValidateRecordsCard(t *testing.T) {
	const ok = `type: measurement, field: value, owners: [MET-1]`
	tests := []struct{ name, card, want string }{
		{"line without records", `{title: X, display: line}`, "uses line display, which needs records:"},
		{"records without line", `{title: X, display: count, records: {` + ok + `}}`, "records needs display: line"},
		{"records with list", `{title: X, list: all, display: line, records: {` + ok + `}}`, "takes no list: or query:"},
		{"records with query", `{title: X, query: "type:ticket", display: line, records: {` + ok + `}}`,
			"takes no list: or query:"},
		{"records with group_by", `{title: X, display: line, group_by: status, records: {` + ok + `}}`,
			"takes only title, display, width, permission and records"},
		{"no type", `{title: X, display: line, records: {field: value, owners: [MET-1]}}`, "records.type is required"},
		{"unknown type", `{title: X, display: line, records: {type: nope, field: value, owners: [MET-1]}}`,
			`records.type "nope" is not a record type`},
		{"unknown type with count", `{title: X, display: line, records: {type: nope, stat: count, owners: [MET-1]}}`,
			`records.type "nope" is not a record type`},
		{"no field", `{title: X, display: line, records: {type: measurement, owners: [MET-1]}}`,
			"records.field is required unless records.stat is count"},
		{"string field", `{title: X, display: line, records: {type: measurement, field: note, owners: [MET-1]}}`,
			`records.field "note" is not an integer or decimal field`},
		{"no owners", `{title: X, display: line, records: {type: measurement, field: value}}`,
			"records.owners needs at least one entity id"},
		{"too many owners", `{title: X, display: line, records: {type: measurement, field: value,
			owners: [A, B, C, D, E, F, G, H, I]}}`, "a chart shows at most 8 lines"},
		{"empty owner", `{title: X, display: line, records: {type: measurement, field: value, owners: [""]}}`,
			"records.owners has an empty id"},
		{"bad bucket", `{title: X, display: line, records: {` + ok + `, bucket: quarter}}`,
			`records.bucket "quarter" is not one of hour, day, week, month`},
		{"bad stat", `{title: X, display: line, records: {` + ok + `, stat: median}}`,
			`records.stat "median" is not one of`},
		{"bad range", `{title: X, display: line, records: {` + ok + `, range: forever}}`,
			`records.range "forever" is not a duration`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := validateRecordsCardYAML(t, tc.card)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.want)
		})
	}
}

// An entity dashboard counts for its anchor, which a records card cannot
// be narrowed to, so it is refused there.
func TestValidateRecordsCard_NotOnEntityDashboard(t *testing.T) {
	src := `
version: "1.0"
dashboards:
  one:
    entity_type: category
    cards:
      - {title: X, display: line, records: {type: measurement, field: value, owners: [MET-1]}}
`
	var cfg Config
	require.NoError(t, yaml.Unmarshal([]byte(src), &cfg))
	err := ValidateConfig([]byte(src), &cfg, recordsCardMetamodel())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "a card of an entity dashboard needs list:")
}
