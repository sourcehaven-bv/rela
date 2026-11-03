package dataentry

import (
	"cmp"
	"maps"
	"slices"
	"strconv"
	"strings"

	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	entityPkg "github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// The folds below compute a dashboard card's value from rows that are already
// row-gated and redacted (TKT-2SGPJZ). They never read the store: every input
// is in the rows, so the cost of a card is the cost of reading its list once.
//
// A property a row hides from this principal is in the row's Redacted list and
// missing from its Properties. A fold leaves such a row out of every group,
// cell or color that would need the value, and reports the card as partial.
// The row still counts towards totals: the list itself already shows it.

// axisProperty describes one property a fold groups on.
type axisProperty struct {
	name   string
	title  string            // the axis title: the property's label, or its name
	values []string          // declared enum values, in order; nil for an integer
	labels map[string]string // display labels by value
	isEnum bool
	// min and max are an integer's declared range. Nil: accepted — that
	// end of a matrix axis follows the rows.
	min, max *int
	// desc reverses a matrix axis.
	desc bool
}

func newAxisProperty(meta *metamodel.Metamodel, def *metamodel.EntityDef, name string) axisProperty {
	pd := def.Properties[name]
	values := dataentryconfig.GetValidEnumValues(pd, meta)
	labels := pd.Labels
	if ct, ok := meta.Types[pd.Type]; ok && len(labels) == 0 {
		labels = ct.Labels
	}
	title := pd.Label
	if title == "" {
		title = name
	}
	p := axisProperty{name: name, title: title, values: values, labels: labels, isEnum: len(values) > 0}
	if !p.isEnum {
		p.min, p.max = pd.Min, pd.Max
	}
	return p
}

// newMatrixAxis is newAxisProperty with a matrix card's title override and
// order for the axis.
func newMatrixAxis(
	meta *metamodel.Metamodel, def *metamodel.EntityDef, name, title, order string,
) axisProperty {
	p := newAxisProperty(meta, def, name)
	if title != "" {
		p.title = title
	}
	p.desc = order == dataentryconfig.MatrixOrderDesc
	return p
}

// axisSlot reads where a present value goes on a matrix axis: its key on
// the axis, or ok false when it has no place there. An enum axis lists only
// declared values, so a hand-edited value outside the enum has none. An
// integer axis takes whole numbers within the declared range; its key is
// the number written plainly, so "03" and 3 are one row.
func (p axisProperty) axisSlot(key string) (slot string, n int64, ok bool) {
	if p.isEnum {
		return key, 0, slices.Contains(p.values, key)
	}
	n, err := strconv.ParseInt(key, 10, 64)
	if err != nil {
		return "", 0, false
	}
	if (p.min != nil && n < int64(*p.min)) || (p.max != nil && n > int64(*p.max)) {
		return "", 0, false
	}
	return strconv.FormatInt(n, 10), n, true
}

func (p axisProperty) label(value string) string {
	if l, ok := p.labels[value]; ok && l != "" {
		return l
	}
	return value
}

// valueOf reads the property from a row: its key, whether the row has a
// value, and whether the value is hidden from this principal.
func (p axisProperty) valueOf(e *entityPkg.Entity) (key string, present, hidden bool) {
	if e.IsRedacted(p.name) {
		return "", false, true
	}
	key, present = axisKey(e.Properties[p.name])
	return key, present, false
}

// axisKey renders a scalar property value as a group key. A number that is a
// whole value renders without a fraction, so 3 read from YAML as an int and
// 3.0 read from JSON are one group.
func axisKey(v any) (string, bool) {
	switch x := v.(type) {
	case nil:
		return "", false
	case string:
		return x, x != ""
	case int:
		return strconv.Itoa(x), true
	case int64:
		return strconv.FormatInt(x, 10), true
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64), true
	case bool:
		return strconv.FormatBool(x), true
	default:
		return "", false
	}
}

// groupCounts counts rows per value of p, in display order: declared order
// for an enum, else by count (most first) then value. A row without a value
// is counted under the empty value, listed last.
func groupCounts(
	rows []*entityPkg.Entity, p axisProperty, matched map[string]bool,
) (groups []v1.DashboardGroup, partial bool) {
	counts := map[string]*v1.DashboardGroup{}
	for _, e := range rows {
		key, _, hidden := p.valueOf(e)
		if hidden {
			partial = true
			continue
		}
		g, ok := counts[key]
		if !ok {
			g = &v1.DashboardGroup{Value: key, Label: p.label(key)}
			counts[key] = g
		}
		g.Count++
		if matched[e.ID] {
			g.Matched++
		}
	}
	for _, g := range counts {
		groups = append(groups, *g)
	}
	rank := enumRank(p.values)
	slices.SortFunc(groups, func(a, b v1.DashboardGroup) int {
		if (a.Value == "") != (b.Value == "") {
			if a.Value == "" {
				return 1
			}
			return -1
		}
		ra, aok := rank[a.Value]
		rb, bok := rank[b.Value]
		if aok && bok {
			return cmp.Compare(ra, rb)
		}
		if aok != bok {
			if aok {
				return -1
			}
			return 1
		}
		return cmp.Or(cmp.Compare(b.Count, a.Count), cmp.Compare(a.Value, b.Value))
	})
	return groups, partial
}

func enumRank(values []string) map[string]int {
	rank := make(map[string]int, len(values))
	for i, v := range values {
		rank[v] = i
	}
	return rank
}

// matrixInput is everything foldMatrix needs besides the rows.
type matrixInput struct {
	rows, columns axisProperty
	toneBy        *axisProperty
	// colorFor maps a tone_by value to the style class its cells take; empty
	// for a value with no color.
	colorFor func(value string) string
}

// foldMatrix counts rows per (row value, column value) cell.
//
// A cell's color is that of the most severe tone_by value among its rows,
// where severity is the declared enum order and the last value is the most
// severe. A row whose tone_by value is hidden counts but does not color.
//
// A row without a value on an axis, or with one the axis has no place for
// (see [axisProperty.axisSlot]), is not placed and counts in Unplaced.
func foldMatrix(rows []*entityPkg.Entity, in matrixInput) (m v1.DashboardMatrix, partial bool) {
	counts := map[matrixCellKey]int{}
	worst := map[matrixCellKey]int{}
	toneRank := map[string]int{}
	if in.toneBy != nil {
		toneRank = enumRank(in.toneBy.values)
	}
	var rowSpan, colSpan observedSpan
	for _, e := range rows {
		r, rok, rhidden := in.rows.valueOf(e)
		c, cok, chidden := in.columns.valueOf(e)
		if rhidden || chidden {
			// A hidden value never widens an axis either: the extent of an
			// integer axis would otherwise disclose it.
			partial = true
			continue
		}
		if !rok || !cok {
			m.Unplaced++
			continue
		}
		r, rn, rplaced := in.rows.axisSlot(r)
		c, cn, cplaced := in.columns.axisSlot(c)
		if !rplaced || !cplaced {
			m.Unplaced++
			continue
		}
		rowSpan.add(rn)
		colSpan.add(cn)
		k := matrixCellKey{r, c}
		counts[k]++
		rank, toned, hidden := in.toneRankOf(e, toneRank)
		partial = partial || hidden
		if prev, seen := worst[k]; toned && (!seen || rank > prev) {
			worst[k] = rank
		}
	}
	var truncR, truncC bool
	m.RowProperty, m.ColumnProperty = in.rows.name, in.columns.name
	m.RowTitle, m.ColumnTitle = in.rows.title, in.columns.title
	m.Rows, truncR = axisValues(in.rows, rowSpan)
	m.Columns, truncC = axisValues(in.columns, colSpan)
	m.Truncated = truncR || truncC
	m.Cells = in.cells(m.Rows, m.Columns, counts, worst)
	return m, partial
}

type matrixCellKey struct{ row, col string }

// toneRankOf reads a row's tone_by severity: its rank, whether it has a known
// one, and whether the value is hidden from this principal.
func (in matrixInput) toneRankOf(e *entityPkg.Entity, toneRank map[string]int) (rank int, toned, hidden bool) {
	if in.toneBy == nil {
		return 0, false, false
	}
	t, ok, hidden := in.toneBy.valueOf(e)
	if hidden || !ok {
		return 0, false, hidden
	}
	rank, toned = toneRank[t]
	return rank, toned, false
}

// cells lists the non-empty cells in display order, each colored by its
// most severe tone.
func (in matrixInput) cells(
	rows, columns []v1.DashboardAxisValue, counts, worst map[matrixCellKey]int,
) []v1.DashboardCell {
	var out []v1.DashboardCell
	for _, rv := range rows {
		for _, cv := range columns {
			k := matrixCellKey{rv.Value, cv.Value}
			n := counts[k]
			if n == 0 {
				continue
			}
			cell := v1.DashboardCell{Row: rv.Value, Column: cv.Value, Count: n}
			if rank, ok := worst[k]; ok && in.colorFor != nil {
				cell.Color = in.colorFor(in.toneBy.values[rank])
			}
			out = append(out, cell)
		}
	}
	return out
}

// observedSpan is the placed values of an integer axis: the lowest, the
// highest, and each distinct value. It holds at most one entry per row of
// the page the card folds.
type observedSpan struct {
	lo, hi   int64
	any      bool
	distinct map[int64]struct{}
}

func (s *observedSpan) add(n int64) {
	if s.distinct == nil {
		s.distinct = map[int64]struct{}{}
	}
	s.distinct[n] = struct{}{}
	if !s.any {
		s.lo, s.hi, s.any = n, n, true
		return
	}
	s.lo, s.hi = min(s.lo, n), max(s.hi, n)
}

// axisValues lists an axis in display order. An enum axis is every declared
// value. An integer axis is every whole number from its low end to its high
// end, so a value nobody uses still has its row and the grid has no holes.
// Each end is the declared bound, else the observed extreme; with neither
// the axis is the one declared bound, or empty.
//
// Config load refuses a declared range wider than
// [dataentryconfig.MaxMatrixAxisValues]. When an end follows the rows and
// the span is wider than that, filling it would let one outlier push every
// real value off the axis, so the axis lists only the values that occur.
// Those are capped at the cap from the end shown first, and truncated says
// so.
func axisValues(p axisProperty, seen observedSpan) (out []v1.DashboardAxisValue, truncated bool) {
	var keys []int64
	switch lo, hi, ok := integerAxisEnds(p, seen); {
	case p.isEnum:
		for _, v := range p.values {
			out = append(out, v1.DashboardAxisValue{Value: v, Label: p.label(v)})
		}
	case !ok:
	case !dataentryconfig.MatrixAxisTooWide(lo, hi):
		for n := lo; ; n++ {
			keys = append(keys, n)
			if n == hi {
				break
			}
		}
	default:
		keys = slices.Sorted(maps.Keys(seen.distinct))
	}
	if p.desc {
		slices.Reverse(out)
		slices.Reverse(keys)
	}
	if len(keys) > dataentryconfig.MaxMatrixAxisValues {
		keys, truncated = keys[:dataentryconfig.MaxMatrixAxisValues], true
	}
	for _, n := range keys {
		k := strconv.FormatInt(n, 10)
		out = append(out, v1.DashboardAxisValue{Value: k, Label: p.label(k)})
	}
	return out, truncated
}

// integerAxisEnds picks an integer axis's low and high end: the declared
// bound where there is one, else the observed extreme. Observed values all
// lie within the declared bounds (axisSlot), so lo <= hi.
func integerAxisEnds(p axisProperty, seen observedSpan) (lo, hi int64, ok bool) {
	switch {
	case p.min != nil:
		lo = int64(*p.min)
	case seen.any:
		lo = seen.lo
	case p.max != nil:
		lo = int64(*p.max)
	default:
		return 0, 0, false
	}
	switch {
	case p.max != nil:
		hi = int64(*p.max)
	case seen.any:
		hi = seen.hi
	default:
		hi = lo
	}
	return lo, hi, true
}

// matrixColorFor resolves a tone_by value's color: the card's `colors:`
// entry (a tone or any palette color), else the app's `styles:` entry for the
// property's type, else none. Config load has validated every `colors:` value,
// so a parse failure here means no color rather than an error.
func matrixColorFor(
	card dataentryconfig.DashboardCard, cfg *dataentryconfig.Config, styles map[string]string,
) func(string) string {
	return func(value string) string {
		if name, ok := card.Colors[value]; ok {
			if c, isTone := dataentryconfig.ToneColor(name); isTone {
				return c.StyleClass()
			}
			c, err := dataentryconfig.ParseColor(name, cfg.OperatorColors())
			if err != nil {
				return ""
			}
			return c.StyleClass()
		}
		if cls, ok := styles[value]; ok {
			return cls
		}
		// The SPA looks a style up under the value lowercased with spaces
		// as underscores (styleKey in styleColors.ts); match that.
		return styles[strings.ReplaceAll(strings.ToLower(value), " ", "_")]
	}
}
