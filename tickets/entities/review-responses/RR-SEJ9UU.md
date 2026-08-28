---
id: RR-SEJ9UU
type: review-response
title: 'C2: Program must be a type alias; a wrapper breaks struct fields and CompileAll'
finding: If the facade changes Eval it needs a wrapper struct rather than an alias. That breaks *predicate.Program struct fields at affordances/affordances.go:46/53/65 and statemachine/statemachine.go:109; forces Compile and CompileAll to allocate; and requires preserving CompileAll progs[i]==nil contract pinned by lint_test.go:41-52.
severity: critical
resolution: Program is a type alias (type Program = expr.Program). Facade is ~30 lines of aliases plus Compile CompileAll WithMaxDepth WithStepBudget and EvalBool.
status: addressed
---
