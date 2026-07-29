package lua

import (
	"fmt"

	lua "github.com/yuin/gopher-lua"
)

// maxReadLimit bounds how many rows ANY single Lua read binding returns
// (DEC-IYHLNF). It is a hard ceiling, not a default a caller can raise:
// `limit` may lower it, nothing removes it.
//
// 2000 is the upper end of the 500–2000 batch band that most database tooling
// settles on (Rails find_in_batches defaults to 1000, psycopg2 itersize to
// 2000, typical JDBC fetch sizes 500–1000). The cost curve is U-shaped with a
// wide flat bottom: below ~100 per-round-trip overhead dominates, above ~5000
// you hold a large result set while doing per-row work. Between those, the
// difference rarely shows up in a profile — so this is a defensible engineering
// default, not a derived constant. Tune it on evidence.
//
// Deliberately NOT parseV1Pagination's 25/100. Those bound an HTTP response for
// a browser; a Lua script runs IN-PROCESS next to the store, so there is no
// response to bound and smaller pages would only mean more database round
// trips. Do not "harmonize" the two without re-deriving why.
//
// A var, not a const, so tests can lower it without seeding thousands of rows
// (same rationale as dataentry's listExportCap).
var maxReadLimit = 2000

// initialRowCapacity pre-sizes a read binding's result slice. Capped well
// below maxReadLimit on purpose: `limit` is an upper BOUND, not a prediction,
// so allocating for it up front would reserve 2000 slots for a query that
// typically returns a handful. Growth past this is amortized doubling.
const initialRowCapacity = 64

// readOpts is the parsed form of the options table every read binding accepts:
// `f(required..., {limit = n, ...})`. Binding-specific keys are read by the
// binding itself; this carries only what they all share.
//
// Until store-side paging lands there is no `cursor` (DEC-IYHLNF stage 2). It
// is ABSENT rather than accepted-and-ignored on purpose: a cursor that silently
// never advances turns the idiomatic paging loop into an infinite one, and an
// offset-backed stand-in would skip and duplicate rows under concurrent writes.
// An unknown option raises, so a script written against the future API fails
// loudly here instead of silently returning page one forever.
type readOpts struct {
	limit int
}

// listEntitiesArgs reads rela.list_entities' second argument, which may be
// either the options table or a bare filter string.
//
// The bare string is kept because `list_entities("ticket", "status=open")`
// is the single most common call in every script in the tree, and a filter
// expression is genuinely the one thing worth a positional shorthand. It is
// exactly equivalent to `{filter = "status=open"}` and gets the same default
// bound — the shorthand buys brevity, never different semantics.
func listEntitiesArgs(s *lua.LState) (filterExpr string, o readOpts, err error) {
	if s.GetTop() >= 2 {
		if str, isStr := s.Get(2).(lua.LString); isStr {
			return string(str), readOpts{limit: maxReadLimit}, nil
		}
	}
	o, err = parseReadOpts(s, 2, "filter")
	if err != nil {
		return "", o, err
	}
	if s.GetTop() >= 2 {
		if tbl, isTbl := s.Get(2).(*lua.LTable); isTbl {
			switch v := tbl.RawGetString("filter").(type) {
			case *lua.LNilType:
			case lua.LString:
				filterExpr = string(v)
			default:
				return "", o, fmt.Errorf("option %q must be a string, got %s", "filter", v.Type())
			}
		}
	}
	return filterExpr, o, nil
}

// parseReadOpts reads the options table at stack position pos.
//
// An absent or nil argument yields the ceiling. A non-table raises — unlike
// rela.get_relations' legacy tolerance of a stray non-table argument, this is
// new API surface with no callers to protect, so it is strict from the start.
func parseReadOpts(s *lua.LState, pos int, known ...string) (readOpts, error) {
	o := readOpts{limit: maxReadLimit}
	if s.GetTop() < pos {
		return o, nil
	}
	v := s.Get(pos)
	if v == lua.LNil {
		return o, nil
	}
	tbl, ok := v.(*lua.LTable)
	if !ok {
		return o, fmt.Errorf("options must be a table, got %s", v.Type())
	}

	// Reject unknown keys. A typo'd option that is silently ignored is the
	// same defect class as the dropped filter in TKT-9FKX8X: the script asks
	// one question and unknowingly gets the answer to another.
	allowed := map[string]bool{"limit": true}
	for _, k := range known {
		allowed[k] = true
	}
	var unknown error
	tbl.ForEach(func(k, _ lua.LValue) {
		if unknown != nil {
			return
		}
		name, isStr := k.(lua.LString)
		if !isStr || !allowed[string(name)] {
			unknown = fmt.Errorf("unknown option %q", k.String())
		}
	})
	if unknown != nil {
		return o, unknown
	}

	switch lv := tbl.RawGetString("limit").(type) {
	case *lua.LNilType:
		// Absent: keep the ceiling.
	case lua.LNumber:
		n := int(lv)
		if float64(n) != float64(lv) {
			return o, fmt.Errorf("option \"limit\" must be a whole number, got %v", float64(lv))
		}
		// 0 is rejected rather than meaning "unbounded" — that is what it
		// means on store.ListEntitiesPage, and silently inheriting the
		// opposite meaning here would be the worst kind of near-miss.
		if n <= 0 {
			return o, fmt.Errorf("option \"limit\" must be positive, got %d", n)
		}
		o.limit = min(n, maxReadLimit)
	default:
		return o, fmt.Errorf("option \"limit\" must be a number, got %s", lv.Type())
	}
	return o, nil
}
