---
id: REV-AVF3O3
type: review-checklist
title: 'Review: World chrome speaks the operator''s words or stays silent: messages, on_absent redirect, copy on_success'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`)

**Comment findings.** `just comment-report` on the touched files reports only
pre-existing findings (restatement on `DefaultMetamodelYAML`/`ListResponse`,
nil-contract prose on three older `affordances.go` helpers, and the
`CopyDef`/`planCopy` duplication that predates this ticket). Nothing introduced
by this diff.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** cranky-code-reviewer and rela-security-reviewer ran in
parallel on `git diff fix/worlds-face-address...HEAD`. Security found no XSS
(every operator string lands in a mustache interpolation; `{title}` never
reaches a `v-html` sink), no open redirect (`landing`/`redirect` write a query
param or a load-validated face), no disclosure (the new wire fields are
`schema.yaml` text) and no ACL change; its one finding was the redirect cycle,
merged into RR-5TDYWW at the higher severity. All fourteen addressed in commit
d730bd5a: RR-51XOG3 (critical: `landing:` unknown keys silently dropped),
RR-5TDYWW (critical: two-world redirect cycle), RR-SKDOFD, RR-Q8SWR6, RR-XTHMDD,
RR-4F65WN, RR-3Y3UR6 (significant), RR-S1QJT8, RR-7W31ZO, RR-BENZ9Z, RR-3UR6T2,
RR-VJYG4V, RR-D00YBI (minor), RR-LMOCE6 (nit).

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
- No rela-authored chrome sentence on any surface: PASS. Detail note, absent banner and its "Go to" button, list/board projection notes, form refusal, stand-in badge text and tooltip, "Created the X face" toast all removed; SPA suites assert silence with nothing declared (EntityDetail.world, EntityList.world, KanbanView.world, WorldBadge, RelationPicker).
- Operator text per world/face/copy with placeholder substitution: PASS. Go: field-coverage tests, `schemamessages_test.go`, `TestCopyOnSuccessWire`; SPA: `worldText` unit tests plus the per-surface suites; Chrome walk on the atlas verify project showed the Dutch read-only note, the projection note, the "Concept" badge, the form refusal and the toast "Thuiswerkbeleid is vastgesteld." landing on `POLICY-002@vastgesteld`.
- `on_absent.redirect` and `on_success.landing` validated at load: PASS. `TestWorlds_OnAbsentRedirectValidated` (undeclared, self-loop, two-world cycle, legitimate chain), `TestValidateCopies_Landing`, `TestCopyLanding_UnmarshalRefusesWhatItCannotMean`.
- Placeholder allowlist pinned across Go and TS: PASS. `TestChromePlaceholdersInSyncWithFrontend`.
- Atlas verify manual still passes against the extended fixture (19 claims).

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-NT4GX0

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (N/A: PR opened and watched directly per the standing instruction: 🌍 title, auto-merge, reviewer tschmits)

<!--
Deliberately NOT tracked here: the PR URL and whether CI passed.

Both post-date this checklist. `/pr` requires the ticket to be `done` and
validating clean before it opens the PR, and a `done` review-checklist may have
no unchecked items — so an item asking for the PR URL can only be satisfied by a
PR that does not exist yet. Checking it early would mean asserting "CI passed"
before CI ran, which turns the checklist from evidence into a formality.

GitHub records both authoritatively, and the branch and commit messages carry
the ticket ID, so the ticket-to-PR link is recoverable without duplicating it
here. See TKT-UFV01M. -->
