---
id: RR-AVSNSN
type: review-response
title: List ?q= under field hiding does a per-hit load and traversal
finding: freeTextIDs pays one entity load and one traversal per search hit on list ?q=, against collection-reads.md.
severity: significant
reason: 'Deferred to BUG-57A53F, which already tracks it (see RR-8QT70P). It ships as an accepted, budget-pinned exception to collection-reads.md: without it, list ?q= matches text in fields hidden from the caller, a value leak, and closing that leak outweighs the cost. The cost is bounded: search caps a list query at 1,000 hits, and TestQueryBudget_ListFreeTextUnderFieldHiding pins exactly one entity load and one traversal per hit, so any growth fails the test. Batching needs a cross-backend change to the shared SearchVisibleFields contract.'
status: deferred
---
