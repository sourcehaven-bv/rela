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

	if rejectUnknownKeys(dr, ls, "shows", tbl, "type", "contains", "absent", "exactly") {
		return 0
	}

	contains, cerr := claimList(tbl, "contains")
	absent, aerr := claimList(tbl, "absent")
	exactly, eerr := claimList(tbl, "exactly")
	for _, err := range []error{cerr, aerr, eerr} {
		if err != nil {
			return dr.luaFail(ls, "shows: %v", err)
		}
	}
	hasExactly := hasField(tbl, "exactly")

	if len(contains) == 0 && len(absent) == 0 && !hasExactly {
		return dr.luaFail(ls,
			"shows{type=%q}: asserts nothing. Give at least one of contains=, absent= or "+
				"exactly= — a call with no claim passes whatever the code does, which is "+
				"worse than no call at all", typ)
	}

	// An unknown type yields an EMPTY set, which satisfies `exactly={}` and any
	// `absent=` claim — so a typo'd type makes exactly the negative claims
	// vacuous, and those are the ones this verb recommends. (`contains=` fails
	// safe, which is why the hole is easy to miss.)
	if _, ok := dr.meta.Entities[typ]; !ok {
		return dr.luaFail(ls, "shows{type=%q}: no such entity type in the schema. An unknown "+
			"type reads as an empty set, so absent= and exactly={} would pass no matter what "+
			"the code did. Declared types: %s", typ, strings.Join(declaredTypes(dr), ", "))
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

// declaredTypes lists schema entity types for a failure message, so a typo is
// fixable without opening schema.yaml.
func declaredTypes(dr *docRuntime) []string {
	out := make([]string, 0, len(dr.meta.Entities))
	for name := range dr.meta.Entities {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// claimList reads a list-of-ids claim, refusing anything that is not a table of
// strings.
//
// Both silent coercions here weaken a claim rather than strengthen it, which is
// the wrong direction to fail. A non-string element used to be DROPPED, so
// `exactly={"a", 42}` quietly asserted a smaller set than written. And a scalar
// `exactly="a"` reads as present-but-empty, which `hasField` correctly reports
// as a claim — so it silently became "assert this type is empty", the opposite
// of what the author wrote.
func claimList(tbl *lua.LTable, key string) ([]string, error) {
	v := tbl.RawGetString(key)
	if v == lua.LNil {
		return nil, nil
	}
	arr, ok := v.(*lua.LTable)
	if !ok {
		return nil, fmt.Errorf("%s must be a list, e.g. %s={\"ID-1\"} — a bare value silently "+
			"reads as an empty list, which asserts the opposite of what you wrote", key, key)
	}
	var out []string
	var bad []string
	arr.ForEach(func(_, item lua.LValue) {
		if s, ok := item.(lua.LString); ok {
			out = append(out, string(s))
			return
		}
		bad = append(bad, item.String())
	})
	if len(bad) > 0 {
		return nil, fmt.Errorf("%s contains non-string %s (%s) — dropping them would silently "+
			"weaken the claim", key, pluralize("entry", len(bad)), strings.Join(bad, ", "))
	}
	return out, nil
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

// rejectUnknownKeys fails when a table carries a key the verb does not know.
//
// A misspelled claim is INVISIBLE otherwise: `shows{type="x", contains={...},
// absnt={"y"}}` silently drops `absnt` and passes on the strength of the claim
// that WAS spelled correctly, so the author believes two things are checked
// when only one is. The "asserts nothing is an error" rule catches this only
// when the typo is the sole claim; this catches it when it hides beside a
// valid one, which is the harder case to notice.
//
// Rejecting unknown keys is safe here because these tables are a closed
// vocabulary — an assertion has no user-extensible options.
func rejectUnknownKeys(dr *docRuntime, ls *lua.LState, verb string, tbl *lua.LTable, known ...string) bool {
	allowed := make(map[string]bool, len(known))
	for _, k := range known {
		allowed[k] = true
	}
	var unknown []string
	tbl.ForEach(func(k, _ lua.LValue) {
		name, ok := k.(lua.LString)
		if !ok {
			return
		}
		if !allowed[string(name)] {
			unknown = append(unknown, string(name))
		}
	})
	if len(unknown) == 0 {
		return false
	}
	sort.Strings(unknown)
	sortedKnown := append([]string(nil), known...)
	sort.Strings(sortedKnown)
	dr.luaFail(ls, "%s: unknown %s %s — a misspelled claim would be silently "+
		"dropped, so it is refused. Known keys: %s",
		verb, pluralize("key", len(unknown)), strings.Join(unknown, ", "),
		strings.Join(sortedKnown, ", "))
	return true
}

func pluralize(word string, n int) string {
	if n == 1 {
		return word
	}
	return word + "s"
}
