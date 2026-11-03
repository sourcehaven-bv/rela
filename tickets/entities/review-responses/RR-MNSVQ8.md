---
id: RR-MNSVQ8
type: review-response
title: Undecidable-guard check and _actions.buttons are unsound for a caller holding two roles
finding: 'The load check judges earlier entries one poster class at a time, and a class is a single role (internal/realtimecfg/branches.go:157-175). A real caller can hold several roles, and the decider tries every entry that any of them admits (internal/realtime/session.go:219-224, 505-516). Example: entry 0 has roles [lead] and a session.reminders guard; entry 1 has roles [member] and a button. The plan loads this (AC 4 case ''earlier guard with session for a disjoint role loads''). sessionAffordances then lists entry 1 for a lead+member caller, but that caller''s click can take entry 0, moving to another state and running its outputs under a label that promised entry 1''s target. TestButtons_DeciderAgrees iterates ''for each role'', so it cannot catch this. Fix: TransitionButton records the earlier entries whose guard is undecidable. The affordance hides the button from any caller who admits one of them, through the same held-roles map. The load refuses only when every person class of the entry admits such an entry. Add a principal holding two roles to the agreement test, following examplePrincipals in internal/realtime/session_affordance_test.go:296.'
severity: critical
resolution: Each TransitionButton now records its shadowing and undecidable earlier entries, and the shared SessionState.Offers rule hides the button from any caller who admits one of them, judged on all held roles; session.state counts as decidable within its state. TestButtons_DeciderAgrees now includes a lead+member principal and asserts the shadow-hidden direction.
status: addressed
---
