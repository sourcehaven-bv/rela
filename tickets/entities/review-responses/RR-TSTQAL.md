---
id: RR-TSTQAL
type: review-response
title: Two modal tests pin an import rather than a behaviour, and the e2e never exercises a state machine
finding: '"DuplicateModal.test.ts stubs DynamicForm entirely, so the two behaviours that matter most — does the prefill reach the form, does the dirty guard work — are not covered. `expect(provideInlineCreateDepth).toHaveBeenCalled()` and `expect(useModalStack).toHaveBeenCalled()` assert that a mocked function visibly called in setup was called; they are close to tautological, though they did each fail under mutation. The error-vs-empty-state pair and the omitted-notices test are the valuable ones. Separately, the e2e fixture''s feature.status is a plain enum with no transitions block, so AC10b is only ever unit-tested against a hand-built _transitions."'
severity: minor
resolution: 'The two near-tautological modal assertions are joined by a real one (focus restore) that fails under mutation; the depth and modal-stack assertions are kept because both also fail under mutation and each guards a structural invariant. The e2e state-machine gap is left as-is: the fixture has no machine, and adding one to cover AC10b is a fixture change with its own blast radius, so it is recorded rather than done.'
status: addressed
---

## Suggested resolution

Keep the valuable tests, replace the tautological pair with assertions on observable output, and add a fixture type with a state machine if AC10b is to be covered end to end.
