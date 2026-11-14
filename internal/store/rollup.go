package store

import (
	"context"
	"fmt"
	"iter"
	"maps"
	"math"
	"math/big"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// RollupFunc is the aggregate a [RollupQuery] computes.
type RollupFunc int

const (
	// RollupCount counts the related rows. [RollupResult.Value] stays nil.
	RollupCount RollupFunc = iota
	// RollupMin and RollupMax take the smallest and largest value of
	// [RollupQuery.Value]: numerically under [RollupQuery.Numeric], else
	// byte-wise on non-empty strings (the [GraphQuery.OrderBy] order). That
	// is time order only for YYYY-MM-DD dates, so the roll-up compiler
	// refuses datetimes and other date formats.
	RollupMin
	RollupMax
	// RollupSum adds the integral numbers of [RollupQuery.Value], or its
	// decimals under [RollupQuery.Decimal].
	RollupSum
	// RollupLatest reads [RollupQuery.Value] from the row with the largest
	// non-empty string [RollupQuery.By]; on a tie, the smallest id wins.
	RollupLatest
)

// RollupQuery asks, for each id in Sources, for an aggregate over the
// related rows: the rows Related selects that share a [RollupQuery.Relation]
// edge with the source (TKT-6I2OPG).
//
// Related is the caller's whole statement of which rows may count. A
// principal-facing caller passes the principal's read query for the related
// type, so the store aggregates only rows the principal may read: the gate
// runs before the fold, inside the backend, and no ungated row is ever
// loaded. Only identity-scoped edges (an empty tail) are followed.
//
// A roll-up over a path (TKT-LTSNHD) first walks [RollupQuery.Path] from
// the sources; Relation then joins the entities the last step reached to the
// related rows. Each step's Through gates the entities it reaches as Related
// gates the rows, so an entity on the path that Through does not select
// breaks every path through it. A related row reached from one source by
// several paths counts once.
//
// A value that is not of the expected shape is skipped, never coerced:
// a number is an integral JSON number (Go integer kinds, or a float64 with no
// fraction), a string is a non-empty string. A list never counts as either.
type RollupQuery struct {
	// Related selects the rows to aggregate. Faces must be a world
	// selection, so each related entity is one row; OrderBy, Limit and
	// Offset must be unset.
	Related GraphQuery
	// Path are the steps from a source to the entities Relation starts
	// from. Empty means Relation starts from the sources.
	Path []RollupStep
	// Relation is the canonical relation type joining a source, or the
	// entity the last step reached, to a row.
	Relation string
	// SourceIsTo says the edge runs from the related row to the source.
	// False means it runs from the source to the related row.
	SourceIsTo bool
	// Sources are the ids to answer for. An empty slice answers nothing.
	Sources []string

	Func RollupFunc
	// Value is the property aggregated (min, max, sum) or read (latest).
	Value string
	// By orders the rows for RollupLatest.
	By string
	// Numeric compares Value as a number for RollupMin and RollupMax.
	Numeric bool
	// Decimal sums Value as exact decimals for RollupSum: a number, or a
	// string in plain decimal notation (see [RollupDecimal]). The sum is the
	// decimal text with as many digits after the point as the longest
	// fraction summed, such as "14.00" for "12.50" and "1.5".
	Decimal bool

	// Require lists conditions every counted row must meet. A row meets a
	// condition when any of its queries matches it.
	Require []RowCondition
	// PickRequire is checked on the row RollupLatest picks. When it does
	// not hold, the result has no Value, but the row still wins: an older
	// row is never read instead.
	PickRequire []RowCondition
}

// RollupStep is one step of a [RollupQuery.Path]: the identity-scoped edges
// of Relation from the entities reached so far, to the entities Through
// selects.
type RollupStep struct {
	// Relation is the canonical relation type walked.
	Relation string
	// SourceIsTo says the edge runs from the entity this step reaches to
	// the entity before it.
	SourceIsTo bool
	// Through selects the entities the step may reach, with the rules of
	// [RollupQuery.Related]: a world selection, no paging or sorting.
	Through GraphQuery
}

// RowCondition holds for a row when any query in Any matches it. A nil query
// matches every row; an empty Any matches none. Each query is evaluated
// against the row itself at every face of its endpoints, as a field grant's
// `when:` is (search.FieldGrant): EntityType is the related type or empty,
// FaceIn restricts the row's own face, and Faces, OrderBy, Limit and Offset
// are ignored.
type RowCondition struct {
	Any []*GraphQuery
}

// RollupResult is one source's answer. Count is the number of related rows;
// Value is the aggregate, nil when no row supplied one. A Value is an int64
// for a number, a string, or a bool, whichever the stored JSON held. A
// decimal sum is its decimal text.
type RollupResult struct {
	Count int
	Value any
}

// Rollupper is the native form of [Rollup]: one statement per query.
// OPTIONAL, type-asserted with [RollupGeneric] as the fallback.
type Rollupper interface {
	Rollup(ctx context.Context, q RollupQuery) (map[string]RollupResult, error)
}

// RollupReader is what [RollupGeneric] reads through.
type RollupReader interface {
	GraphQueryer
	ListRelations(ctx context.Context, q RelationQuery) iter.Seq2[*entity.Relation, error]
}

// Rollup answers q on st, through the backend's [Rollupper] when it has one.
// Sources without a related row are absent from the map.
func Rollup(ctx context.Context, st Store, q RollupQuery) (map[string]RollupResult, error) {
	if r, ok := st.(Rollupper); ok {
		return r.Rollup(ctx, q)
	}
	return RollupGeneric(ctx, st, q)
}

// ValidateRollupQuery refuses a query every backend would answer
// differently or not at all.
func ValidateRollupQuery(q RollupQuery) error {
	if q.Relation == "" {
		return fmt.Errorf("%w: rollup: relation is required", ErrInvalidQuery)
	}
	if err := validateRollupGate("related", q.Related); err != nil {
		return err
	}
	for i, step := range q.Path {
		if step.Relation == "" {
			return fmt.Errorf("%w: rollup: path step %d: relation is required", ErrInvalidQuery, i)
		}
		if err := validateRollupGate(fmt.Sprintf("path step %d", i), step.Through); err != nil {
			return err
		}
	}
	switch q.Func {
	case RollupCount:
	case RollupMin, RollupMax, RollupSum:
		if q.Value == "" {
			return fmt.Errorf("%w: rollup: value property is required", ErrInvalidQuery)
		}
	case RollupLatest:
		if q.Value == "" || q.By == "" {
			return fmt.Errorf("%w: rollup: latest needs a value and a by property", ErrInvalidQuery)
		}
	default:
		return fmt.Errorf("%w: rollup: unknown function %d", ErrInvalidQuery, q.Func)
	}
	return nil
}

// validateRollupGate refuses a related or step query that does not select
// one row per entity.
func validateRollupGate(what string, q GraphQuery) error {
	switch {
	case q.EntityType == "":
		return fmt.Errorf("%w: rollup: %s entity type is required", ErrInvalidQuery, what)
	case q.Limit != 0 || q.Offset != 0 || len(q.OrderBy) > 0:
		return fmt.Errorf("%w: rollup: the %s query must not page or sort", ErrInvalidQuery, what)
	}
	if _, ok := q.Faces.World(); !ok {
		return fmt.Errorf("%w: rollup: the %s query must select a world", ErrInvalidQuery, what)
	}
	return nil
}

// RollupGeneric answers q with ordinary reads and folds in Go: per path
// step and for the last relation one relation read and one header read, and
// one MatchingFaces per condition query. It is the reference the SQL
// backends are tested against.
func RollupGeneric(ctx context.Context, st RollupReader, q RollupQuery) (map[string]RollupResult, error) {
	if err := ValidateRollupQuery(q); err != nil {
		return nil, err
	}
	out := map[string]RollupResult{}
	if len(q.Sources) == 0 {
		return out, nil
	}
	// reached maps each entity reached so far to the sources it is
	// reached from.
	reached := map[string][]string{}
	for _, src := range q.Sources {
		reached[src] = []string{src}
	}
	for _, step := range q.Path {
		rows, linked, err := rollupStep(ctx, st, reached, step.Relation, step.SourceIsTo, step.Through)
		if err != nil || len(rows) == 0 {
			return out, err
		}
		reached = make(map[string][]string, len(rows))
		for _, h := range rows {
			reached[h.ID] = linked[h.ID]
		}
	}
	rows, linked, err := rollupStep(ctx, st, reached, q.Relation, q.SourceIsTo, q.Related)
	if err != nil || len(rows) == 0 {
		return out, err
	}
	ids := make([]string, len(rows))
	for i, h := range rows {
		ids[i] = h.ID
	}
	required, err := HoldingRows(ctx, st, q.Related.EntityType, q.Require, ids)
	if err != nil {
		return nil, err
	}
	pickable, err := HoldingRows(ctx, st, q.Related.EntityType, q.PickRequire, ids)
	if err != nil {
		return nil, err
	}

	folds := map[string]*rollupFold{}
	for _, h := range rows {
		if !required(h) {
			continue
		}
		for _, src := range linked[h.ID] {
			f := folds[src]
			if f == nil {
				f = &rollupFold{q: q}
				folds[src] = f
			}
			f.add(h, pickable(h))
		}
	}
	for src, f := range folds {
		out[src] = f.result()
	}
	return out, nil
}

// rollupStep follows the identity-scoped edges of relation from the
// entities in reached to the rows gate selects. It returns those rows and,
// per row id, the sources it is reached from, each once.
func rollupStep(
	ctx context.Context, st RollupReader, reached map[string][]string,
	relation string, sourceIsTo bool, gate GraphQuery,
) (rows []EntityHeader, linked map[string][]string, err error) {
	near := slices.Sorted(maps.Keys(reached))
	identity := entity.Face("")
	rq := RelationQuery{Type: relation, EntityIDs: near, FromFace: &identity, Direction: DirectionOutgoing}
	if sourceIsTo {
		rq.Direction = DirectionIncoming
	}
	linked = map[string][]string{} // far id -> source ids
	for r, lerr := range st.ListRelations(ctx, rq) {
		if lerr != nil {
			return nil, nil, lerr
		}
		from, far := r.From, r.To
		if sourceIsTo {
			from, far = r.To, r.From
		}
		for _, src := range reached[from] {
			if !slices.Contains(linked[far], src) {
				linked[far] = append(linked[far], src)
			}
		}
	}
	if len(linked) == 0 {
		return nil, linked, nil
	}

	gate.Related = append(slices.Clone(gate.Related), DirectedRelation{
		Incoming: !sourceIsTo,
		Pred:     RelationPredicate{OfTypes: []string{relation}, Endpoints: near},
	})
	for h, err := range GraphQueryHeaders(ctx, st, gate) {
		if err != nil {
			return nil, nil, err
		}
		if _, ok := linked[h.ID]; ok {
			rows = append(rows, h)
		}
	}
	return rows, linked, nil
}

// HoldingRows returns a test for "every condition holds on this row",
// answering each condition query once for all ids.
func HoldingRows(
	ctx context.Context, st GraphQueryer, entityType string, conds []RowCondition, ids []string,
) (func(EntityHeader) bool, error) {
	type faceSet map[string][]entity.Face
	answers := make([][]faceSet, len(conds)) // nil entry: matches every row
	for i, c := range conds {
		answers[i] = make([]faceSet, len(c.Any))
		for j, when := range c.Any {
			if when == nil {
				continue
			}
			wq := *when
			wq.EntityType = entityType
			wq.Faces = AllFaces()
			wq.OrderBy, wq.Limit, wq.Offset = nil, 0, 0
			got, err := st.MatchingFaces(ctx, wq, ids)
			if err != nil {
				return nil, err
			}
			answers[i][j] = got
		}
	}
	return func(h EntityHeader) bool {
		for i, c := range conds {
			held := false
			for j, when := range c.Any {
				if when == nil || slices.Contains(answers[i][j][h.ID], h.Face) {
					held = true
					break
				}
			}
			if !held {
				return false
			}
		}
		return true
	}, nil
}

// rollupFold accumulates one source's rows.
type rollupFold struct {
	q     RollupQuery
	count int
	// num and str hold the running min or max.
	num    int64
	hasNum bool
	str    string
	// latest: the winning row's order key and id, and its value.
	by   string
	byID string
	pick any
	// sum is exact, so a total is judged once, at the end.
	sum *big.Int
	// dec and decScale hold a decimal sum and its longest fraction.
	dec      *big.Rat
	decScale int
}

func (f *rollupFold) add(h EntityHeader, pickable bool) {
	f.count++
	switch f.q.Func {
	case RollupMin, RollupMax:
		f.addExtreme(h.Properties[f.q.Value])
	case RollupSum:
		if f.q.Decimal {
			f.addDecimal(h.Properties[f.q.Value])
			return
		}
		n, ok := RollupNumber(h.Properties[f.q.Value])
		if !ok {
			return
		}
		if f.sum == nil {
			f.sum = new(big.Int)
		}
		f.sum.Add(f.sum, big.NewInt(n))
	case RollupLatest:
		by, ok := RollupText(h.Properties[f.q.By])
		if !ok {
			return
		}
		if f.byID == "" || by > f.by || (by == f.by && h.ID < f.byID) {
			f.by, f.byID = by, h.ID
			f.pick = nil
			if pickable {
				f.pick = RollupScalar(h.Properties[f.q.Value])
			}
		}
	case RollupCount:
	}
}

// addDecimal folds one value into the running decimal sum.
func (f *rollupFold) addDecimal(raw any) {
	r, scale, ok := RollupDecimal(raw)
	if !ok {
		return
	}
	if f.dec == nil {
		f.dec = new(big.Rat)
	}
	f.dec.Add(f.dec, r)
	f.decScale = max(f.decScale, scale)
}

// addExtreme folds one value into the running min or max.
func (f *rollupFold) addExtreme(raw any) {
	if f.q.Numeric {
		n, ok := RollupNumber(raw)
		if !ok {
			return
		}
		if !f.hasNum || (f.q.Func == RollupMin && n < f.num) || (f.q.Func == RollupMax && n > f.num) {
			f.num, f.hasNum = n, true
		}
		return
	}
	s, ok := RollupText(raw)
	if !ok {
		return
	}
	if f.str == "" || (f.q.Func == RollupMin && s < f.str) || (f.q.Func == RollupMax && s > f.str) {
		f.str = s
	}
}

func (f *rollupFold) result() RollupResult {
	res := RollupResult{Count: f.count}
	switch f.q.Func {
	case RollupMin, RollupMax:
		if f.q.Numeric && f.hasNum {
			res.Value = f.num
		} else if !f.q.Numeric && f.str != "" {
			res.Value = f.str
		}
	case RollupSum:
		if f.q.Decimal {
			if f.dec != nil {
				res.Value = f.dec.FloatString(f.decScale)
			}
			break
		}
		// A total outside int64 has no value: every backend sums exactly
		// and refuses the same totals.
		if f.sum != nil && f.sum.IsInt64() {
			res.Value = f.sum.Int64()
		}
	case RollupLatest:
		res.Value = f.pick
	case RollupCount:
	}
	return res
}

// RollupNumber reads v as an integral number, the only numbers a roll-up
// aggregates.
func RollupNumber(v any) (int64, bool) {
	switch n := v.(type) {
	case int:
		return int64(n), true
	case int8:
		return int64(n), true
	case int16:
		return int64(n), true
	case int32:
		return int64(n), true
	case int64:
		return n, true
	case uint8:
		return int64(n), true
	case uint16:
		return int64(n), true
	case uint32:
		return int64(n), true
	case uint64:
		if n > math.MaxInt64 {
			return 0, false
		}
		return int64(n), true
	case uint:
		if uint64(n) > math.MaxInt64 {
			return 0, false
		}
		return int64(n), true
	case float64:
		if n != math.Trunc(n) || n < math.MinInt64 || n >= math.MaxInt64 {
			return 0, false
		}
		return int64(n), true
	case float32:
		return RollupNumber(float64(n))
	}
	return 0, false
}

// RollupDecimal reads v as an exact decimal for a decimal sum, with the
// number of digits after its point. A string counts only in plain decimal
// notation: an optional sign, digits, and optionally a point and digits. A
// float is read through its shortest decimal text, as the decimal property
// type reads one. Anything else, a list included, is no decimal.
func RollupDecimal(v any) (*big.Rat, int, bool) {
	var text string
	switch x := v.(type) {
	case string:
		if !decimalNotation(x) {
			return nil, 0, false
		}
		text = x
	case float64:
		if math.IsInf(x, 0) || math.IsNaN(x) {
			return nil, 0, false
		}
		text = strconv.FormatFloat(x, 'f', -1, 64)
	case float32:
		return RollupDecimal(float64(x))
	default:
		n, ok := RollupNumber(v)
		if !ok {
			return nil, 0, false
		}
		text = strconv.FormatInt(n, 10)
	}
	r, ok := new(big.Rat).SetString(text)
	if !ok {
		return nil, 0, false
	}
	scale := 0
	if _, frac, found := strings.Cut(text, "."); found {
		scale = len(frac)
	}
	return r, scale, true
}

// decimalNotation reports whether s is `[+-]?digits(.digits)?`.
func decimalNotation(s string) bool {
	if s != "" && (s[0] == '+' || s[0] == '-') {
		s = s[1:]
	}
	whole, frac, hasPoint := strings.Cut(s, ".")
	return allDigits(whole) && (!hasPoint || allDigits(frac))
}

func allDigits(s string) bool {
	if s == "" {
		return false
	}
	for i := range len(s) {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// RollupText is the text a min, max or latest_by orders v by, or !ok when v
// has none. A time value is its JSON text (RFC 3339), which is what a
// database backend stores and compares, so every backend orders it alike:
// the fs store parses an unquoted YAML date into a time value.
func RollupText(v any) (string, bool) {
	switch s := v.(type) {
	case string:
		return s, s != ""
	case time.Time:
		return s.Format(time.RFC3339Nano), true
	}
	return "", false
}

// RollupScalar normalizes a picked value to the shapes [RollupResult.Value]
// promises: a string, a bool or an int64. Anything else (a fraction, a list,
// an object, an empty string) is no value.
func RollupScalar(v any) any {
	switch s := v.(type) {
	case string:
		if s == "" {
			return nil
		}
		return s
	case bool:
		return s
	case time.Time:
		return s.Format(time.RFC3339Nano)
	}
	if n, ok := RollupNumber(v); ok {
		return n
	}
	return nil
}
