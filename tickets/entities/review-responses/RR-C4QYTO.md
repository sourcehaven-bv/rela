---
id: RR-C4QYTO
type: review-response
title: Delegation switches string sorts to natural order while pushdown keeps byte order
finding: 'Design review C2. filter.compareStrings (sort.go:122-129) uses natsort.Less, which is numeric-chunk-aware AND case-insensitive. applyV1Sorting (api_v1.go:2243-2248) and pgstore (COLLATE "C", graphquery.go:537) are raw byte order. Independently verified: [Zebra apple item10 item9] sorts to [apple item9 item10 Zebra] in Go vs [Zebra apple item10 item9] byte-wise. queryplan.StringShaped (queryplan.go:119) admits PropertyTypeString as pushdown-eligible, so after delegation the SAME request returns byte order when pushed and natural order on the Go path (add ?q=, an `in` filter, or run on fsstore/memstore). This breaks the equivalence contract documented at listpushdown.go:26-33 ("ordering by such a property is byte-wise on both sides") for the most common sort key in the product, `title`. TestListPushdown_MatchesGoPathPageForPage will NOT catch it: its fixture sorts by `due` and `title` with same-case, digit-free values, where the two orders coincide. Requires an explicit decision: (a) make SortMulti byte-wise for strings (regresses search/dashboard ordering), (b) narrow StringShaped to decline string SORT eligibility while keeping equality filters, or (c) teach SQL natural order (not indexable). The plan''s "OUT of scope: natural-number sort on strings" reads as a future feature; it is present behaviour on the delegation target.'
severity: critical
status: open
---
