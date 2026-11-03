---
id: REV-VUNG6L
type: review-checklist
title: 'Review: i18n infrastructure for the data-entry SPA (vue-i18n, en+nl, language resolution)'
started: "2026-10-27"
completed: "2026-10-27"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (frontend: 335 files, 4830 tests; rela-components: 589; Go: dataentryconfig, configedit, apiwire/v1, dataentry. The full race suite runs in CI.)
- [x] Lint clean (eslint 0 errors; golangci-lint 0 issues on the touched packages; just arch-lint OK after allowing golang.org/x/text/language for dataentryconfig; just plimsoll clean)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] ~~Coverage maintained (`just coverage-check`)~~ (N/A locally: no package lost tests and the new Go function is fully covered by language_test.go; CI enforces the floors)

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed (none raised)
- [x] All significant review-responses addressed (4 of 4)
- [x] Self-reviewed the diff for unrelated changes (Prettier churn on existing files reverted)

**Review Responses:** design review RR-LC3TBZ, RR-E2SF4K, RR-00YZ8L, RR-J7URXL,
RR-PJPIP0; code review RR-CC0RY2, RR-R6E607, RR-AIKRO1, RR-5JWQNM (significant),
RR-49HMF4, RR-O350KT, RR-T2EB34, RR-QQTRI8, RR-OY4WSG (minor), RR-MMQME2,
RR-5B4YQ1 (nits) (4 significant addressed, 5 minor addressed or won't-fix with
reason, 2 nit groups addressed or deferred with reason).

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

1. PASS. Browser nl-NL gives Dutch, en-US gives English: language.test.ts rows; manual run shows the Dutch sidebar and account menu.
2. PASS. `app.language: nl` with an en-US browser renders Dutch; choosing English on Settings switches live and survives a reload (manual, playwright); locale.test.ts and slice.test.ts.
3. PASS. `dutch!`, `de`, `nl_NL` and extension tags fail with a named app.language error (TestValidateAppLanguage, TestValidateApp_Language).
4. PASS. `<html lang>` follows the resolved locale (nl-BE, en-GB, en-US in tests and manually); formatDate gives "15 jan 2024" under nl.
5. PASS. not_found shows "Niet gevonden" under nl; an unknown code, conflict and forbidden keep the server text; English keeps the server text (errors.test.ts).
6. PASS. guards.test.ts lints a new probe file with bare strings and a locale-less Intl call through the project config; mutation check: adding a bare string to an allowlisted file fails the ratchet.
7. PASS. Mutation check: removing a key from nl.json fails parity; the unused-key check runs over comment-stripped sources.
8. PASS. localeMessages switches per call (slice.test.ts); theme toggle reads Systeem/Licht/Donker in the screenshot.

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated (GUIDE-data-entry "Interface language", regenerated docs/data-entry.md)
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-5YHTZR

## Final Checks

- [x] Commit message explains the why, not just what (minimal message per the user's rule; the ticket holds the reasoning)
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use (rules in frontend/CLAUDE.md "Interface text and languages")

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI (next step after done)
