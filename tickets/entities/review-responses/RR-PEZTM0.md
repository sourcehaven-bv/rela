---
id: RR-PEZTM0
type: review-response
title: q.Sort claimed ignored but not pinned by any test
finding: The VisibleSearcher godoc states q.Sort is ignored, but neither implementation was tested for it — a backend accidentally honoring Sort would violate the cross-backend ordering contract invisibly.
severity: minor
resolution: 'New conformance case SortIsIgnored: identical result stream with and without q.Sort, on every backend.'
status: addressed
---
