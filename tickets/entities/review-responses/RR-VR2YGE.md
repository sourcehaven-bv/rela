---
id: RR-VR2YGE
type: review-response
title: RR-7Z3SFC's prefilled-relation drop generalizes to the duplicate at N-times the surface
finding: RR-7Z3SFC (critical, TKT-R4BMJM) found that prefilled relations are eaten by pruneWizardHiddenRelations and the cardRelations exclusion, producing 'entity created, no edge, no error'. Its shipped fix is a narrow post-condition at DynamicForm.vue:1451-1481 that checks ONLY linkParams.value?.as === 'to' — a single pre-linked peer. A duplicate prefills many peers across many relation types through relations.value, none of which that check covers, so the same silent-drop failure reappears at N times the surface. AC7 asserts the negative (unchecking a type means no edges); nothing asserts the positive survives the two pruning filters. AC6 would catch it in e2e only by accident.
severity: significant
resolution: RR-7Z3SFC named in the ticket and AC19a added, requiring prefilled edges to survive pruneWizardHiddenRelations and the cardRelations exclusion at N-peer scale.
status: addressed
---

## Resolution required

Name RR-7Z3SFC in the plan and state how its post-condition generalizes from one
pre-linked peer to an arbitrary prefilled relation set, rather than
rediscovering it during implementation. Treat the duplicate prefill as a third
input to the pruning reconciliation RR-OI6P51 asked for.

Add an AC asserting prefilled edges survive both `pruneWizardHiddenRelations`
and the `cardRelations` exclusion.
