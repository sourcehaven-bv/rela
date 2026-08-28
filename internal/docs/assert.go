package docs

import (
	"fmt"
	"sort"
	"strings"

	lua "github.com/yuin/gopher-lua"

	"github.com/Sourcehaven-BV/rela/internal/store"
)

// shows{} asserts what a manual's prose claims, against the seeded graph.
//
// # Why assertions live in the doc language at all
//
// Documentation and tests drift because they are separate artifacts. A manual
// that executes its own claims cannot drift: there is one artifact. The failure
// is a prose diff a reviewer reads, rather than an assertion message they skim.
//
// # Every argument is optional except the target
//
// A paragraph about visibility says nothing about buttons. `shows{}` asserts
// exactly the claims it is given and nothing else, so a call reads as the
// sentence above it rather than as a schema.
//
// # A call that asserts NOTHING is an error
//
// `shows{type="policy"}` is legal-looking and claims only that the query ran.
// That is the failure mode this project keeps meeting — a check that passes
// while checking nothing — so it is refused rather than quietly green. Any one
// claim is enough; zero is not.
//
// # `exactly` is the honest default for a list
//
// `contains` cannot see an over-inclusive result. A real defect during the
// content-states work returned `["CTL-2","CTL-1","CTL-2"]` — a duplicate and an
// extra — and a `contains={"CTL-2"}` assertion would have passed on it. The
// interesting bugs are over-inclusion, so `exactly` should be cheap to reach
// for.
func (dr *docRuntime) luaShows(ls *lua.LState) int {
	tbl := argTable(ls)
	if tbl == nil {
		return dr.luaFail(ls, `shows: expects a table, e.g. shows{type="policy", contains={"POL-1"}}`)
	}

	typ := fieldString(ls, tbl, "type")
	if typ == "" {
		return dr.luaFail(ls, "shows: `type` is required — it names the set being asserted about")
	}

	contains := fieldStringSlice(ls, tbl, "contains")
	absent := fieldStringSlice(ls, tbl, "absent")
	exactly := fieldStringSlice(ls, tbl, "exactly")
	hasExactly := hasField(tbl, "exactly")

	if len(contains) == 0 && len(absent) == 0 && !hasExactly {
		return dr.luaFail(ls,
			"shows{type=%q}: asserts nothing. Give at least one of contains=, absent= or "+
				"exactly= — a call with no claim passes whatever the code does, which is "+
				"worse than no call at all", typ)
	}

	got, err := dr.entityIDs(typ)
	if err != nil {
		return dr.luaFail(ls, "shows{type=%q}: %v", typ, err)
	}

	if msg := checkShows(typ, got, contains, absent, exactly, hasExactly); msg != "" {
		return dr.luaFail(ls, "%s", msg)
	}
	return 0
}

// entityIDs lists the seeded ids of one type, sorted so a failure message is
// stable and diffable.
func (dr *docRuntime) entityIDs(typ string) ([]string, error) {
	var ids []string
	for e, err := range dr.store.ListEntities(dr.ctx, store.EntityQuery{Type: typ}) {
		if err != nil {
			return nil, err
		}
		ids = append(ids, e.ID)
	}
	sort.Strings(ids)
	return ids, nil
}

// checkShows is the pure assertion core: it takes what was found and what was
// claimed, and returns a human-readable failure or "".
//
// Split out from the Lua binding so the rules are testable without a runtime,
// and so the failure TEXT is itself under test — a doctest's value is its
// failure output, and prose that only appears on a red build is prose nobody
// proofreads.
func checkShows(typ string, got, contains, absent, exactly []string, hasExactly bool) string {
	have := make(map[string]bool, len(got))
	for _, id := range got {
		have[id] = true
	}

	var missing, unexpected []string
	for _, id := range contains {
		if !have[id] {
			missing = append(missing, id)
		}
	}
	for _, id := range absent {
		if have[id] {
			unexpected = append(unexpected, id)
		}
	}

	if hasExactly {
		want := make(map[string]bool, len(exactly))
		for _, id := range exactly {
			want[id] = true
			if !have[id] {
				missing = append(missing, id)
			}
		}
		for _, id := range got {
			if !want[id] {
				unexpected = append(unexpected, id)
			}
		}
	}

	if len(missing) == 0 && len(unexpected) == 0 {
		return ""
	}

	var b strings.Builder
	fmt.Fprintf(&b, "shows{type=%q} failed", typ)
	if len(missing) > 0 {
		fmt.Fprintf(&b, "\n  missing:  %s", strings.Join(dedupe(missing), ", "))
	}
	if len(unexpected) > 0 {
		fmt.Fprintf(&b, "\n  unexpected: %s", strings.Join(dedupe(unexpected), ", "))
	}
	// The seeded set is printed on every failure, not just the exact-match one.
	// Most confusion when a world assertion fails is not knowing what was
	// actually there — the claim is easy to re-read, the state is not.
	fmt.Fprintf(&b, "\n  seeded %s: %s", typ, joinOrNone(got))
	return b.String()
}

func joinOrNone(ids []string) string {
	if len(ids) == 0 {
		return "(none)"
	}
	return strings.Join(ids, ", ")
}

func dedupe(in []string) []string {
	seen := make(map[string]bool, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

// hasField reports whether a key is present at all, which is not the same as
// its value being non-empty. `exactly={}` is a MEANINGFUL claim — "this type
// has no entities" — and must not be read as "no exactly claim given".
func hasField(tbl *lua.LTable, key string) bool {
	return tbl.RawGetString(key) != lua.LNil
}
