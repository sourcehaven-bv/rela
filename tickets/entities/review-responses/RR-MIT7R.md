---
id: RR-MIT7R
type: review-response
title: Transitions-info vertical spacing regressed (14px -> 8px)
finding: Old .transitions-info had margin-top:8px AND lived as direct child of .form-field (gap:6px), total ~14px gap. New SelectWidget drops margin-top and wraps panel in .select-widget {gap:8px}. Net 14px -> 8px. Small but visible.
severity: significant
resolution: 'SelectWidget.vue: added margin-top:6px on .transitions-info. Combined with .select-widget gap:8px (which only applies between the select and the panel when the panel is the only sibling), the visible stack is now 14px to match the pre-refactor spacing (old layout had .form-field gap:6px + .transitions-info margin-top:8px = 14px).'
status: addressed
---
