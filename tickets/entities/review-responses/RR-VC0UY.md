---
id: RR-VC0UY
type: review-response
title: Keydown handler bound to input only, not the dialog
finding: 'CommandPaletteModal.vue:231 binds @keydown to <input>. ConfirmModal binds it to the overlay (ConfirmModal.vue:97). If focus ever leaves the input while palette is open, Escape/Arrow keys go nowhere. Today this is ''fine'' because Tab is preventDefault''d and clicks on options navigate-and-close — but it''s brittle (system overlay steals focus, screen-reader VO+arrow nav, future ''clear'' button). Fix: bind handler to the modal container or overlay so keydown bubbles up. Mirror ConfirmModal''s approach.'
severity: significant
resolution: 'Moved @keydown="handleKeydown" from the input to the .cmdk-overlay div so it catches keydown via bubbling regardless of which descendant is focused. Updated existing tests to dispatch with bubbles: true. Mirrors ConfirmModal:97 pattern.'
status: addressed
---
