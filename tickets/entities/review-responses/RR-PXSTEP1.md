---
id: RR-PXSTEP1
type: review-response
title: Step 1 is described as behaviour-preserving; that holds today but nothing pins it
finding: 'Step 1 (move bypass_acl registration out of the allowWrites branch) is justified as "no behaviour
  change (today allowWrites is true wherever an elevated handle exists)". I verified that is true right
  now: the only elevation wiring sites are the cascade runner and the document render, both of which build
  writer runtimes.


  But the parenthetical is the whole safety argument, and it is a statement about the current call graph,
  not an invariant. After step 2 it becomes false by construction — that is the POINT of step 2 — so the
  two steps cannot be reordered or landed independently without thought.


  Worth stating in the ticket that step 1 is safe *because* of a property step 2 deliberately removes,
  and adding a test at step 1 that pins bypass_acl registration against a runtime built WITHOUT writes.
  Otherwise step 1 looks like an independent tidy-up that someone could land alone, months apart, with
  the reasoning lost.'
severity: minor
status: open
---
