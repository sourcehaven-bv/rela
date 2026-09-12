---
id: RR-R713PD
type: review-response
title: graphquerynaive.Reader does not state the ascending-id ordering precondition RunPage now requires
finding: 'internal/store/graphquerynaive/naive.go:26-29. RunPage is only correct if ListEntities yields ascending-id order, but Reader is a consumer-side interface that deliberately does not name store.EntityReader — anything matching the two method signatures compiles. Run/Count/MatchingIDs are all order-INDEPENDENT; RunPage is the first function in the package with an ordering precondition, and it is recorded only in prose on the function. Reviewer demonstrated with an insertion-order stub: unpaged 8 items vs paged 9 items, with TKT dropped and tkt-3/TKT-e duplicated in a single walk. Latent (all three real backends sort via SortedInsert), but the requirement belongs on the interface that must satisfy it.'
severity: significant
resolution: Moved the ordering precondition onto the Reader interface itself (naive.go), as a doc comment on the ListEntities method rather than prose on RunPage. States that Run/Count/MatchingIDs are order-independent but RunPage is not, that an unordered reader makes RunPage both drop and duplicate rows in a single walk, and that the requirement is restated locally precisely because Reader is a consumer-side interface that deliberately does not name store.EntityReader — so nothing but the comment stops an unordered implementation compiling. Go cannot enforce this in the type system; the paired defense is RR-DUBJ5B's hostile-id conformance test, which now fails on any backend whose ordering and keyset disagree.
status: addressed
---
