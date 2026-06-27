---
id: IMPL-QRAIMK
type: implementation-checklist
title: 'Implementation: Fixture consolidation: mcp on appbuildtest, validation metamodel dedup, testutil fixes'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (WithValidation builder self-test incl. append ordering)
- [x] Integration tests written — existing mcp/validation suites are the coverage; they run on the consolidated fixtures
- [x] Happy path implemented (all three parts of TKT-R2KBG6)
- [x] Edge cases from planning handled (Config loader delta verified inert — loadDataEntryConfig swallows both error shapes; templater miss path verified equivalent)
- [x] Error handling in place

## Test Quality

- [x] Using fixture builders or factories (the point of the ticket)
- [x] No hardcoded values in assertions
- [x] Only specifying values that matter (ticketMeta keeps rules at call sites)
- [x] Interpolated values constructed from objects
- [x] Property comparisons use original object

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified
- [x] Edge cases manually verified

**Verification Evidence:**

- mcp: test_helpers_test.go 139 → ~48 lines; the whole hand-rolled service graph (incl. the nopTemplater workaround and nopConfigLoader) replaced by appbuildtest accessors. mcp green under `-race -count=2 -shuffle=on`; dispatch tests (TKT-TLQ94B) still build via NewServer.
- validation: 16 of 17 inline metamodel literals migrated to ticketMeta (script with per-site safety checks: single ticket type, string-only props, no extra metamodel fields; 1 site skipped — no Validations section). Reviewer diffed migrated sites against the previous revision: semantically identical.
- testutil: AssertEqual/AssertNotEqual → reflect.DeepEqual (reviewer verified all 26 existing callers compare scalars where == and DeepEqual agree — no behavior flips); AssertStringContains/NotContains → strings.Contains (empty-string edge cases verified equivalent).
- Code review: 0 critical, 0 significant, 2 nits (both fixed: ProjectRoot-split comment, ordering test). Reviewer independently verified no semantic drift on all four flagged axes (search backfill, templater miss path, config loader, autocascade gating) and noted backfill failures are now intentionally louder (panic on bad seed data).
- Full `just ci` green.

## Quality

- [x] Code follows project patterns (appbuildtest is the established fixture seam)
- [x] DRY — that's the ticket
- [x] No security issues introduced
- [x] No silent failures (backfill strictness improved)
- [x] No debug code left behind
