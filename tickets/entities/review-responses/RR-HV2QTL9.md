---
id: RR-HV2QTL9
type: review-response
title: Done state must stay off the sidebar
finding: The sidebar payload is principal-independent; a done flag there would put per-principal data in it.
severity: significant
resolution: The sidebar carries only done_check; the state comes from /api/v1/_page_status.
status: addressed
---
