---
id: RR-0C3IZX
type: review-response
title: 'Plan claimed section sort: already exists; ViewSection has no Sort field'
finding: The ticket's config example and its 'Child sort order' section asserted that multi-key section sort already works, citing config.go:587 and :1229. Those Sort fields belong to List and dashboard cards. ViewSection (config.go:1301-1311) has no Sort field, and nothing in the view path sorts at all -- no sort. call exists in sections.go or views.go, so collection order is traversal/store order. The user's explicit requirement (sort by status then due date) was therefore recorded as free when it is new config plus a new sort step.
severity: critical
resolution: 'Ticket corrected: ViewSection gains `Sort []SortSpec` (reusing the existing SortSpec type so YAML matches lists/dashboards), applied to the child slice in the nested builder before the fold and budget so rows and counts agree, with sort properties validated against the child type the way columns: already are (validate.go:1519-1535). Config example annotated NEW rather than EXISTING. Scope increase acknowledged in the ticket rather than hidden.'
status: addressed
---
