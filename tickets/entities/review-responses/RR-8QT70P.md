---
id: RR-8QT70P
type: review-response
title: List ?q= under field hiding has no budget test
finding: List ?q= now runs SearchVisibleFields with a field verdict per hit (up to 1000 hits). collection-reads.md requires a budget test under a field-hiding policy.
severity: minor
reason: 'Added TestQueryBudget_ListFreeTextUnderFieldHiding. It shows the list reads are size independent but two per-hit terms remain inside the shared SearchVisibleFields contract (also paid by /_search and the MCP searcher): one entity load per hit in search.Visible and one traversal per hit for a related() when:, because HiddenFieldsFunc has no batch hook. The test pins both at exactly one per hit. Removing them is a cross-backend contract change, filed as BUG-57A53F with measure MEAS-FREETEXT-VERDICT-BATCHED.'
status: deferred
---
