---
id: RR-MJGVME2
type: review-response
title: CSS zoom may disturb drag coordinates
finding: Zooming the main pane in present mode can offset pointer coordinates for kanban drag.
severity: significant
resolution: Manual verification of kanban drag in present mode is an implementation-checklist item; fall back to no scaling if it breaks.
status: addressed
---
