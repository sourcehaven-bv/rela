---
id: PLAN-XO6G10
type: planning-checklist
title: 'Planning: Hot-path benchmarks'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem understood — performance contracts documented in prose, no numbers (2 benchmarks repo-wide)
- [x] Scope — 4 bench files + just bench recipe; explicitly NO CI regression tracking
- [x] Acceptance criteria: each benchmark exercises the real documented hot path (not a mock), reuses package fixtures, runs via just bench

## Research

- [x] ~~/research~~ (N/A)
- [x] Verified API surfaces before writing: ValidateCreate, PolicyResolver.FieldVerdicts/RelationVerdicts (UC10 fixture shape with real Declarative + member-of walk), search.New + Query, validation.Service.Check
- [x] Existing benchmarks reviewed (lua BenchmarkMdParse, pgstore BenchmarkGraphQuery) for conventions

## Approach

- [x] b.Loop() (modern), b.ReportAllocs, realistic seeds (1000-entity store/index; 100-entity validation batch; 200-ticket graph with member-of role walk)
- [x] Alternatives: benchstat CI tracking rejected (own project); microbenchmarks of internals rejected (measure the documented contracts, not implementation details)

## Security Considerations

- [x] N/A (test-only)

## Test Plan

- [x] Smoke-run all benchmarks (-benchtime=10x); full test suites of touched packages still green

## Risk Assessment

- [x] Effort s. Risk: benchmarks rot if unused — mitigated by just bench discoverability and contract-anchored doc comments

## Documentation Planning

- [x] N/A

## Design Review

- [x] ~~/design-review~~ (N/A-with-substitute: scope agreed in session 2026-06-10; CI-tracking explicitly excluded by plan)
