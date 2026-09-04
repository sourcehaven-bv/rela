---
id: RR-O5W4A3
type: review-response
title: ACL pushdown has no header variant, so content-free listings miss gated principals
finding: visibility listPushdown (internal/visibility/pushdown.go:56-131) returns full entities on all three branches; GraphQueryer (internal/store/graphquery.go:177) has no header-projecting method, and gantt_handler.go:304-306 already documents falling back to full-entity loads for that reason. D.2 as written would serve headers only to AllowAll principals; every real ACL user keeps paying for the body.
severity: significant
resolution: 'D.2 gains an explicit sub-task: a store.GraphHeaderQueryer optional capability (GraphQueryHeaders, type-asserted like store.Formatter) with a pg-native projection and a generic fallback, plus visibility.listPushdownHeaders used by visibleReader for listings. The plan now states that both AllowAll and Query principals take the header path; the Go fallback (relation filters, free text) also reads headers.'
status: addressed
---
