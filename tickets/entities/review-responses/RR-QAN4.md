---
id: RR-QAN4
type: review-response
title: Stale comment 'favicon only - v1 assets removed' in router.go never was accurate
finding: The comment at router.go:14 claimed '/static/ contains favicon only - v1 assets removed' but that was never true. The /static/ mount exposes the entire internal/dataentry/static/ subtree including static/v2/ (the Vue bundle), so GET /static/v2/index.html and GET /static/v2/assets/... are both reachable through the /static/ path in addition to the canonical / route. The comment is actively lying about the mount's surface.
severity: significant
resolution: 'Replaced the comment with an accurate one: ''Legacy /static/ mount. The Vue bundle is also reachable here as /static/v2/*, but the SPA''s built index.html references assets as /assets/*, served via the catch-all below.'' Did NOT tighten the actual mount surface (pre-existing issue, not this ticket''s scope) but documented the reality so future readers are not misled. Also updated the panic messages to include the filesystem paths (''static'', ''static/v2'') for better operational visibility when they fire.'
status: addressed
---
