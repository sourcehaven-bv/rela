---
id: RR-FTEW47
type: review-response
title: 'S5: compile-time RRULE validation describes a mechanism that does not exist'
finding: Plan AC4 said a bad RRULE literal fails at compile. But FuncSig is Params []Type plus Return Type - the compiler type-checks types not values and there is no per-argument compile-time validation hook. The only value-level compile work is literal coercion in walkRelational which is comparison-specific. Also rrule_next(new.schedule ...) takes its rule from a property which could never be compile-validated.
severity: significant
resolution: 'AC4 scoped down to: a malformed RRULE fails at Eval with a clear error. Adding a Validate func([]Value) error hook on FuncSig invoked only for const args was considered and rejected as out of scope.'
status: addressed
---
