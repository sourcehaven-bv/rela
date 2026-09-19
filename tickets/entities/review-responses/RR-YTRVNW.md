---
id: RR-YTRVNW
type: review-response
title: Plan claimed a Max fail-closed guarantee the read path does not deliver for ACL-hidden targets
finding: 'The plan''s security section promised to preserve ''an unevaluable target counts as matching when Max is set'', implying it covers ACL-hidden targets. It does not: visibility.PolicyReader.FilterRelations (internal/visibility/policyreader.go:161) drops an edge when either endpoint is invisible, before the evaluator runs. A hidden target therefore never reaches GetEntity and never reaches the failClosed branch — it is silently undercounted. The criterion as written would have passed on a regression, because it described a property that was never true.'
severity: critical
resolution: 'Mechanism verified and accepted; the CONCLUSION that it is a soundness hole is rejected. Validation runs over an ACL-pruned graph by design — a gate is a statement about the visible graph, not a global invariant — and CLI/CI wire an unrestricted reader for the authoritative verdict. Two corrections to the review: RelationConstraint''s godoc (types.go:1504-1517) is already accurate (it states the drop and that it matters mainly for Max), and validation.go:583-590 is accurate for its own scope (edges that REACH the loop). Neither is a lie. Plan now states the true behaviour and claims only ''behaviour unchanged, including the pruning''. In scope: one cross-referencing sentence at validation.go:583-590 noting the upstream pruning, so the next reader does not infer a stronger property. The failClosed branch genuinely covers dangling edges and MatchAll errors and stays.'
status: addressed
---

Reviewed with the user, who framed it correctly: "validation runs a graph that
is pruned based on acl - so it might not see issues someone else sees with a
fuller graph view". That is the intended semantics, and `docs/metamodel.md`
already documented it honestly.
