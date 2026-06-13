---
id: PLAN-G9GIMW
type: planning-checklist
title: 'Planning: Test hygiene batch'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem understood — four review residues, all itemized in TKT-LR5HLB
- [x] Scope defined; acceptance criteria: each weak test pins real behavior (probed first, not guessed), git suite skips cleanly without the binary, timeout handlers return on client cancellation

## Research

- [x] ~~/research~~ (N/A: xs hygiene)
- [x] Probed before pinning: cascade write counts (3/1 via countingStore), unclosed-frontmatter outcome (deterministic error)

## Approach

- [x] Documented in ticket; alternatives: deleting the cascade test rejected — the invariant is real, the assertions were just missing

## Security Considerations

- [x] N/A (test-only)

## Test Plan

- [x] Five packages under `-race -count=2 -shuffle=on`; full `just ci` before PR

## Risk Assessment

- [x] Effort xs. Risk: pinned write-counts couple to the upsert pipeline shape — accepted; that coupling is the point (same pattern as the existing no-scan pin) and the comment explains the breakdown

## Documentation Planning

- [x] N/A

## Design Review

- [x] ~~/design-review~~ (N/A-with-substitute: items and approach enumerated and approved in session reviews)
