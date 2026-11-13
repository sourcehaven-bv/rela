---
id: RR-CG1QZ65
type: review-response
title: The Admits mapping depends on an unstated invariant
finding: In the engine an Admits refusal means skip; in a state machine it means 403. They agree only while each (from, to) has at most one branch.
severity: significant
resolution: 'Addressed in the plan: the invariant is written on decideMove, enforced by the duplicate-edge load error, pinned by a test over every compiled branch set, and decideMove fails closed (ErrIllegalTransition) when handed more than one branch.'
status: addressed
---
