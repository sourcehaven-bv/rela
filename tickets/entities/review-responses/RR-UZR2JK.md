---
id: RR-UZR2JK
type: review-response
title: 'Re-anchor the endpoint on _views, not calendars: entry gate, 404-parity, three caps already exist'
finding: 'The plan says ''mirror calendars: exactly'', but calendars have no server-side endpoint, no traversal, and no per-surface enforcement — no view type does (route table api_v1.go:84-120 has no _calendar/_kanban/_list). The actual precedent for a server-side nested-traversal endpoint is GET /api/v1/_views/{entityType}/{entityId} (api_v1.go:115, views_handler.go:402), which already implements: (1) entry gate BEFORE traversal (views_handler.go:427-433) so the pipeline never runs for a denied principal and a hidden id 404s identically to a missing one; (2) byte-identical 404 bodies via the shared entityNotFoundTitle const (RR-NGMI); (3) three independent bounds — fixpoint cap of 10 passes (views.go:34-43), per-rule MaxDepth default 10 (views.go:91-101), visited cycle guard (views.go:167-178). The gantt handler should copy this shape rather than reinvent it. One bug NOT to copy: views.go:105-110 swallows an unparseable where: and continues unfiltered (silent widening) — gantt sources must fail closed. Config-side mirroring of calendars (struct/validation/normalization/wire) remains correct; it is only the ENDPOINT that anchors on _views.'
severity: significant
resolution: 'Plan re-anchored: Research now splits the precedent in two — config half mirrors calendars: (struct/validation/wire passthrough), endpoint half anchors on _views (entry gate before traversal, entityNotFoundTitle 404-parity, the three bounds, raw-traverse-redact-once). The views.go:105-110 where:-swallow is explicitly called out as the bug not to copy; unparseable gantt where: is a load error.'
status: addressed
---
