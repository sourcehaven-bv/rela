---
id: REV-M31EGQ
type: review-checklist
title: 'Review: Review checklist must not track PR URL or CI status — they deadlock the done-before-PR gate'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] ~~All tests pass (`just test`)~~ (N/A: no code changed — a markdown
template and a docs section)
- [x] ~~Lint clean (`just lint`)~~ (N/A: no Go changed)
- [x] Comment lint gate clean (`just comment-lint`) — N/A for the same reason,
but unaffected; no comments touched
- [x] ~~Coverage maintained (`just coverage-check`)~~ (N/A: no code)

Ran instead the check that actually covers this change: `rela validate --project
tickets --check cardinality --check properties --check validations` → **All
validations passed**, over 243 review-checklists including the 171 carrying the
old PR section.

`just lint-md` also passes (the template and `CLAUDE.md` are markdown).

## Code Review

- [x] Run `/code-review` — reviewed the diff (`git diff`), two files
- [x] All critical review-responses addressed — none raised
- [x] All significant review-responses addressed — none raised
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** none

Reviewed for the failure modes that matter on a workflow change:

- **Does it weaken enforcement?** No. `done-review-checklist-no-unchecked` is
untouched; the remaining "Run `/pr`" item is a normal unchecked box that still
fails a `done` checklist.
- **Does it break existing entities?** No. The rule reads entity content, not
the template; validation passes over all 243.
- **Is anything lost?** The Complete step's "PR merged or ready to merge" line
went with the reorder. Checked: nothing in `schema.yaml` or `.claude/` depends
on it, and under the corrected order it was self-contradictory — a ticket cannot
require a merged PR at the moment it becomes `done`, because `/pr` has not run
yet. Removing it resolves a contradiction rather than dropping a requirement.
- **Will it regress?** The template carries an inline comment explaining why
the items are absent and citing this ticket, so the omission does not read as an
oversight.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

1. **New checklist reaches `done` with no PR** — **PASS**. This checklist is
the proof: generated from the updated template, it contains one PR item instead
of three, and is being closed with no PR in existence. On TKT-MTWQ4G the
equivalent step failed with `REV-7ILS94: Done review checklists cannot have
unchecked items`.
2. **Existing checklists keep validating** — **PASS** (all validations passed).
3. **`CLAUDE.md` order matches enforcement** — **PASS** (Complete → Create PR).

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated — N/A for `docs/`; contributor
workflow only, justified in the docs-checklist
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-A239JC

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use — the next ticket through the
workflow gets the corrected template automatically

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
