---
id: RR-BI84Z0
type: review-response
title: Poster rule implemented twice with different shapes
finding: 'realtimecfg entryAdmits (branches.go:24) and realtime poster.admits (realtime/session.go:220) encode the same rule: roles exclude triggers, by: trigger admits only triggers, otherwise anyone. They agree only because roles together with by: trigger is refused at load. The load checks (firesFor, unreachable, transitionCause) and the decider must agree, so a shared table test that runs both over the same entries would catch drift.'
severity: nit
resolution: The poster rule now has one definition, the exported Transition.Admits (a nil holds means a trigger); entryAdmits and the decider's poster.admits both call it, and TestTransition_Admits runs both over the same table.
status: addressed
---
