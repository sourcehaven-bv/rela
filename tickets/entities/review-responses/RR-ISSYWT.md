---
id: RR-ISSYWT
type: review-response
title: Display-only kind in the shared WidgetHintKind
finding: Edit-capable callers resolve WidgetHintKind, so a display-only component could be resolved in edit mode.
severity: minor
resolution: Added DenseHintKind and registry.resolveDense; WidgetHintKind and resolveFromHint are unchanged.
status: addressed
---
