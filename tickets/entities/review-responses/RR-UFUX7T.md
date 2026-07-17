---
id: RR-UFUX7T
type: review-response
title: analyze.go loop must read Detail off the new struct element (not re-derive from rule.Description)
finding: 'internal/dataentry/analyze.go:448 currently does `for _, id := range full.Violations { e,_ := st.GetEntity(ctx, id); ...; Message: rule.Description }` — it re-fetches each entity by ID and ignores the violation''s own message. When RuleResult.Violations changes from []string to a struct {EntityID, Detail}, this loop MUST be rewritten to iterate the struct and read v.EntityID + v.Detail. If the implementer only touches the checker and forgets this loop, Detail is silently dropped and the feature no-ops end-to-end. Fix: make step 4 explicit (struct element, use v.EntityID for GetEntity, set Detail: v.Detail on AnalysisIssue); add a test asserting AnalysisIssue.Detail is populated.'
severity: critical
resolution: Plan step 4 now explicitly rewrites the analyze.go:448 loop to iterate the RuleViolation struct (v.EntityID + v.Detail) and set AnalysisIssue.Detail; a test asserting Detail is populated guards the no-op regression.
status: addressed
---

**Finding:** `internal/dataentry/analyze.go:448` currently does `for _, id :=
range full.Violations { e, _ := st.GetEntity(ctx, id); ...; Message:
rule.Description }`. It re-fetches each entity by ID and ignores the violation's
own message. When `RuleResult.Violations` changes from `[]string` to a struct
`{EntityID, Detail}`, this loop MUST be rewritten to iterate the struct and read
`v.EntityID` + `v.Detail`. If the implementer only touches the checker and
forgets this loop, `Detail` is silently dropped and the feature no-ops
end-to-end.

**Resolution required in plan:** make step 4 explicit — the loop variable
becomes a struct; use `v.EntityID` for the `GetEntity` call and set `Detail:
v.Detail` on the `AnalysisIssue`. Add a test asserting the
`AnalysisIssue.Detail` is populated (guards against this regression).
