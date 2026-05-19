---
id: REV-L5Z1
type: review-checklist
title: 'Review: data-entry: per-request Principal from HTTP header'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint` + `just lint-md` + `just arch-lint`)
- [x] Coverage maintained (`just coverage-check`) — total 76.9% (threshold 65%)

## Code Review

- [x] Ran cranky-code-reviewer agent
- [x] All critical review-responses addressed (RR-7LMA, RR-CDXR)
- [x] All significant review-responses addressed (RR-WBQC, RR-94DJ, RR-7C87, RR-I5OG, RR-H10L, RR-KJMU) — except 2 explicitly deferred with documented reason (RR-DTL4, RR-CX6M)
- [x] Self-reviewed the diff for unrelated changes — only docs comment fix in audit/filesystem.go is cross-package, justified as cleanup of the same bug pattern (`r >= 0` dead disjunct + stale non-breaking-space comment)

**Review Responses:**

| ID | Severity | Status | Title |
|---|---|---|---|
| RR-7LMA | critical | addressed | Control-only header value bypasses fall-through |
| RR-CDXR | critical | addressed | Dead disjunct in isControlRune |
| RR-WBQC | significant | addressed | ChainResolvers ignores Tool, only checks User |
| RR-94DJ | significant | addressed | No startup warning on non-loopback bind + --principal-header |
| RR-7C87 | significant | addressed | Two-pass sanitize loop is gratuitously complex |
| RR-I5OG | significant | addressed | truncateRunes allocates full []rune(s) for a prefix |
| RR-H10L | significant | addressed | Test had dead _ = got + inconsistent header-set paths |
| RR-KJMU | significant | addressed | TestHeaderPrincipalResolver_ToolUnchanged missed chain fallback |
| RR-DTL4 | significant | **deferred** | Promote sanitization helpers to internal/principal |
| RR-CX6M | significant | **deferred** | Refuse startup on bad combinations (non-loopback + header) |
| RR-9V36 | minor | addressed | Stale doc claim of non-breaking space in audit.Filesystem |
| RR-CHJL | minor | addressed | EnvPrincipalResolver re-reads $RELA_DATAENTRY_USER per request |
| RR-2T24 | minor | addressed | HeaderPrincipalResolver("") returns closure instead of nil |
| RR-9KH2 | minor | **wont-fix** | TestHeaderPrincipalResolver_WeirdHeaderName proves the wrong thing |

## Acceptance Verification

- [x] Each acceptance criterion tested (named tests in `internal/dataentry/principal_test.go`)
- [x] Test evidence documented in IMPL-WYXW

**Acceptance Status:**

| AC | Status | Evidence |
|---|---|---|
| AC1 | PASS | `TestHeaderPrincipalResolver_PopulatesUser` |
| AC2 | PASS | `TestHeaderPrincipalResolver_AbsentHeaderFallsThrough` |
| AC3 | PASS | `TestHeaderPrincipalResolver_EmptyHeaderFallsThrough` (3 subtests) |
| AC4 | PASS | `TestHeaderPrincipalResolver_Sanitizes` (5 subtests incl. control-only regression for RR-7LMA) |
| AC5 | PASS | `TestChainResolvers_EnvWinsOverHeader` (4 subtests) |
| AC6 | PASS | `TestHeaderPrincipalResolver_EmptyNameDisabled` |
| AC7 | PASS | `TestHeaderPrincipalResolver_ToolUnchanged` (4 subtests incl. chain-fallback per RR-KJMU) |

Plus negative test `TestHeaderPrincipalResolver_WeirdHeaderName` (panic guard on
invalid header name).

## Documentation (enhancements only)

- [x] User-facing documentation updated:
  - `docs-project/entities/guides/GUIDE-audit-log.md` — replaced "user is unknown" prose with attribution chain + trust boundary
  - `docs/security.md` — added "data-entry user attribution" subsection
- [x] Generated docs regenerated via `just docs`

(No DOCS-checklist created — the enhancement's docs updates ride along in the
same commit; a separate docs-checklist would be process overhead for ~30 LOC of
guide changes.)

## Final Checks

- [x] Commit messages explain the why (two commits: initial implementation + cranky-review fixes, each with rationale)
- [x] No TODOs or FIXMEs left unaddressed in the new code
- [x] Ready for another developer to use — the `--principal-header` flag is opt-in; default behavior unchanged

## Pull Request

- [ ] Run `/pr` to create PR and monitor CI
- [ ] All CI checks pass
- [ ] PR URL documented below

**PR:** <!-- pending /pr -->

## Follow-up tickets (post-merge)

- **Promote sanitization to `internal/principal`** (RR-DTL4) — collapses the dataentry / audit duplication into a single `principal.SanitizeField(s, limit)`. Separate refactor; touches audit's sanitization contract.
- **Refuse-startup ergonomics** (RR-CX6M) — revisit if operator confusion shows up in practice.
- **`TestHeaderPrincipalResolver_WeirdHeaderName` rename** (RR-9KH2) — cosmetic, file as low-priority cleanup.
