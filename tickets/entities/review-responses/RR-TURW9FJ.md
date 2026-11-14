---
id: RR-TURW9FJ
type: review-response
title: Concurrency and constructor contracts
finding: State that the compiled table is immutable. SetTemplateVars mutates shared state. Document the nil-meta contract of NewEngineFromMetamodel. Remove the dead branch in matchesFace. CompileFilter failure and bad validate checks must stay runtime fallbacks.
severity: minor
resolution: 'Plan: the trigger table is built once in the constructor and only read afterwards. The dead matchesFace branch is removed. A CompileFilter failure keeps the filter.Match verdict and an unparseable validate check keeps its runtime warning. The nil-meta contract is documented. SetTemplateVars stays as is: it is a test-only setter and out of scope.'
status: addressed
---
