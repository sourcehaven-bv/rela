package docs

import (
	"fmt"
	"sort"
	"strings"

	lua "github.com/yuin/gopher-lua"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

// reads{} / hidden{} assert the READ side of the ACL, which nothing else here
// could reach.
//
// # The gap these close
//
// `shows{}` queries the store with a WorldScope. That exercises WORLD
// RESOLUTION — which face a world selects — and nothing else. It runs as no
// principal at all, so it cannot express "a reader holding only
// `policy@published` cannot see the draft".
//
// `refuses{}`/`permits{}` call AuthorizeWrite. That is the write path.
//
// So a manual could claim the entire read-side security property of faces and
// worlds while asserting none of it, and a conformance suite built from those
// two verbs would prove the easy half. Every read-path finding in the worlds QA
// pass — history serving a draft body to a denied reader, `_faces` disclosing a
// face the caller may not read — lives in the half neither verb touches.
//
// # Why this routes through visibility.PolicyReader
//
// The same reasoning `refuses{}` records for AuthorizeWrite. Answering "may
// this principal read this?" from `policy.Roles` would prove only that the
// manual and the yaml agree, which stays true if the gate stops being
// consulted. PolicyReader IS the read-out surface the app wires
// (appbuild.go, dataentry/app.go), so a claim here fails if a grant widens OR
// if the gate is bypassed.
//
// # hidden{} is the load-bearing one
//
// A denied read is indistinguishable from a missing row: Reader.Get returns
// (nil, false, nil) for both. That is the security property, and it is also
// what makes `hidden{}` easy to pass for the wrong reason — a typo'd id is
// hidden too. So an id that exists in NO face is refused: a claim about
// concealment must be about something that is actually there to conceal.
func luaReads(dr *docRuntime, ls *lua.LState) int  { return luaReadClaim(dr, ls, true) }
func luaHidden(dr *docRuntime, ls *lua.LState) int { return luaReadClaim(dr, ls, false) }

func luaReadClaim(dr *docRuntime, ls *lua.LState, wantVisible bool) int {
	verb := "hidden"
	if wantVisible {
		verb = "reads"
	}

	tbl := argTable(ls)
	if tbl == nil {
		return dr.luaFail(ls, `%s: expects a table, e.g. %s{who="raj@example.com", type="policy", id="POL-1"}`,
			verb, verb)
	}
	if rejectUnknownKeys(dr, ls, verb, tbl, "who", "type", "id", "face", "emit") {
		return 0
	}
	show := fieldBoolDefault(ls, tbl, "emit", true)

	who := fieldString(ls, tbl, "who")
	typ := fieldString(ls, tbl, "type")
	id := fieldString(ls, tbl, "id")
	face := fieldString(ls, tbl, "face")

	if who == "" || typ == "" || id == "" {
		return dr.luaFail(ls, "%s: `who`, `type` and `id` are all required (got who=%q type=%q id=%q) — "+
			"a read claim names a principal, a type and a row", verb, who, typ, id)
	}
	if _, ok := dr.meta.Entities[typ]; !ok {
		return dr.luaFail(ls, "%s{type=%q}: no such entity type in the schema. Declared types: %s",
			verb, typ, strings.Join(declaredTypes(dr.meta), ", "))
	}
	if dr.policy == nil {
		return dr.luaFail(ls, "%s: the project has no acl.yaml, so there is no policy to assert "+
			"against", verb)
	}
	if _, ok := dr.policy.Assignments[who]; !ok {
		return dr.luaFail(ls, "%s{who=%q}: no such principal in acl.yaml's assignments. An "+
			"unassigned principal reads nothing, so hidden{} would pass no matter what the "+
			"policy said. Assigned: %s", verb, who, strings.Join(sortedAssignments(dr.policy.Assignments), ", "))
	}

	target := readTarget(id, face)

	// The vacuous-pass guard for hidden{}: a row that does not exist is hidden
	// from everyone, so the claim would hold against any policy — including one
	// that grants the world. Checked against the RAW store, which is the only
	// place "does this exist at all" can be answered without a gate.
	if _, err := dr.store.GetEntity(dr.ctx, target); err != nil {
		return dr.luaFail(ls, "%s{id=%q}: no such entity in the seeded graph. A row that does "+
			"not exist is hidden from every principal, so this claim would pass against any "+
			"policy. Seeded %s: %s", verb, target, typ, joinIDs(seededIDs(dr, typ)))
	}

	reader, err := readerFor(dr)
	if err != nil {
		return dr.luaFail(ls, "%s: %v", verb, err)
	}

	ctx := principal.With(dr.ctx, principal.Principal{User: who, Tool: principal.ToolCLI})
	_, visible, gerr := reader.Get(ctx, typ, target)
	if gerr != nil {
		return dr.luaFail(ls, "%s{id=%q}: the read gate errored: %v", verb, target, gerr)
	}

	if visible != wantVisible {
		return dr.luaFail(ls, "%s", checkRead(who, typ, target, wantVisible))
	}

	emitEvidence(dr.emit, show, readEvidence(dr, who, target, face, wantVisible))
	return 0
}

// readTarget builds the stored id a face claim addresses.
//
// A face is part of an entity's ADDRESS, not a query parameter: the bare id and
// `id@face` are two rows. Spelling that here keeps the manual writing
// face="draft" (which reads as English) rather than an id with an @ in it.
func readTarget(id, face string) string {
	if face == "" {
		return id
	}
	return id + "@" + face
}

// readerFor builds the gated reader over the seeded graph.
//
// Mirrors appbuild.scriptEntityReader: a DeclarativeGate over the evaluator,
// wrapped in a PolicyReader — the same two types the application wires, so a
// claim fails if the row gate or the face gate stops being consulted.
//
// # Why the redactor is a no-op, and what that costs
//
// The real FieldRedactor is built from internal/affordances, which
// internal/docs may not import (.go-arch-lint.yml scopes its visibility edge
// deliberately). So this asserts ROW and FACE gating — "may this principal see
// this row at all" — and NOT field-level `visible:` redaction.
//
// That boundary is stated rather than papered over: `reads{}` takes no
// `redacted=` key, so a manual cannot make a field-redaction claim that would
// pass vacuously against a NopRedactor. Field redaction is asserted through
// api{} instead, which goes over HTTP against a real server that has the
// genuine redactor wired.
func readerFor(dr *docRuntime) (visibility.Reader, error) {
	d, err := acl.NewDeclarative(dr.policy, acl.NewStoreGraph(dr.store), dr.store)
	if err != nil {
		return nil, fmt.Errorf("building the evaluator failed: %w", err)
	}
	gate, err := visibility.NewDeclarativeGate(d)
	if err != nil {
		return nil, fmt.Errorf("building the read gate failed: %w", err)
	}
	reader, err := visibility.NewPolicyReader(gate, visibility.NopRedactor{}, dr.store)
	if err != nil {
		return nil, fmt.Errorf("building the reader failed: %w", err)
	}
	return reader, nil
}

// checkRead renders the failure for a wrong visibility verdict.
//
// The two directions are different incidents and read as such: a row that
// should have been concealed and was not is a disclosure; one that should have
// been readable and was not is a lockout.
func checkRead(who, typ, target string, wantVisible bool) string {
	if wantVisible {
		return fmt.Sprintf("reads{who=%q, id=%q} failed\n"+
			"  claimed: %s can read this %s\n"+
			"  actual:  the row is HIDDEN from them\n"+
			"  (a denied read and a missing row are indistinguishable by design, so this is "+
			"either a missing grant or the wrong face)", who, target, who, typ)
	}
	return fmt.Sprintf("hidden{who=%q, id=%q} failed\n"+
		"  claimed: %s cannot see this %s\n"+
		"  actual:  the row was RETURNED — this is a disclosure, not a policy mismatch",
		who, target, who, typ)
}

// readEvidence states a read outcome in the reader's terms.
func readEvidence(dr *docRuntime, who, target, face string, visible bool) evidence {
	role := "no role"
	if r, ok := dr.policy.Assignments[who]; ok {
		role = fmt.Sprintf("role `%s`", r)
	}
	what := fmt.Sprintf("`%s`", target)
	if face != "" {
		what = fmt.Sprintf("the `%s` face of `%s`", face, strings.TrimSuffix(target, "@"+face))
	}
	if !visible {
		return evidence{
			claim: fmt.Sprintf("`%s` (%s) **cannot see** %s.", who, role, what),
			note: "The row exists. To this reader the response is identical to one for an id " +
				"that was never created, so it cannot be used to confirm the entity is there.",
		}
	}
	return evidence{claim: fmt.Sprintf("`%s` (%s) **can read** %s.", who, role, what)}
}

// seededIDs lists what WAS seeded for a type, for the unknown-id message.
func seededIDs(dr *docRuntime, typ string) []string {
	var ids []string
	for e, err := range dr.store.ListEntities(dr.ctx, store.EntityQuery{Type: typ, AllStates: true}) {
		if err != nil {
			return ids
		}
		ids = append(ids, e.ID)
	}
	sort.Strings(ids)
	return ids
}
