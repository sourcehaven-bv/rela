---
id: RR-WUB2
type: review-response
title: Anchor not refreshed when editor resizes mid-session (fullscreen toggle)
finding: 'MarkdownEditor.vue lines 50-53 watches `popupState.value?.anchor` and refreshes `editorRect`. But `anchor` only changes when `placeAnchor()` is called — currently from `onInputRead` (initial open) and `transitionToPrefix` (after delay). If the user toggles fullscreen WHILE the popup is open (phase=prefix or phase=id), `placeAnchor` doesn''t re-run, so `state.anchor` stays at the pre-fullscreen pixel coords. The window resize listener (line 143) refreshes `editorRect` but not `state.anchor`. Result: popup is positioned using old anchor minus new editorRect, landing somewhere off the trigger character. Practical impact is low (users probably don''t fullscreen mid-typing), but the architecture is incomplete. Fix: re-call `placeAnchor` (or expose it via the controller) on window resize / fullscreen change.'
severity: minor
reason: Fullscreen toggle mid-session is an unusual user action and the cleanest fix (re-anchor on fullscreen change) requires subscribing to EasyMDE-specific events that are not in the public API. Worth a future polish ticket once the basic flow is in user hands.
status: deferred
---
