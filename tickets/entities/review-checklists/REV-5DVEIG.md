---
id: REV-5DVEIG
type: review-checklist
title: 'Review: Add a datetime metamodel property type (time-bearing, with date+time form widget)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`go test ./...` green; frontend `npm run test:run` 1271 pass / 78 files)
- [x] Lint clean (`just arch-lint` no warnings; `npm run lint` 0 errors — 89 pre-existing warnings in `stress/` + max-lines only)
- [x] Coverage maintained (`just coverage-check` PASS — total 76.7%, all package floors satisfied)

## Code Review

- [x] Run `/code-review` command (invoked cranky-code-reviewer agent)
- [x] All critical review-responses addressed (none — 0 critical found)
- [x] All significant review-responses addressed (RR-K3WEW2, RR-P9NKU7 — both fixed + regression tests)
- [x] Self-reviewed the diff for unrelated changes (only datetime-scoped source + tests + docs; build artifacts gitignored)

**Review Responses:** 7 findings, 0 critical / 2 significant / 3 minor / 2 nit —
all `addressed`:
- RR-K3WEW2 (significant) — list/table columns now honor the display-timezone override (threaded `effectiveTimezone` through `formatValue`/`formatCellValue` + `EntityList.vue`).
- RR-P9NKU7 (significant) — `formatDatetime` parses naive values via `TZDate` so view/edit agree.
- RR-B1JI4C (minor) — display-mode test asserts zone-correct text; added `formatDatetime` test block.
- RR-D6W86R (minor) — added DST spring-forward emit test + docstring note on gap/overlap normalization.
- RR-2Y3ALS (minor) — `formatDatetime` returns null (not throw) on invalid tz.
- RR-HOLGCD (nit) — documented minute-granular sub-minute drop.
- RR-1TBFJS (nit) — reworded datetime validation error to "…string or timestamp".

(Design-review round, earlier: 9 findings, 2 critical, all addressed/wont-fix —
see the ticket's has-review-response links.)

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist (IMPL-8GTRJB)

**Acceptance Status:** all PASS — evidence in IMPL-8GTRJB. Summary:
- AC1 builtin registration — PASS (`IsBuiltinType` test + CLI load).
- AC2 validation (string + time.Time + bare date + empty; junk rejected) — PASS (table test + live `analyze properties` on unquoted + bare-date files).
- AC3 filter `>`/`<` instant + strict-instant `=` — PASS (match_test + live CLI `--where`).
- AC4 sort chronological incl. mixed date/datetime — PASS (sort_test + live CLI `--sort`).
- AC5 OpenAPI `date-time` — PASS.
- AC6 type→widget end-to-end incl. FieldRenderer dispatch — PASS.
- AC7 widget round-trip + non-destructive — PASS (widgets.test; mount emits nothing).
- AC8 zone-correct display — PASS (now asserts text, not existence).
- AC9 tz override persist/fallback + honored in lists AND form — PASS (ui.test + format.test + widget test).
- AC10 DST/offset/date-line — PASS (Kolkata +05:30, Auckland date-line, spring-forward boundary).

## Documentation (enhancements only)

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: docs updated inline in this ticket — `docs/metamodel.md` Datetime Properties section + `docs/data-entry.md` Datetime fields and time zones section; no separate docs-checklist warranted for a two-file doc update)
- [x] User-facing documentation updated (metamodel.md + data-entry.md)
- [x] ~~Docs-checklist marked as done~~ (N/A per above)

## Final Checks

- [x] Commit message explains the why, not just what (two commits: feature + review-fixes, each with rationale)
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
- [x] All CI checks pass (E2E toast-locator collision fixed; the only remaining "Rela Tickets" failure is the self-referential gate that clears when this ticket → done)
- [x] PR URL documented below

**PR:** https://github.com/sourcehaven-bv/rela/pull/1131
