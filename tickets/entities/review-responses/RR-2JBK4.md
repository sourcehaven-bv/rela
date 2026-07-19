---
id: RR-2JBK4
type: review-response
title: 'Minor: WithMachines concurrency claim + double-Compile determinism comment both understated their assumptions'
finding: '#6: WithMachines mutates r.machines on a struct doc''d ''safe for concurrent use'' with only a doc-comment guard; safe today (called synchronously before escape) but aspirational. #7: the double-Compile comment claimed ''deterministic -> identical'' without stating the required meta-immutability assumption.'
severity: minor
resolution: '#6: WithMachines doc now states precisely - it is the one mutator, safe only when called during single-threaded wiring before the resolver is shared (the sole production call site), a setter not a New param deliberately to avoid churning 31 callers, never call concurrently with TransitionVerdicts. #7: comment now states both determinism AND meta-immutability, and clarifies correctness rests on shared evalEdge not instance identity. No behavior change; passing machines via New deferred as it would churn all callers for no live benefit.'
status: addressed
---

## Findings (bundled minors)

- **#6 — WithMachines concurrency.** Mutates `r.machines` on a "safe for
concurrent use" struct, guarded only by a doc comment. Safe today (called
synchronously in `ResolverFromProfile` before the resolver escapes), but the
claim was aspirational. **Fixed:** doc now states it's the one mutator, safe
only during single-threaded wiring before sharing, a setter-not-a-`New`-param by
deliberate choice (avoids churning 31 `New` callers), never call concurrently
with `TransitionVerdicts`. Threading via `New` deferred — no live benefit for
the churn.
- **#7 — double-Compile.** The comment sold "deterministic → identical" without
stating the meta-immutability assumption it also relies on. **Fixed:** comment
now names both (determinism + load-once read-only meta) and clarifies
correctness rests on the shared `evalEdge`, not instance identity.
