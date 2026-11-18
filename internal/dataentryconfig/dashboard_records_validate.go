package dataentryconfig

import (
	"fmt"
	"reflect"
	"regexp"
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

// recordsCardRange is the grammar the SPA reads a card's `range:` with:
// digits and one unit. It is stricter than [metamodel.ParseRetention], which
// also takes a sign, so a range the chart cannot read fails at load.
var recordsCardRange = regexp.MustCompile(`^\d+[hdwy]$`)

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
	if !onlyRecordsCardFields(card) {
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
	seen := make(map[string]bool, len(r.Owners))
	for _, o := range r.Owners {
		switch {
		case strings.TrimSpace(o) == "":
			errs = append(errs, at+": records.owners has an empty id")
		case seen[o]:
			errs = append(errs, fmt.Sprintf("%s: records.owners names %q twice", at, o))
		}
		seen[o] = true
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
		d, err := metamodel.ParseRetention(r.Range)
		if err != nil || d <= 0 || !recordsCardRange.MatchString(strings.TrimSpace(r.Range)) {
			errs = append(errs, fmt.Sprintf("%s: records.range %q is not a duration such as 48h, 90d, 2w or 5y",
				at, r.Range))
		}
	}
	return append(errs, validateRecordsCardField(meta, at, r)...)
}

// onlyRecordsCardFields reports whether the card sets no field besides the
// ones a records card takes. It lists what is allowed rather than what is
// not, so a field added to [DashboardCard] later is refused here until a
// records card learns to use it. List, query and scope have their own errors.
func onlyRecordsCardFields(card DashboardCard) bool {
	rest := card
	rest.Title, rest.Display, rest.Width, rest.Permission, rest.Records = "", "", "", "", nil
	rest.List, rest.Query, rest.Scope = "", "", nil
	return reflect.ValueOf(rest).IsZero()
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
