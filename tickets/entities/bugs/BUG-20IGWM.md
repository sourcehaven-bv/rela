---
id: BUG-20IGWM
type: bug
title: Search ordered property filters compare lexicographically (silently wrong)
description: search.MatchFilters evaluated FilterGt/Lt/Gte/Lte by comparing raw stringified attribute values with Go's string operators, so an integer/date property filter compared lexicographically ('10' < '9'). The search backend has no property-type context (it imports neither metamodel nor filter), so it cannot compare these correctly. The store search conformance suite even pinned the lexicographic behavior ('low > high lexicographically'). No application code populates Query.Filters today, so the bug was latent — but the ordered ops were a foot-gun waiting for the first caller.
priority: medium
why1: MatchFilters used val <= f.Value etc. on GetAttributeString output, which is byte-order comparison, not typed comparison.
why2: The search backend matches raw stringified values and has no property-type context to parse integers/dates before comparing.
why3: Ordered operators were added to the search filter API without a way to compare correctly, then pinned as lexicographic by the conformance suite rather than rejected.
why4: Research RES-6PK0S3 established that typed comparison belongs on the metamodel-aware filter.Match path, not search property filters — but the search API still offered ordered ops that could only be wrong.
why5: No application used Query.Filters ordered ops, so the silently-wrong comparison was never exercised and never caught.
prevention: search.ValidateFilters now rejects FilterGt/Lt/Gte/Lte with ErrOrderedFilterUnsupported, checked once up front in Service.Search; MatchFilters treats them as a defensive non-match. The store search conformance suite asserts the error instead of lexicographic results. Typed ordering is documented to belong on filter.Match (RES-6PK0S3, step 2).
status: done
---

## Bug

Found in the 2026-06-09 backend review (C2) and scoped by research
**RES-6PK0S3** as step 2 (option D2).

`search.MatchFilters` evaluated `FilterGt/Lt/Gte/Lte` with Go string operators
on `GetAttributeString` output:

```go
case FilterGt:
    if val <= f.Value { return false }   // "10" <= "9" is true → wrong
```

So an integer/date property filter compared **lexicographically** (`"10" <
"9"`). The search backend imports neither `metamodel` nor `filter`, so it has no
property-type context and *cannot* compare these correctly. The store search
conformance suite (`storetest/search.go`) even pinned the lexicographic result
with the comment "low > high lexicographically".

No application code populates `Query.Filters` today (only `storetest`), so the
bug was **latent** — but the ordered ops were a foot-gun for the first real
caller.

## Fix (PR pending)

Per RES-6PK0S3 (typed comparison belongs on the metamodel-aware `filter.Match`,
not search property filters):

- `search.ValidateFilters` rejects `FilterGt/Lt/Gte/Lte` with the new `ErrOrderedFilterUnsupported`, checked once up front in `Service.Search` (surfaced through the result iterator). `MatchFilters` treats them as a defensive non-match if validation is bypassed.
- The store search **conformance contract** changes: the four ordered-op cases now assert `ErrOrderedFilterUnsupported` instead of lexicographic hit-sets. This is an intended contract change (the old behavior was silently-wrong).

The equality / contains / in / exists ops are unaffected.

## Tests

- `internal/search/filter_ordered_test.go`: `ValidateFilters` rejects all four ordered ops and allows the supported ones; `MatchFilters` defensive-non-matches an ordered op.
- `storetest/search.go`: `OrderedFilterUnsupported/{FilterGt,Gte,Lt,Lte}` assert `ErrOrderedFilterUnsupported` across every store backend (fsstore/memstore/pgstore).
