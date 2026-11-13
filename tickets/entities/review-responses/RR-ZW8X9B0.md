---
id: RR-ZW8X9B0
type: review-response
title: Test helper rewrites engine internals
finding: useAutomations replaces e.automations and recompiles, bypassing compileConditions, rejectFilePropertyWrites and resolveNotifyPaths.
severity: nit
reason: The three typed-comparison tests that use it set only when clauses and set actions, which those steps do not touch. The helper keeps their assertions unchanged, which was the acceptance criterion; M5c removes the package.
status: wont-fix
---
