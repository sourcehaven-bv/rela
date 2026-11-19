---
id: RR-QORARGF
type: review-response
title: Configure UI has no label for display.link
finding: The Configure change list shows the raw key for display.link.
severity: nit
resolution: Not changed.
reason: The SETTING_LABELS map is keyed by the last path segment and already has a link entry for another setting; a fix needs a path-aware label lookup, which is out of scope.
status: wont-fix
---
