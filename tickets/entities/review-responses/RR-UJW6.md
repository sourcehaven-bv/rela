---
id: RR-UJW6
type: review-response
title: Compile signature undeclared — where does Env come from
finding: 'Plan describes Env as required for symbol resolution but never shows the actual signature of Compile. The candidate `Compile(env *Env, source string) (*Program, error)` is the right shape per CLAUDE.md (constructor rejects nil required fields). Document: nil env is a CompileError, not a panic. Pin with `TestCompile_RejectsNilEnv`. Also document where EvalOptions lives — should be per-Eval call (per-request budgets vary), not on *Program: `Eval(b Bindings, opts ...EvalOption) (Value, error)`.'
severity: significant
resolution: API surface section now declares Compile(env *Env, source string) (*Program, error) and Eval(b Bindings, opts ...EvalOption) (Value, error). Nil env is a CompileError, pinned in AC3 + TestCompile_RejectsNilEnv. EvalOption is per-call (WithStepBudget); compile depth budget uses a matching CompileOption.
status: addressed
---
