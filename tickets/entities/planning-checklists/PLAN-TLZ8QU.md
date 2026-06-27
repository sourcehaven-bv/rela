---
id: PLAN-TLZ8QU
type: planning-checklist
title: 'Planning: Default package coverage floor'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood — `threshold.package: 0` defeats the file's own stated purpose
- [x] Scope defined — config-only change to .testcoverage.yml
- [x] Acceptance criteria: default package floor 50; no currently-passing package starts failing (overrides ~5pp below current per the file's convention); helper packages excluded; a scratch untested package fails the check (negative verification)

## Research

- [x] ~~/research~~ (N/A: config change)
- [x] Measured per-package coverage on the branch before setting numbers

## Approach

- [x] Default 50 + explicit lower overrides + exclusions; alternatives (ratchet, per-file floors) rejected — the repo explicitly documents floors-not-ratchet in CLAUDE.md

## Security Considerations

- [x] N/A

## Test Plan

- [x] `just coverage-check` green; scratch-package negative check

## Risk Assessment

- [x] Effort xs. Risk: a package hovering just above its floor flakes on coverage noise — mitigated by the 5pp headroom convention

## Documentation Planning

- [x] N/A (.testcoverage.yml is self-documenting; CLAUDE.md's coverage section already describes floors)

## Design Review

- [x] ~~/design-review~~ (N/A-with-substitute: approach agreed in session 2026-06-10)
