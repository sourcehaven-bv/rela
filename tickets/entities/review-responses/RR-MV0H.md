---
id: RR-MV0H
type: review-response
title: Mid-flight PATCH races against route navigation and clobbers destination view
finding: 'handleCheckboxToggle captures `view = viewData.value` before awaiting the PATCH, then assigns `viewData.value = { ...view, entry: updated, sections: nextSections }` after. If the user navigates A→B during the in-flight PATCH for A, the watch fires `loadView(B)`, then the resolved PATCH for A overwrites B''s view with A''s data. UI now shows entity A but the route says B.'
severity: significant
resolution: Added `if (viewData.value !== view) return` reference-equality check before writing back to `viewData.value`. If the route changed during the in-flight PATCH, `loadView` would have replaced viewData with the destination entity's view; we now detect the swap and bail rather than clobber the destination.
status: addressed
---
