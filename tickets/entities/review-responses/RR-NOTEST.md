---
id: RR-NOTEST
type: review-response
title: The riskiest 60 lines in the branch have no unit tests
finding: '"routePrefilledCardRelations and applyEmbeddedPrefill have zero direct test coverage — a grep for embeddedPrefill/routePrefilledCardRelations/applyEmbeddedPrefill across frontend/ and e2e/ hits only the two components that define and use them. routePrefilledCardRelations mutates pendingCardChanges and deletes from relations.value, and the branch''s own history (commit 44fe6f1f) says it was added to fix a silent data-loss bug. applyEmbeddedPrefill deliberately alters the commit filter by marking properties userTouched. The e2e covers one form configuration (the fixture''s cards widgets), which is why the original bug was caught — but every edge case in the other findings is a form-config permutation e2e will never enumerate."'
severity: critical
resolution: Added DynamicForm.prefill.test.ts (7 cases through a real form mount, asserting the create payload) and prefillRouting.test.ts (7 cases on the pure routing decision). The routing test is mutation-verified.
status: addressed
---

## Suggested resolution

Add unit tests for both functions. DynamicForm.embedded.test.ts already exists as a home.
