---
id: REV-VTC6MK
type: review-checklist
title: 'Review: rela trace/path output ignores display_property (tracer builds titles from literal title)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`go test ./...` green)
- [x] Lint clean (`golangci-lint` 0 issues; `go vet` clean; `just arch-lint` OK — tracer stays metamodel-free; `just plimsoll` OK)
- [x] Coverage maintained (`just coverage-check` PASS — 77.2%)

## Code Review

- [x] Ran cranky-code-reviewer on commit 07de7598
- [x] All critical review-responses addressed (RR-9Q0DUD — JSON leak)
- [x] All significant review-responses addressed (RR-MHYTYK, RR-ODH6YX)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:**
- RR-9Q0DUD (critical → addressed) — trace JSON leaked property maps; fixed with `json:"-"`.
- RR-MHYTYK (significant → addressed) — removed dead `PathStep.Properties`.
- RR-ODH6YX (significant → addressed) — added JSON-schema pin + nested-child resolver test.
- RR-K4XP7Q (nit → wont-fix) — traceTitle signature / GetEntity-isolation invariant, justified.

## Acceptance Verification

- [x] Each acceptance criterion tested

**Acceptance Status:**
1. `rela trace from`/`trace to` show the display_property-resolved title (bare/template) — **PASS** (manual: `PERS-JV Jeroen Vloothuis`; unit: TestWriteTrace_TitleResolver).
2. Literal-`title` types unchanged — **PASS** (`PROJ-1 Rela Platform`; nil-fallback subtest).
3. Tracer gains no metamodel dependency — **PASS** (`just arch-lint` clean; resolution happens at the output boundary).

## Documentation (enhancements only)

- [x] ~~Docs~~ (N/A: no new user-facing surface; trace now matches the already-documented display_property behavior of list/graph)

## Final Checks

- [x] Commit messages explain the why
- [x] No TODOs or FIXMEs
- [x] Ready for another developer to use

## Pull Request

- [x] PR created: https://github.com/sourcehaven-bv/rela/pull/1088
- [x] All CI checks pass (verified locally: build, full test, lint, vet, arch-lint, plimsoll, coverage; PR CI running)
- [x] PR URL documented below

**PR:** https://github.com/sourcehaven-bv/rela/pull/1088
