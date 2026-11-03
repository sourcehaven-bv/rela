---
id: RR-UFL0OT
type: review-response
title: No test that the affordance agrees with the decider
finding: 'AC 4 only checks that transitions is a subset of post. The AC 3 rows pin values that are worked out by hand. Nothing checks that a listed type moves the session and that an unlisted postable type does not, which is the parity pattern the repo uses for affordances (internal/dataentry/CLAUDE.md:52-62, TestDetailAction_AffordanceAndGateAgree). fireable and transitionRoles collect roles in different ways (per role of the current state vs every state for mt, internal/realtime/session.go:132-151), so they can drift. Plan change: add TestAffordances_TransitionsAgreeWithPost in internal/realtime/session_test.go. For every fixture principal, every state, and every type in post: post with props that satisfy the guard on a channel stored in that state, then assert the session moved exactly when the type was in `_actions.transitions`.'
severity: significant
resolution: 'Plan updated: AC 5 TestAffordances_TransitionsAgreeWithPost posts every postable type per principal and state with guard-satisfying props; fireable and transitionRoles share a memoized heldRoles helper (Approach 3).'
status: addressed
---
