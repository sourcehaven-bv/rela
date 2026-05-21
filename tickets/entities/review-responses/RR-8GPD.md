---
id: RR-8GPD
type: review-response
title: Bindings exposes mutable maps as public fields, no validation symmetry with Env
finding: 'bindings.go: Bindings.Vars and Bindings.Funcs are exported map fields with no validation. Asymmetric with Env (which validates names, types, duplicates). Three problems: (1) caller can bind empty names, nil values, etc.; (2) Bindings doc says ''consumed by single Eval'' but the public map allows cross-goroutine mutation that contradicts the contract; (3) NewRecord/NewList don''t copy their input slice/map, so callers can mutate after passing in. Fix: make Bindings a validated builder symmetric with Env: NewBindings(); (*Bindings).SetVar(name, v) error; (*Bindings).SetFunc(name, f) error. Rejects empty names and nil values. Highest-value change because it''s a public API every ACL host call site will use.'
severity: significant
resolution: 'Reshaped Bindings as a validated builder: NewBindings(), (*Bindings).SetVar(name, v) error, (*Bindings).SetFunc(name, f) error. Vars/Funcs maps are now unexported. SetVar/SetFunc reject empty names, nil values, nil func impls. Eval(ctx, *Bindings, opts...) takes a pointer. NewRecord/NewList docs warn callers about ownership transfer. New env_test.go covers TestBindings_RejectsEmptyName, TestBindings_RejectsNilValue, TestBindings_RejectsNilFunc.'
status: addressed
---
