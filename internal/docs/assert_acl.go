package docs

import (
	"fmt"
	"sort"
	"strings"

	lua "github.com/yuin/gopher-lua"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// refuses{} / permits{} assert an ACL claim against the REAL decision path.
//
// # Why these route through acl.Declarative rather than reading Policy.Roles
//
// `roles_matrix{}` reads `policy.Roles` directly because it DESCRIBES the
// configuration — it renders a table of what the yaml says. An assertion makes
// a different kind of claim: that a principal is actually refused. Answering
// that from the same map would prove only that the manual and the matrix agree,
// which they would even if every gate were bypassed downstream.
//
// So these call `AuthorizeWrite` — the entry point the write path itself uses.
// A manual asserting "an auditor cannot update a control" then fails if the
// grant is widened OR if the gate stops being consulted.
//
// # Why a deny may name its rule
//
// `acl.Decision` carries RuleKind/RuleID precisely because opaque denials are
// unsupportable. A manual that says "refused because the role lacks the grant"
// can assert `because=` and will fail if the deny starts coming from somewhere
// else — a deny for an unintended reason is a passing test hiding a broken one.
func (dr *docRuntime) luaRefuses(ls *lua.LState) int { return dr.luaAuthz(ls, false) }
func (dr *docRuntime) luaPermits(ls *lua.LState) int { return dr.luaAuthz(ls, true) }

func (dr *docRuntime) luaAuthz(ls *lua.LState, wantAllow bool) int {
	verb := "refuses"
	if wantAllow {
		verb = "permits"
	}

	tbl := argTable(ls)
	if tbl == nil {
		return dr.luaFail(ls, `%s: expects a table, e.g. %s{who="auditor", op="update", type="policy"}`, verb, verb)
	}

	if rejectUnknownKeys(dr, ls, verb, tbl, "who", "op", "type", "id", "because", "unassigned") {
		return 0
	}

	who := fieldString(ls, tbl, "who")
	op := fieldString(ls, tbl, "op")
	typ := fieldString(ls, tbl, "type")
	id := fieldString(ls, tbl, "id")
	because := fieldString(ls, tbl, "because")
	unassigned := fieldBool(ls, tbl, "unassigned")

	if who == "" || op == "" || typ == "" {
		return dr.luaFail(ls, "%s: `who`, `op` and `type` are all required "+
			"(got who=%q op=%q type=%q) — an authorization claim is meaningless "+
			"without all three", verb, who, op, typ)
	}
	// Same reasoning as shows{}: an authorization claim about a type that does
	// not exist tells you nothing about the policy.
	if _, ok := dr.meta.Entities[typ]; !ok {
		return dr.luaFail(ls, "%s{type=%q}: no such entity type in the schema. Declared types: %s",
			verb, typ, strings.Join(declaredTypes(dr), ", "))
	}
	if !validOp(op) {
		return dr.luaFail(ls, "%s{op=%q}: unknown op — one of create, update, delete, rename", verb, op)
	}
	// A principal with no assignment has no grants, so it is refused BY
	// CONSTRUCTION — which makes every refuses{} with a misspelled `who` green
	// forever, unable to fail. That is the vacuous pass this feature exists to
	// prevent, so an unknown principal is an error.
	//
	// "This principal has no role" is nonetheless a REAL claim worth making
	// (there is no self-service sign-up), so it stays available — but it must
	// be stated, because an intended unassigned principal and a typo are
	// otherwise byte-identical to a reviewer.
	if dr.policy != nil && !unassigned {
		if _, ok := dr.policy.Assignments[who]; !ok {
			return dr.luaFail(ls, "%s{who=%q}: no such principal in acl.yaml's assignments. "+
				"An unassigned principal has no grants, so this claim would pass no matter "+
				"what the policy said. Fix the spelling, or write unassigned=true if the "+
				"point IS that this principal has no role. Assigned: %s",
				verb, who, strings.Join(sortedAssignments(dr.policy.Assignments), ", "))
		}
	}
	if unassigned && dr.policy != nil {
		if role, ok := dr.policy.Assignments[who]; ok {
			return dr.luaFail(ls, "%s{who=%q, unassigned=true}: that principal IS assigned "+
				"(role %q), so the claim describes the wrong thing", verb, who, role)
		}
	}

	if dr.policy == nil {
		return dr.luaFail(ls, "%s: the project has no acl.yaml, so there is no policy to assert "+
			"against. Remove the claim, or document a project that has one", verb)
	}

	// The seeded memstore is both the graph (for role-relation membership) and
	// the query backend, so a manual can seed the very edge that confers a role
	// and then assert what it grants.
	d, err := acl.NewDeclarative(dr.policy, acl.NewStoreGraph(dr.store), dr.store)
	if err != nil {
		return dr.luaFail(ls, "%s: building the evaluator failed: %v", verb, err)
	}

	ctx := principal.With(dr.ctx, principal.Principal{User: who, Tool: principal.ToolCLI})
	dec := d.AuthorizeWrite(ctx, acl.WriteRequest{
		Op:      acl.Op(op),
		Subject: acl.EntitySubject{Type: typ, ID: id},
	})

	if msg := checkAuthz(verb, who, op, typ, wantAllow, because, dec); msg != "" {
		return dr.luaFail(ls, "%s", msg)
	}
	return 0
}

// checkAuthz is the pure claim-vs-decision comparison, split out so the failure
// prose is testable without an evaluator.
func checkAuthz(verb, who, op, typ string, wantAllow bool, because string, dec acl.Decision) string {
	if dec.Allow != wantAllow {
		var b strings.Builder
		fmt.Fprintf(&b, "%s{who=%q, op=%q, type=%q} failed\n", verb, who, op, typ)
		if wantAllow {
			fmt.Fprintf(&b, "  claimed: permitted\n  actual:  REFUSED")
		} else {
			fmt.Fprintf(&b, "  claimed: refused\n  actual:  PERMITTED")
		}
		if dec.RuleKind != "" {
			fmt.Fprintf(&b, "\n  rule:    %s/%s", dec.RuleKind, dec.RuleID)
		}
		if dec.Reason != "" {
			fmt.Fprintf(&b, "\n  reason:  %s", dec.Reason)
		}
		return b.String()
	}

	// The decision matched. If the manual also named WHY, hold it to that: a
	// deny arriving from an unintended rule is a green test over a real change.
	if because != "" && !reasonMatches(dec, because) {
		return fmt.Sprintf("%s{who=%q, op=%q, type=%q}: the decision was right but the "+
			"reason was not\n  claimed because: %s\n  actual rule:     %s/%s\n  actual reason:   %s",
			verb, who, op, typ, because, dec.RuleKind, dec.RuleID, dec.Reason)
	}
	return ""
}

// sortedAssignments lists the assigned principals for a failure message, so a
// typo is fixable without opening acl.yaml.
func sortedAssignments(a map[string]string) []string {
	out := make([]string, 0, len(a))
	for k := range a {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// reasonMatches decides whether `because` pins the rule that actually fired.
//
// The rule identity (RuleKind, RuleID, or "kind/id") must match EXACTLY. It was
// a substring test, which meant `because="a"` matched almost any deny — a claim
// that pins nothing while looking like it pins something.
//
// The free-text Reason keeps substring matching, because it is prose and a
// manual should be able to quote the meaningful fragment rather than the whole
// sentence. A minimum length keeps that from becoming the same loophole.
func reasonMatches(dec acl.Decision, because string) bool {
	switch because {
	case dec.RuleKind, dec.RuleID, dec.RuleKind + "/" + dec.RuleID:
		return true
	}
	const minReasonFragment = 8
	return len(because) >= minReasonFragment && strings.Contains(dec.Reason, because)
}

func validOp(op string) bool {
	switch acl.Op(op) {
	case acl.OpCreate, acl.OpUpdate, acl.OpDelete, acl.OpRename:
		return true
	}
	return false
}
