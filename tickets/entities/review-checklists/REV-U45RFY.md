---
id: REV-U45RFY
type: review-checklist
title: 'Review: rela migrate strips form labels the SPA cannot re-derive, permanently downgrading them to raw property/relation ids'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Coverage maintained (`just coverage-check`)

Full Go suite passes. `just lint` reports **0 issues**. `just arch-lint` OK.
`just plimsoll` (god-object lint) OK. Coverage **77.2%** — package floor (50%)
and total floor (65%) both satisfied. Frontend: `vue-tsc` clean, eslint **0
errors**, **1448 tests pass** across 88 files.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed — none were raised
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-U2LC1P (significant, addressed), RR-QZWI1I
(significant, addressed), RR-VQHQSG (minor, addressed), RR-GW9711 (minor,
addressed), RR-HNKO3R (minor, deferred with reason).

No critical findings. The reviewer independently mutation-tested the kept
relation-label strip and confirmed the guarding test is discriminating rather
than vacuous.

The two significant findings were both real and both empirically demonstrated,
by the reviewer and re-confirmed by me before fixing:

1. **Docs edited on the generated side** — `docs/data-entry.md` and
`docs/lua-scripting.md` are generated from `docs-project/entities/guides/`. The
next `scripts/generate-docs.sh` run would have reverted them to promising a
derivation the code no longer performs. Sources updated; regeneration now
reproduces the intended output.

2. **The structural lint test was evadable** — it banned four helper *names*,
so an arrow function passed, and so did restoring `HistoryView.propertyLabel`,
one of the derivations this very change deleted. Rewritten to match the *shape*
of the transform. The rewritten guard immediately caught two live sites the
original missed, including `cli/graph.go` (RR-VQHQSG) — which is precisely the
value the guard was supposed to provide.

Self-review: the diff touches `internal/cli/graph.go`, which was not in the
original scope. It is in scope for the decision (it derived a display label from
a type id) and was surfaced by the guard, not by speculation.
`frontend/src/components/forms/DynamicForm.vue` is deliberately NOT touched — it
belongs to separate in-flight work, so its one derivation is allowlisted with a
stated reason and left as follow-up rather than silently widening this diff.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- **The server must start with a title-cased label present** — PASS.
`Detect() == false` for `label: 'Titel'` on property `titel`; pinned by
`TestLabelsAreNeverStripped` and `TestListColumnLabelsAreNeverStripped`.
- **The label must survive `rela migrate`** — PASS. Verified against the bug
report's exact three Dutch labels; all three present in the post-Apply YAML.
- **An unlabelled field renders the raw identifier, not a guess** — PASS.
`FieldRenderer.test.ts` (three cases, including kebab-case),
`TestBuildSectionEntityData_LabelIsAuthoredNeverDerived`.
- **The metamodel relation label becomes live config for forms** — PASS.
`RelationPicker.test.ts` — metamodel label used when the form label was
stripped, raw id when neither exists, explicit label wins over both.
- **No derivation survives anywhere** — PASS, enforced structurally by
`TestNoLabelDerivation`, whose ability to catch a reintroduction was verified
with three planted probes (arrow function, restored `propertyLabel`, renamed Go
helper) plus a clean baseline.

## Documentation (enhancements only)

Skipped per the section's own instruction ("Skip this section for bugs and
internal refactors") — no `docs-checklist` entity created. Documentation was
nonetheless updated as part of the fix, because the existing docs actively
documented the removed behaviour:

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: bug, not an enhancement)
- [x] User-facing documentation updated — `GUIDE-data-entry.md`,
`GUIDE-lua-scripting.md`, `GUIDE-metamodel.md` (sources), regenerated into
`docs/`. Added an explicit "Labels are authored, never derived" callout.
- [x] ~~Docs-checklist marked as done~~ (N/A: none created)

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

Not yet committed — awaiting user go-ahead per project convention.

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (Deferred: nothing
      committed yet — the user reviews the diff before any commit or push.)
- [x] ~~All CI checks pass~~ (Deferred: no PR yet. Every check CI runs was run
      locally and passes — `just lint` 0 issues, `just arch-lint`, `just
      plimsoll`, full `go test ./...`, `just coverage-check` 77.2%, frontend
      typecheck + 1448 tests + eslint 0 errors.)
- [x] ~~PR URL documented below~~ (Deferred: no PR yet.)

**PR:** not yet created — awaiting user go-ahead to commit.

## Known accepted regression

Per DEC-6C1NAA the user explicitly accepted that unlabelled fields now render
raw identifiers. In `tickets/data-entry.yaml` that is 89/94 form fields, 41/41
list columns, and 47/47 view-section fields. Mostly single words losing a
capital (`status`, `effort`); the visible cases are multi-word (`due_date`,
`estimated_hours`). The reviewer independently confirmed the blast radius is
cosmetic only — no label is used as a lookup key, sort key, export header, or
e2e selector. One incidental improvement: `SidePanel.vue:131` used
`field.label.toLowerCase()` as a Badge property lookup, which produced a broken
`"due date"`; it now receives the correct raw `due_date`.
