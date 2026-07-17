---
id: RR-TNMRC
type: review-response
title: AC5 lists 'and/or short-circuit' but semantics of short-circuit over a fail-safe evaluator need a note
finding: 'AC5 requires testing and/or short-circuit. With the ''eval errors return false, never throw'' rule, short-circuit interacts subtly: if the left of `A or B` errors, is it false (so B is evaluated) or does the whole expr fail-safe? Define: per-node eval errors coerce to false locally (so `errors or true` == true), NOT whole-expression bail. This keeps a single bad reference from nuking an otherwise-valid OR. Add a test. Nit-to-minor, but it''s exactly the kind of thing left ambiguous that causes surprising visibility bugs.'
severity: minor
resolution: 'Spec pins fail-safe semantics: per-node eval errors coerce to false locally (not whole-expression bail), so `brokenRef or form.ok == true` still yields true; and/or short-circuit normally. Parse failure -> constant false + warn. AC4 requires short-circuit tests with an erroring operand on both and/or.'
status: addressed
---
