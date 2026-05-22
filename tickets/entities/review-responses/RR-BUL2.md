---
id: RR-BUL2
type: review-response
title: Concurrency contract on *Program undeclared
finding: Plan says step counter is per-Eval but doesn't say *Program is safe for concurrent Eval. Once ACL wires up, every authz check on every request shares one *Program. IR must be immutable (no caches, no memoization, no sync.Once lazy fields); Eval must allocate its own visitor state. Make this an explicit invariant in doc.go and pin with a `TestProgram_Eval_Concurrent` race-detector test that calls Eval from N goroutines with distinct Bindings.
severity: significant
resolution: 'Invariant documented in plan and will be repeated in doc.go: ''*Program is immutable after Compile, safe for concurrent Eval; Eval allocates per-call visitor state.'' AC9 + TestProgram_Eval_Concurrent (under -race) pin this.'
status: addressed
---
