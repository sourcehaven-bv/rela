---
id: RR-5J4RI
type: review-response
title: useKeyboardShortcuts doesn't check isAnyModalOpen()
finding: 'useKeyboardShortcuts.handleKeydown does NOT call isAnyModalOpen(). isInputFocused() papers over most cases, but the Escape branch is checked BEFORE isInputFocused and explicitly calls router.back() on form pages. The palette adds e.stopPropagation() to suppress this — works ONLY because the palette''s keydown handler is on the input. If focus ever drifts (future close button, browser quirk), Escape escapes the palette and triggers router.back() on the underlying form. useListActions:151 already checks isAnyModalOpen — useKeyboardShortcuts should too. Fix: add `if (isAnyModalOpen()) return` early in handleKeydown (after the Cmd+K branch, since Cmd+K must still work to *re*-open, and Escape must still work for ConfirmModal which already stopPropagates).'
severity: significant
resolution: 'Added `if (isAnyModalOpen()) return` early in useKeyboardShortcuts.handleKeydown, after the Cmd+K branch (so Cmd+K still works to open palette on top of any modal). New test suite ''modal-stack gate'' verifies that on a form-edit route with a modal registered: Escape doesn''t trigger router.back(), ? doesn''t open shortcuts modal, g-prefix nav doesn''t navigate.'
status: addressed
---
