---
id: RR-SH85S4
type: review-response
title: Confirm-gate logic must land in a composable — DynamicForm.vue is 1886 lines against a 500-line threshold
finding: |-
    frontend/eslint.config.js:118 sets 'max-lines': ['warn', { max: 500 }]. DynamicForm.vue is already ~1886 lines (3.8x over) and is the god-component the project's own linting flags. The plan adds four more concerns to it: retained-value state, the confirm gate state machine, revert-with-suppression-generation, and hypothetical condition evaluation — roughly another 150 lines on the hottest path in the file.

    frontend/CLAUDE.md conventions favor composables, and the mechanism is self-contained with no template coupling. It should land as its own composable (e.g. useHiddenFieldPolicy.ts) alongside useAutoSave / useFormWizard, with DynamicForm.vue only wiring it. This also gives the missing unit tests a seam to bind to — note there is currently NO unit test for pruneWizardHidden or affordanceVisible at all (BUGA-QUI067), and the two hardest findings (cascade semantics, retention storage) are pure logic in exactly those functions, where unit tests are far cheaper than e2e.

    The plan is silent on placement.
severity: significant
resolution: |-
    Implemented as frontend/src/composables/useHiddenFieldPolicy.ts (196 lines) with its own 22-test unit suite. DynamicForm.vue only wires it: policyFor/propertyDef/labelFor/confirm callbacks in, retain/resolveHide/withSuppression out.

    This gave the missing tests a seam, which mattered — the analysis found there was NO unit test for pruneWizardHidden or affordanceVisible at all, and the two hardest design findings (retention storage, revert suppression timing) are pure logic now covered by fast unit tests rather than only e2e.

    Partial on the line count: DynamicForm.vue went 1886 -> 1999. The policy MECHANISM is fully extracted; the residual growth is wiring plus the watcher rewrite (which necessarily lives with the watcher). Still ~4x the 500-line warn threshold, unchanged in kind from before this ticket — decomposing the component is TKT-N0IKN9's job, not this bug fix's.
status: addressed
---
