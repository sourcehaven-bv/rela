---
id: REV-HFO4MT
type: review-checklist
title: 'Review: Sync 1/5: shared canonical entity/relation serializer + content hash'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass — `go test -race ./internal/canonical/ ./internal/store/fsstore/ ./internal/markdown/` → ok (incl. 966k-exec cross-backend fuzz + 246k body-convergence fuzz); CI Test + Fuzz green on PR #1006
- [x] Lint clean — `golangci-lint run` on all three touched packages → 0 issues; CI Lint + Architecture green
- [x] Coverage maintained — canonical package 96% (well above default floor 50); fsstore/markdown unchanged or improved

## Code Review

- [x] Run `/code-review` command (cranky-code-reviewer) — done; found 5 critical + 4 significant
- [x] All critical review-responses addressed — RR-3A4I1Z, RR-KTAK7N, RR-QUXNPR, RR-N7D3OK, RR-FQM4NQ
- [x] All significant review-responses addressed — RR-G92SKT, RR-URBR6S, RR-K484BN, RR-H5A0MZ
- [x] Self-reviewed the diff for unrelated changes — fsstore formatter dedup is the only adjacent change; behavior-preserving (all store/entitymanager tests pass). archfile updated to register the new component.

**Review Responses:** RR-3A4I1Z RR-KTAK7N RR-QUXNPR RR-N7D3OK RR-FQM4NQ
(critical, addressed); RR-G92SKT RR-URBR6S RR-K484BN RR-H5A0MZ (significant,
addressed). All 9 verified-and-fixed; 2 of the bugs were found by the new
fuzzers, not the original tests.

## Acceptance Verification

- [x] Each acceptance criterion tested
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
- AC "same logical entity via fsstore and pgstore → identical hash": **PASS** — `TestHashEntity_CrossBackendDecode` (raw YAML decode vs JSON+UseNumber) + `FuzzCrossBackendDecode` (966k execs clean), covering scalars, whole/fractional floats, dates, datetimes, lists, nested + non-string-keyed maps, unicode, control chars, large ints.
- AC "relations hashed by logical content": **PASS** — `TestHashRelation_*` (determinism, type-invariance, direction).
- AC "`fsstore/echo.go:46 hashContent` untouched": **PASS** — not modified (verified via git diff).

## Documentation (enhancements only)

- [x] ~~Docs-checklist created and linked via has-docs~~ (N/A: internal infrastructure package; no user-facing surface yet)
- [x] ~~User-facing documentation updated~~ (N/A: `internal/canonical` is not user-visible; the sync CLI/docs land in TKT-T4H4YK, where docs are scoped)
- [x] ~~Docs-checklist marked as done~~ (N/A: see above)

**Docs Checklist:** N/A — the package doc comment (canonical.go) documents the
invariant and rationale for maintainers; end-user docs belong to the CLI
sub-ticket.

## Final Checks

- [x] Commit message explains the why, not just what — the fix commit enumerates each finding and the reasoning
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use — public API `HashEntity`/`HashRelation`, documented; the cross-backend invariant + the fixed-point/length-prefix rationale are in the package doc

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI — PR opened, auto-merge (squash) enabled, reviewer `tschmits` requested
- [x] All CI checks pass — Test, Fuzz, Lint, Architecture, Postgres Backend, E2E, Frontend all green on PR #1006
- [x] PR URL documented below

**PR:** https://github.com/sourcehaven-bv/rela/pull/1006 — auto-merge enabled
(squash), reviewer @tschmits requested, merges on green CI + approval.
