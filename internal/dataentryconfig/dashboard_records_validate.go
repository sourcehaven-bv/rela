package dataentryconfig

import (
	"fmt"
	"slices"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// DisplayLine is the display of a trend chart over records (TKT-RK810V7).
const DisplayLine = "line"

// MaxRecordsCardOwners caps the lines one chart draws. More lines than this
// cannot be told apart by color, so the card is split instead.
const MaxRecordsCardOwners = 8

// The values of a records card's `bucket:` and `stat:`. The buckets are the
// ones /api/v1/_records/{type}/_aggregate takes.
var (
	recordsCardBuckets = []string{"hour", "day", "week", "month"}
	recordsCardStats   = []string{"latest", "avg", "sum", "min", "max", "count"}
)

// validateRecordsCard checks a `display: line` card: it reads records, never
// a list or a query, and its type, owners, field, bucket, range and stat
// must make an aggregate request the API accepts.
func validateRecordsCard(meta *metamodel.Metamodel, at string, card DashboardCard) []string {
	if card.Display != DisplayLine {
		return []string{fmt.Sprintf("%s: records needs display: %s", at, DisplayLine)}
	}
	r := card.Records
	if r == nil {
		return []string{fmt.Sprintf("%s uses %s display, which needs records:", at, DisplayLine)}
	}
	var errs []string
	if card.List != "" || card.Query != "" {
		errs = append(errs, at+": a records card takes no list: or query:")
	}
	if card.GroupBy != "" || card.Property != "" || len(card.Columns) > 0 || len(card.Sort) > 0 ||
		card.Limit != 0 || card.Match != "" || card.setsMatrixOption() {
		errs = append(errs, at+": a records card takes only title, display, width, permission and records")
	}
	if card.Scope != nil {
		errs = append(errs, at+": a records card cannot be scoped to an entity dashboard's anchor")
	}
	switch n := len(r.Owners); {
	case n == 0:
		errs = append(errs, at+": records.owners needs at least one entity id")
	case n > MaxRecordsCardOwners:
		errs = append(errs, fmt.Sprintf("%s: records.owners has %d ids; a chart shows at most %d lines",
			at, n, MaxRecordsCardOwners))
	}
	for _, o := range r.Owners {
		if strings.TrimSpace(o) == "" {
			errs = append(errs, at+": records.owners has an empty id")
			break
		}
	}
	if r.Bucket != "" && !slices.Contains(recordsCardBuckets, r.Bucket) {
		errs = append(errs, fmt.Sprintf("%s: records.bucket %q is not one of %s",
			at, r.Bucket, strings.Join(recordsCardBuckets, ", ")))
	}
	if r.Stat != "" && !slices.Contains(recordsCardStats, r.Stat) {
		errs = append(errs, fmt.Sprintf("%s: records.stat %q is not one of %s",
			at, r.Stat, strings.Join(recordsCardStats, ", ")))
	}
	if r.Range != "" {
		if d, err := metamodel.ParseRetention(r.Range); err != nil || d <= 0 {
			errs = append(errs, fmt.Sprintf("%s: records.range %q is not a duration such as 48h, 90d, 2w or 5y",
				at, r.Range))
		}
	}
	return append(errs, validateRecordsCardField(meta, at, r)...)
}

// validateRecordsCardField checks the record type and the field it folds.
func validateRecordsCardField(meta *metamodel.Metamodel, at string, r *DashboardRecords) []string {
	if r.Type == "" {
		return []string{at + ": records.type is required"}
	}
	if r.Field == "" && r.Stat != "count" {
		return []string{at + ": records.field is required unless records.stat is count"}
	}
	if meta == nil {
		return nil
	}
	def, ok := meta.Records[r.Type]
	if !ok {
		return []string{fmt.Sprintf("%s: records.type %q is not a record type in the schema", at, r.Type)}
	}
	if r.Field == "" {
		return nil
	}
	p, ok := def.Fields[r.Field]
	if !ok || !metamodel.IsNumericType(p.Type) {
		return []string{fmt.Sprintf("%s: records.field %q is not an integer or decimal field of %q",
			at, r.Field, r.Type)}
	}
	return nil
}
