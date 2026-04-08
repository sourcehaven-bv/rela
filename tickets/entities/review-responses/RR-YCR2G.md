---
id: RR-YCR2G
type: review-response
title: ConfirmModal does not restore focus when closed
finding: 'ConfirmModal focuses the Cancel button on open but never restores focus to the previously-focused element when it closes. Focus drops to <body>, which is a screen-reader regression and breaks keyboard visual focus indicators. Standard WAI-ARIA dialog pattern: save document.activeElement on open, restore it on close.'
severity: significant
resolution: ConfirmModal.vue now saves document.activeElement on open (into a previouslyFocused ref) and restores focus to it when open flips back to false. Added a unit test that creates a trigger button, focuses it, opens the modal, asserts Cancel has focus, closes the modal, and asserts the trigger regains focus.
status: addressed
---
