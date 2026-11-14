package dataentry

import (
	"context"
	"maps"
	"math/big"
	"slices"
	"time"

	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	entityPkg "github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/filter"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// minAvgScale is the fewest digits after the point an average shows, so
// the average of 1 and 2 reads 1.50 rather than 2.
const minAvgScale = 2

// cardRows redacts raw once and adds the roll-ups among props to each row,
// for this reader: a card reads a roll-up wherever it reads a property. The
// rows are copies, so neither raw nor a redacted row is written through.
func cardRows(
	ctx context.Context, a *App, typeName string, raw []*entityPkg.Entity, props ...string,
) []*entityPkg.Entity {
	rows := redactRows(ctx, a, raw)
	s := newRollupSource(a)
	if s.schema == nil || s.schema() == nil {
		return rows
	}
	names := s.schema().Meta.RollupNames(typeName)
	want := map[string]bool{}
	for _, p := range props {
		if slices.Contains(names, p) {
			want[p] = true
		}
	}
	if len(want) == 0 || len(rows) == 0 {
		return rows
	}
	out := make([]*entityPkg.Entity, len(rows))
	batch := make([]rollupRow, len(rows))
	for i, e := range rows {
		cp := *e
		cp.Properties = maps.Clone(e.Properties)
		if cp.Properties == nil {
			cp.Properties = map[string]any{}
		}
		out[i] = &cp
		batch[i] = rollupRow{ref: e.Ref(), props: cp.Properties}
	}
	s.add(ctx, typeName, batch, worldScopeFrom(ctx), func(p string) bool { return want[p] })
	return out
}

// summaryDisplays are the card displays that fold one property over the
// card's rows into one value.
var summaryDisplays = map[string]dataentryconfig.SummaryFunction{
	"sum": dataentryconfig.SummarySum,
	"avg": dataentryconfig.SummaryAvg,
	"min": dataentryconfig.SummaryMin,
	"max": dataentryconfig.SummaryMax,
}

// foldSummary computes a sum, avg, min or max card over rows, which are
// redacted and carry their roll-ups. A row whose property is hidden from
// the reader is left out and makes the card partial; a row without a value
// is left out. Value stays nil when no row supplied one.
//
// The rows are gated and redacted before the fold, so the value is what
// this reader could compute from what they see. It is per principal.
func foldSummary(
	fn dataentryconfig.SummaryFunction, meta *metamodel.Metamodel, def *metamodel.EntityDef, prop string,
	rows []*entityPkg.Entity,
) (value any, partial bool) {
	pd := def.Properties[prop]
	var kept []*entityPkg.Entity
	for _, e := range rows {
		if e.IsRedacted(prop) {
			partial = true
			continue
		}
		if v, ok := e.Properties[prop]; ok && v != nil && v != "" {
			kept = append(kept, e)
		}
	}
	if len(kept) == 0 {
		return nil, partial
	}
	switch fn {
	case dataentryconfig.SummarySum, dataentryconfig.SummaryAvg:
		return numericFold(fn, pd, prop, kept), partial
	case dataentryconfig.SummaryMin, dataentryconfig.SummaryMax:
		return extremeFold(fn, meta, pd, prop, kept), partial
	case dataentryconfig.SummaryCount:
	}
	return nil, partial
}

// numericFold sums or averages prop exactly. An integer sum is a number; a
// decimal sum, and every average, is decimal text with the longest
// fraction summed (an average at least minAvgScale).
func numericFold(
	fn dataentryconfig.SummaryFunction, pd metamodel.PropertyDef, prop string, rows []*entityPkg.Entity,
) any {
	sum := new(big.Rat)
	scale, n := 0, 0
	for _, e := range rows {
		r, sc, err := metamodel.ParseDecimal(e.Properties[prop])
		if err != nil {
			continue
		}
		sum.Add(sum, r)
		scale = max(scale, sc)
		n++
	}
	if n == 0 {
		return nil
	}
	if fn == dataentryconfig.SummaryAvg {
		return new(big.Rat).Quo(sum, new(big.Rat).SetInt64(int64(n))).FloatString(max(scale, minAvgScale))
	}
	if pd.Type == metamodel.PropertyTypeInteger && sum.IsInt() && sum.Num().IsInt64() {
		return sum.Num().Int64()
	}
	return sum.FloatString(scale)
}

// extremeFold picks the smallest or largest value of prop in the order a
// list sorts it by, so a card and a sorted list agree.
func extremeFold(
	fn dataentryconfig.SummaryFunction, meta *metamodel.Metamodel, pd metamodel.PropertyDef, prop string,
	rows []*entityPkg.Entity,
) any {
	specs := []filter.SortSpec{{Property: prop, Direction: "asc"}}
	sorted := slices.Clone(rows)
	filter.QuerySortApply(newEntityQuerySort(specs, sorted, meta), sorted, entityRecord, specs)
	pick := sorted[0]
	if fn == dataentryconfig.SummaryMax {
		pick = sorted[len(sorted)-1]
	}
	v := pick.Properties[prop]
	if t, ok := v.(time.Time); ok {
		// The fs store parses an unquoted YAML date into a time value; serve
		// it in the property's own form.
		return t.Format(pd.GetDateFormat())
	}
	return v
}

// dashboardSummary fills a sum, avg, min or max card.
func dashboardSummary(
	ctx context.Context, a *App, s *Schema, card dataentryconfig.DashboardCard, def *metamodel.EntityDef,
	typeName string, raw []*entityPkg.Entity, out v1.DashboardCardValue,
) v1.DashboardCardValue {
	rows := cardRows(ctx, a, typeName, raw, card.Property)
	out.Property = card.Property
	out.Value, out.Partial = foldSummary(summaryDisplays[card.Display], s.Meta, def, card.Property, rows)
	return out
}
