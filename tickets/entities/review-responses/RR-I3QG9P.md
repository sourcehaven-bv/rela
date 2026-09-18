---
id: RR-I3QG9P
type: review-response
title: ViewSection.Sort is ambiguous about which level it orders, and the nested cap is per-parent
finding: 'Design review S10. (1) ViewSection carries TWO collections at different levels, Source and Children (config.go:1425-1432), each with its own entity type. A single `Sort []SortSpec` on ViewSection does not say which level it orders. The motivating use case from TKT-MJKZQ3 (children sorted by status with completed last, then due ascending) wants the CHILDREN. The plan proposes one field and never binds it to a level, so a YAML API decision would be made by accident at implementation time. Decide before it ships: either separate parent_sort/child_sort keys mirroring the existing ParentColumns/ChildColumns split, or one key with a documented level. The ParentColumns/ChildColumns precedent argues strongly for the former. (2) AC5 says sort before the cap as though there were one collection and one cap. nestedChildPreview is a PER-PARENT cap applied as min(len(kids), nestedChildPreview, budget) in sections_nested.go, where budget is a node budget consumed ACROSS parents. So the sort must run per parent''s child bucket before that min, and which parents get their full 25 depends on iteration order. A test seeding one parent with 30 children passes while the multi-parent budget interaction stays untested - and that is exactly where a user notices, because a late parent silently shows fewer and differently chosen children.'
severity: significant
status: open
---
