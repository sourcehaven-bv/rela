---
id: REV-TZ0X4Y
type: review-checklist
title: 'Review: rela init generates a schema.yaml that fails to load, making new projects unusable'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`)

Run via `just ci`, which chains fmt-check, vet, lint, arch-lint, comment-lint,
the full test suite and coverage-check. `just arch-lint` reported "OK - No
warnings found"; the new test's `projectsetup → migration` import is already
permitted in `.go-arch-lint.yml`.

No new comment-lint findings: the one added doc comment carries no
`[Bracketed.Reference]` and asserts no nil contract.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-SDWGJ9 (critical), RR-XS1DMH (critical), RR-VO8OOQ
(significant), RR-DRW5WN (significant), RR-00EJVZ (significant) — all
`addressed`.

The review materially changed the scope. It established that
`metamodel.FSLoader.Load` treats migration detection as a hard gate, so the
defect was not a cosmetic notice but a total failure to load a
freshly-initialized project. I verified this independently by building the
parent commit and running `rela list` in a fresh init dir. The bug's title,
description, priority and why1 were corrected from that finding, and the fix
grew to cover the other copies of the same stale template.

Two out-of-scope defects the review surfaced are filed rather than fixed here:
TKT-6WPC86 (rela-desktop writes `entity_types`/`relation_types`, which parse to
nothing) and TKT-3WK063 (whether a missing `id_type` should remain a permanent
hard load failure).

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- PASS — a freshly initialized project loads. `rela list requirement` succeeds
post-fix; the same command on a pre-fix binary fails with "uses deprecated
syntax".
- PASS — no pending migrations. `rela migrate status` reports "data schema in
sync"; `rela validate` reports all configuration files valid.
- PASS — new projects get the intended default. `rela create requirement`
minted `REQ-CGLP`, a short ID, not a sequential one.
- PASS — regression test is non-vacuous. Stripping `id_type` from the template
fails it with `generated schema.yaml needs migration "short-id-default"`.
- PASS — the other template copies are fixed. `rela migrate --check` reports
"No migrations needed" for both prototype projects; `tickets/`, `docs-project/`,
`prototypes/perf/project` and `prototypes/worlds/project` were already clean.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: bug, not an enhancement)
- [x] User-facing documentation updated
- [x] ~~Docs-checklist marked as done~~ (N/A: bug, not an enhancement)

Although this is a bug, docs changed because they carried the same defect:
`docs/metamodel.md` taught omitting `id_type` (the exact condition the detector
refuses), and `docs/cli-reference.md` stated the migration's rename direction
backwards. Both `docs-project/` mirrors were updated to match.

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

The commit message was amended after review to lead with the real severity (the
project would not load) and to record why `short` rather than the migration's
`sequential` is correct: the pre-fix path forced new users through a migration
that silently pinned their project to legacy sequential IDs.

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI

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
