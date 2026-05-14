---
id: RR-BJ5U
type: review-response
title: setHighlight reads state.phase to pick list, hover-during-transition could mis-target
finding: 'useBacktickAutocomplete.ts lines 493-506 `setHighlight` reads `state.phase` and selects `prefixItems` or `entityItems`. The popup template binds `@mouseenter` per row — the row exists during phase 1 OR phase 2 but not both. However, in the fast-typing collapse path (RR-RH10''s territory), phase transitions `pending→prefix→id` happen synchronously inside the setTimeout callback. If the user''s mouse happens to be over a row that''s about to disappear and the renderer commits the new phase, the queued mouseenter event from the just-rendered row could fire AFTER phase changed but BEFORE re-render — setHighlight then reads `state.phase===''id''` and indexes into `entityItems` (which is empty in that moment because runSearch hasn''t returned). Realistically this is hard to hit because the queued event fires synchronously, but the function is fragile to this kind of phase-tearing. Fix: have the caller (popup) pass the phase it intends, or have `setHighlight` accept an explicit `(phase, index)` pair. Defensive but cheap.'
severity: minor
reason: setHighlight reads state.phase to pick the active list. Phase-tearing during a fast transition is theoretical -- all phase transitions run synchronously inside Vue's reactive batch and there's no rAF or microtask boundary between phase change and the next hover/click event. If a real race surfaces a follow-up can refactor to take a phase argument explicitly.
status: deferred
---
