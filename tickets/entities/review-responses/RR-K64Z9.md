---
id: RR-K64Z9
type: review-response
title: paletteOpen module-level ref is service-locator-y
finding: 'useKeyboardShortcuts.ts defines paletteOpen at module scope, exports it, imports it in App.vue. Works for one global modal — shortcutsModalOpen and paletteOpen now make two; three would be a service locator. Fix: track as follow-up — extract useGlobalModals() composable or drive palette state via a Pinia store action when a third lands.'
severity: nit
reason: Nit-level. shortcutsModalOpen already follows the same module-level pattern; consistency with prior art beats a one-off refactor here. Revisit when a third global modal needs the same wiring — then extract useGlobalModals() or move to a Pinia store.
status: deferred
---
