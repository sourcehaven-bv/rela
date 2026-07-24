---
id: RR-FJUQSF
type: review-response
title: FieldRedactor interface has no error channel — fail-closed contract must be explicit
finding: 'HiddenProperties(ctx, e) map[string]struct{} cannot signal an internal failure (matches affordances.FieldVerdicts, which also can''t error). A future impl that fails internally could silently return nil = ''nothing hidden'' = fail-OPEN. Document the interface contract explicitly: an impl that cannot compute verdicts MUST return the hide-everything set (or panic), never nil; the built-in adapter over affordances inherits the engine''s own fail-closed behavior. Add a godoc note + a conformance case with a deliberately failing redactor stub.'
severity: minor
resolution: 'Plan revised: FieldRedactor interface godoc carries the explicit fail-closed contract (cannot compute → hide-everything set, never nil); conformance includes a hide-everything stub case (PLAN-RR12W4 Approach + Negative Tests).'
status: addressed
---
