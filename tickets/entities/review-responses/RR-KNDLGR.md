---
id: RR-KNDLGR
type: review-response
title: AC5 must name its declared-set source to avoid coupling analysis to the world compiler
finding: arch-lint already gives analysis → metamodel, so AC5's subtraction needs no arch-lint change IF it reads *metamodel.Metamodel directly (analysis already holds it as Deps.Meta). If an implementer instead reaches for the compiled store.WorldScope — the natural instinct since PR-A builds the compiler — analysis needs `worlds` added to mayDependOn, which the plan does not list, and the analysis answer becomes dependent on world compilation succeeding. That is backwards for a detect-stranded-data check.
severity: minor
resolution: 'Fixed in PR-A (72bf2f21). Service.faceDeclared reads *metamodel.Metamodel directly (s.deps.Meta, already mandatory on analysis.Deps), never the compiled store.WorldScope. No arch-lint change was needed — analysis → metamodel was already permitted. The helper''s doc comment records why: the undeclared-face check is about DECLARATIONS, not worlds, so it must keep working on a project whose worlds: block is malformed; coupling it to world compilation would make the stranded-data report disappear exactly when the schema is broken.'
status: addressed
---

**Finding (design review, TKT-WAV8XP PR-A planning).**

Verified: `.go-arch-lint.yml` gives `analysis → {lua, metamodel, project,
schema, storage, store, tracer, validation}`, so AC5 needs NO arch-lint change
provided it reads the declared set from `*metamodel.Metamodel` — which
`analysis` already holds as `Deps.Meta`.

But the plan does not say which source AC5 reads, and the natural instinct for
an implementer is to reach for the compiled `store.WorldScope`, since PR-A
builds the compiler in the same PR. That would require adding `worlds` to
`analysis`'s `mayDependOn` (unlisted in the plan) and — worse — would make the
analysis answer depend on world compilation SUCCEEDING.

That is backwards. The undeclared-face check is about DECLARATIONS, not about
worlds, and a "detect stranded data" check must still work on a project whose
`worlds:` block is malformed.

**Resolution:** one line in the plan: AC5 reads `*metamodel.Metamodel` directly.
Prevents an unnecessary arch-lint edit and a real coupling.
