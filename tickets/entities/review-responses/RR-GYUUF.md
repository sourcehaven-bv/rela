---
id: RR-GYUUF
type: review-response
title: Tab trap will silently break a11y when more controls are added
finding: 'CommandPaletteModal.vue:187-191. e.preventDefault() on Tab keeps focus on input — fine for the happy path with only one tabbable element. But the moment someone adds a ''Clear'' button or filter chips, the trap blocks them. Fix: add a comment + test documenting the limitation: ''Only one focusable element — keep focus pinned. When more controls are added, swap for a real focus trap (TKT-X4P99 useFocusTrap).'' Defer until needed.'
severity: minor
resolution: 'Updated the inline comment on the Tab handler to make the limitation explicit: ''When more controls are added (clear button, filter chips), swap this for a proper focus trap.'' Future maintainers will know to revisit.'
status: addressed
---
