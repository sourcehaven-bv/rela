package dataentry

import (
	"cmp"
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

// maxMatrixAxisValues caps an integer axis, which takes its values from the
// rows rather than from a declared enum.
const maxMatrixAxisValues = 50

// axisProperty describes one property a fold groups on.
type axisProperty struct {
	name   string
	values []string          // declared enum values, in order; nil for an integer
	labels map[string]string // display labels by value
	isEnum bool
}

// onAxis reports whether a present value has a place on a matrix axis. An
// enum axis lists only declared values, so a hand-edited value outside the
// enum has none.
func (p axisProperty) onAxis(key string) bool {
	return !p.isEnum || slices.Contains(p.values, key)
}

func newAxisProperty(meta *metamodel.Metamodel, def *metamodel.EntityDef, name string) axisProperty {
	pd := def.Properties[name]
	values := dataentryconfig.GetValidEnumValues(pd, meta)
	labels := pd.Labels
	if ct, ok := meta.Types[pd.Type]; ok && len(labels) == 0 {
		labels = ct.Labels
	}
	return axisProperty{name: name, values: values, labels: labels, isEnum: len(values) > 0}
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
func foldMatrix(rows []*entityPkg.Entity, in matrixInput) (m v1.DashboardMatrix, partial bool) {
	type cellKey struct{ row, col string }
	counts := map[cellKey]int{}
	worst := map[cellKey]int{}
	toneRank := map[string]int{}
	if in.toneBy != nil {
		toneRank = enumRank(in.toneBy.values)
	}
	present := map[string]map[string]bool{in.rows.name: {}, in.columns.name: {}}
	for _, e := range rows {
		r, rok, rhidden := in.rows.valueOf(e)
		c, cok, chidden := in.columns.valueOf(e)
		if rhidden || chidden {
			partial = true
			continue
		}
		if !rok || !cok || !in.rows.onAxis(r) || !in.columns.onAxis(c) {
			m.Unplaced++
			continue
		}
		k := cellKey{r, c}
		counts[k]++
		present[in.rows.name][r] = true
		present[in.columns.name][c] = true
		if in.toneBy == nil {
			continue
		}
		t, tok, thidden := in.toneBy.valueOf(e)
		if thidden {
			partial = true
			continue
		}
		if rank, known := toneRank[t]; tok && known {
			if prev, seen := worst[k]; !seen || rank > prev {
				worst[k] = rank
			}
		}
	}
	var truncR, truncC bool
	m.RowProperty, m.ColumnProperty = in.rows.name, in.columns.name
	m.Rows, truncR = axisValues(in.rows, present[in.rows.name])
	m.Columns, truncC = axisValues(in.columns, present[in.columns.name])
	m.Truncated = truncR || truncC
	for _, rv := range m.Rows {
		for _, cv := range m.Columns {
			k := cellKey{rv.Value, cv.Value}
			n := counts[k]
			if n == 0 {
				continue
			}
			cell := v1.DashboardCell{Row: rv.Value, Column: cv.Value, Count: n}
			if rank, ok := worst[k]; ok && in.colorFor != nil {
				cell.Color = in.colorFor(in.toneBy.values[rank])
			}
			m.Cells = append(m.Cells, cell)
		}
	}
	return m, partial
}

// axisValues lists an axis: every declared value of an enum, or the present
// values of an integer ascending, capped at maxMatrixAxisValues.
func axisValues(p axisProperty, present map[string]bool) (out []v1.DashboardAxisValue, truncated bool) {
	if p.isEnum {
		for _, v := range p.values {
			out = append(out, v1.DashboardAxisValue{Value: v, Label: p.label(v)})
		}
		return out, false
	}
	keys := make([]string, 0, len(present))
	for k := range present {
		keys = append(keys, k)
	}
	// Numbers ascending, then any hand-edited non-numeric value by text, so
	// the order is total.
	slices.SortFunc(keys, func(a, b string) int {
		fa, errA := strconv.ParseFloat(a, 64)
		fb, errB := strconv.ParseFloat(b, 64)
		switch {
		case errA == nil && errB == nil:
			return cmp.Or(cmp.Compare(fa, fb), cmp.Compare(a, b))
		case errA == nil:
			return -1
		case errB == nil:
			return 1
		}
		return cmp.Compare(a, b)
	})
	if len(keys) > maxMatrixAxisValues {
		keys, truncated = keys[:maxMatrixAxisValues], true
	}
	for _, k := range keys {
		out = append(out, v1.DashboardAxisValue{Value: k, Label: k})
	}
	return out, truncated
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
