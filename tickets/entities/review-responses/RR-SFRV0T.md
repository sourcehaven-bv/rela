---
id: RR-SFRV0T
type: review-response
title: Failed candidate fetch degrades silently (console.error only)
finding: FilterBar.vue loadRelationCandidates logs a non-cancel fetch failure via console.error only. A user whose candidate fetch 500s gets an empty filter widget with no visible explanation. The codebase has uiStore.error/toast conventions for surfacing load failures. Consider routing through it, or at least documenting that a failed candidate load degrades silently.
severity: minor
resolution: 'FilterBar.vue: a non-cancel candidate-fetch failure now calls uiStore.error(...) with the control label, in addition to console.error, so the user sees a toast instead of a silently-empty widget. Tested: ''a non-cancel fetch failure surfaces a toast and leaves the widget empty''.'
status: addressed
---
