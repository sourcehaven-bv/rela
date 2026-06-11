---
id: REV-HV6Q2N
type: review-checklist
title: 'Review: Hot-path benchmarks'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] Touched packages green under `-race -count=2 -shuffle=on`; lint 0 issues; gofmt clean
- [x] Full `just ci` green

## Code Review

- [x] `/code-review` run; reviewer traced all fixtures through the real resolution code and reproduced the alloc numbers exactly
- [x] Findings: 0 critical, 0 significant, 2 minors (both addressed: stale test reference + recipe comment; nit folded in), 1 leverage finding ADOPTED — alloc-ceiling contract test now runs in regular CI (RR-Z0VN2X, RR-9ZB7ZO)
- [x] Non-vacuity independently verified for all three benchmark fixtures

## Verification

- [x] First numbers on record: ValidateCreate 1.4µs (no-scan holds at 1000 entities), Verdicts 8.6µs uncached, Search 2.5ms, Lua validation ~150× when/then

**PR:** (added once created)
