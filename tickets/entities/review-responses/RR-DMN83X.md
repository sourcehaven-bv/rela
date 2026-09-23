---
id: RR-DMN83X
type: review-response
title: queryplan evaluator init and dedupe key
finding: StaticIndexSpecs builds ev lazily only for lists (queryplan.go:200-203); a nav entry that does not trigger it loses its scope columns. The dedupe key is built inline (:209-211). The EXPLAIN claim does not hold for scoped entries.
severity: minor
resolution: Init ev for nav entries that declare scopes; extract the dedupe key into a helper shared by list and nav specs; EXPLAIN claim removed from the plan.
status: addressed
---
