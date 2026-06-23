---
id: PLAN-T5BPYR
type: planning-checklist
title: 'Planning: Weekly fuzz sweep'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined — see TKT-PCLGGL (script + weekly workflow + issue automation + just recipe; per-PR job untouched)
- [x] Acceptance criteria: discovery-based (no hand-list), every target gets fuzz time, failures upload artifacts AND auto-file/append a deduped GitHub issue with reproduction instructions

## Research

- [x] ~~/research~~ (N/A: CI plumbing)
- [x] Checked patterns: security.yml weekly cron ("0 9 * * 1") — fuzz scheduled before it; govulncheck-filtered.sh is the existing scripts/ precedent
- [x] Verified discovery regex excludes the storetest shared fuzz helpers (they take extra params beyond *testing.F, so they're not targets; the per-backend wrappers are)

## Approach

- [x] grep-based discovery (file scan, no per-package compile); `go test -run='^$' -fuzz='^Name$' -fuzztime=$FUZZTIME` per target; continue-on-failure with summary file
- [x] Issue dedupe: comment on existing open `fuzz-failure`-labeled issue, else create (label created idempotently)
- [x] Alternatives: `go test -list` discovery rejected (compiles every package twice); auto-committing regression corpus rejected (write-permission + review concerns — artifacts + issue instead)

## Security Considerations

- [x] Workflow permissions minimal: contents read + issues write; no untrusted input interpolated into run blocks (summary file is repo-generated)

## Test Plan

- [x] Local run with short FUZZTIME across all targets; deliberate-failure injection to verify summary + exit code

## Risk Assessment

- [x] Effort s. Risks: long-tail wall time (39 × 25s + compiles ≈ 25-30 min — under the 60 min job timeout); pgstore targets silently skipping (documented, follow-up noted)

## Documentation Planning

- [x] N/A (CI-internal; script self-documents)

## Design Review

- [x] ~~/design-review~~ (N/A-with-substitute: approach incl. issue automation requested and shaped by reviewer in session 2026-06-10)
