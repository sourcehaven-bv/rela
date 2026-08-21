---
id: RR-5KYLPZ
type: review-response
title: 'C1: zero call-site edits incompatible with bool-returning Eval'
finding: 'Plan promised a facade whose Eval returns plain bool AND that 5 importers compile unchanged. Mutually exclusive: affordances/resolver.go:754 and statemachine/predicate.go:74 both type-assert v.(predicate.Bool) on the result. A bool return is a compile error there. Both call sites also have a defensive non-bool branch; in affordances that branch is fail-closed security (non-bool -> deny grant); a bool Eval silently deletes it.'
severity: critical
resolution: Eval keeps (Value error). Bool ergonomics via new additive predicate.EvalBool. Existing call sites and their fail-closed branches untouched.
status: addressed
---
