---
id: RR-F4S4B0
type: review-response
title: loadGuardHolds duplicates the decider's guard bindings in a second package
finding: 'internal/realtimecfg/buttons.go:321-354 copies internal/realtime/session.go:568-610 (guardHolds): the same message/session record, predicatefns.Bind, ErrNilOperand rule and non-bool error. The prop rules were moved into realtimecfg so the load and the post run one copy; the guard evaluation was left as two copies. undecidable() limits the drift today, but a change to how message or session.state is bound, or to the nil-operand rule, must now be made in two places and nothing fails if one is missed. Fix: move the evaluation into realtimecfg, e.g. func GuardHolds(ctx, prog, fields, SessionFacts{State, Generation, EnteredAt, Reminders}, now) (bool, error). Have realtime.guardHolds call it with the live facts and loadGuardHolds call it with zero facts. The post path''s behaviour stays the same and there is one binding.'
severity: minor
resolution: Moved the guard binding into realtimecfg.GuardHolds with a GuardFacts session record (new internal/realtimecfg/guard.go). The decider calls it with the live facts and reminder counters; loadGuardHolds calls it with the owning state and zero counters. realtime.guardHolds is gone.
status: addressed
---
