---
id: RR-7VKNB
type: review-response
title: 'Reactivity/perf: evaluation cadence and parse-caching unspecified'
finding: 'Conditions are re-evaluated as the user types (that''s the point). The spec doesn''t say whether expressions are parsed once (cached AST) or re-parsed on every keystroke, nor how evaluation integrates with Vue reactivity (a computed over formData). For a ~15-step GDPR form with a condition per step/field this is many evals per keystroke. Cheap to get right if planned: parse each distinct expression string once (memoize AST by string), expose `compile(expr) -> Program` + `program.eval(bindings)` (mirroring predicate''s Program/Eval split), and let DynamicForm hold compiled programs in a computed. Minor because volumes are small, but specifying it avoids a re-parse-per-keystroke implementation.'
severity: minor
resolution: Spec adopts a compile/eval split (compile(expr)->Program, program.eval(bindings)) mirroring predicate, with module-level memoization of compiled programs by string. DynamicForm holds compiled programs in a computed and re-evals on formData change — no re-parse per keystroke. AC2 pins memoization.
status: addressed
---
