---
id: RR-WNQM
type: review-response
title: FilterBar missing onBeforeUnmount cleanup of debounce timer
finding: 'FilterBar''s textDebounceTimer is never cleared on unmount. If the component unmounts with a pending timer, the timer still fires after teardown. Vue 3 silently no-ops emit on unmounted components but it''s wasted work and a leak in dev tools. Fix: add onBeforeUnmount(() => { if (textDebounceTimer) clearTimeout(textDebounceTimer) }).'
severity: minor
resolution: FilterBar.vue adds onBeforeUnmount handler that clears textDebounceTimer so a pending debounced emit can't fire after the component unmounts. Folded into the RR-QTS0 fix since both touch the same file region.
status: addressed
---
