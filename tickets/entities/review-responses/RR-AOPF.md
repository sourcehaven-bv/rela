---
id: RR-AOPF
type: review-response
title: Duplicated `beforeEach` + `mountWith` setup across three describe blocks
finding: The new `describe('AnalyzeView entity title rendering', ...)` block re-declares the same `beforeEach` and inline `mountWith` helper as the two earlier describe blocks at lines 61-75 and 169-182. Three identical copies; not introduced by this PR but the third copy makes the smell visible. A 10-minute follow-up could hoist a shared setup helper to module scope.
severity: nit
reason: The duplication (3 copies of beforeEach + mountWith) is pre-existing — two copies already lived at lines 61-75 and 169-182 before this PR. Hoisting the shared setup is a cross-block refactor on an unrelated test file, outside the scope of this one-line fix. Keeping the work atomic per the project's bias against scope creep. A future test-quality sweep can fold all three into a shared module-scope helper.
status: deferred
---
