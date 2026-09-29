---
id: RR-QHSLAD
type: review-response
title: Reader-side relation and export disclosure not covered
finding: '[security] Reader specs never checked relations or export. Live, the published-only reader sees POL-1@draft''s implements edge to CTL-1 on POL-1@published and on CTL-1 (a product disclosure outside this diff). The edge fixmes ran as the editor only.'
severity: significant
resolution: Added reader-scoped fixme specs for both relation views and for export. All three fail today. The product leak is reported to the orchestrator for a bug ticket.
status: addressed
---

[security] Reader specs never checked relations or export. Live, the
published-only reader sees POL-1@draft's implements edge to CTL-1 on
POL-1@published and on CTL-1 (a product disclosure outside this diff). The edge
fixmes ran as the editor only.
