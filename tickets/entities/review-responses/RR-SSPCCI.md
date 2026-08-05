---
id: RR-SSPCCI
type: review-response
title: GraphCount total is a count oracle over hidden rows
finding: PLAN-VZXHRJ proposes sourcing the count/total/truncated triple from GraphCount, which is documented as returning (matched, total) where total is 'the total number of entities of q.EntityType IGNORING those predicates'. The predicates ARE the ACL. Reporting that total to a script tells it how many rows of the type exist including ones the principal may not read — a count-based existence oracle, the same class of leak the read seam exists to close (hidden must be indistinguishable from nonexistent, DEC-ZBI39P). total must be the pre-cap count of VISIBLE rows (i.e. derived from matched), never the raw type count.
severity: significant
resolution: Plan now specifies total = matched (visible rows), never GraphCount's predicate-ignoring total, and truncated = matched > count. Added AC11 asserting the raw type count is not observable through the binding when M > N.
status: addressed
---

Raised by `/design-review` against PLAN-VZXHRJ, before implementation.

Note this is a leak the *old* code could not have: `PolicyReader.Filter` never
saw a pre-filter count, so there was nothing to leak. The pushdown makes the
unfiltered total newly available, and the plan reached for it because
`GraphCount` hands back both numbers.

The `truncated` derivation still works — it just has to be `matched > count`,
not `total > count`.
