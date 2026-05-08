---
id: RR-MP29E
type: review-response
title: Escape handling must stopPropagation to avoid global handler firing
finding: 'useKeyboardShortcuts.ts:44-61 has a global Escape branch that runs *before* the isInputFocused guard. While the palette is open with its input focused, an Escape keydown bubbles to the document handler which may try to blur() the now-removed input or router.back() on form pages. ConfirmModal solves this with e.stopPropagation() on Escape (lines 80-83). Required: in Approach, expand the keyboard handler description to call e.stopPropagation() on Escape, and e.preventDefault() on Arrow keys (caret movement). Add an integration test asserting Escape closes the palette without invoking router.back() on a form route.'
severity: significant
resolution: 'Plan updated: the input''s keydown handler now calls e.stopPropagation() on Escape (mirroring ConfirmModal:79-84) and e.preventDefault() on ArrowUp/ArrowDown/Tab/Shift+Tab. Added an integration test (AC7 + edge case ''Escape on form route'') asserting that opening the palette on /form/edit/... and pressing Escape emits close without invoking router.back().'
status: addressed
---
