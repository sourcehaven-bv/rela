---
id: IMPL-SE1K0
type: implementation-checklist
title: 'Implementation: Restructure rela.md AST: preserve inline structure (text → inlines)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end (parse→render fixed-point)
- [x] Each acceptance criterion verified with test scenario
- [x] Edge cases manually verified

**Verification Evidence:**

- All 20 ACs covered: AC1 (no `text` on block nodes), AC2 (blockquote
children), AC3 (multi-block list-item children), AC4 (no phantom task checkbox
inline), AC5–AC8 (link/image/raw HTML/autolink/breaks round-trip), AC9
(constructor auto-wrap), AC10 (inline constructors), AC11/AC12 (flatten +
headers/first_paragraph), AC13 (corpus property), AC14 (existing fixtures
byte-equivalent for non-link content), AC18 (benchmark documented).
- AC18 benchmark result: post-refactor 390k ns/op, 1153k B/op, 3882
allocs/op vs baseline 159k ns/op, 319k B/op, 1368 allocs/op — that's ~2.5–3× on
the kitchen-sink fixture. Documented as a known consequence in the new docs
section. Optimization (table pooling, lazy materialization) deferred to a
follow-up.
- `just test` (race-enabled): all packages pass.
- `just lint` (golangci-lint v2.11.4): 0 issues.
- `just coverage-check`: thresholds satisfied.
- `just docs`: regenerated, no diffs after commit.
- `just arch-lint`: no warnings.

## Quality

- [x] Code follows project patterns
- [x] No security issues introduced
- [x] No silent failures
- [x] No debug code left behind
