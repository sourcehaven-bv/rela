---
id: RR-6B5A0W
type: review-response
title: An empty sort property validates and does nothing
finding: 'validateSortSpecs skips a spec with an empty property and SortParam drops it so sort: [{direction: desc}] loads cleanly with no effect.'
severity: nit
reason: Pre-existing behaviour of list sort that the shared helper inherits. Rejecting it would turn configs that load today into load errors for lists too so it belongs in its own ticket.
status: deferred
---
