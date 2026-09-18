---
id: REV-95JNE3
type: review-checklist
title: 'Review: Cut CI wall clock: de-serialize the build tail and fix Go cache thrash'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) — via CI run 35272630661, all jobs green
- [x] Lint clean (`just lint`) — golangci-lint green in CI; `actionlint` on the
workflow reports 20 findings on both this branch and the base commit, i.e.
parity, all pre-existing
- [x] Comment lint gate clean (`just comment-lint`) — green in CI
- [x] Coverage maintained (`just coverage-check`) — green in CI; no Go source
changed, so coverage is unaffected by construction

**Comment findings.** No Go comments were added or changed; the diff is workflow
YAML and one TypeScript config comment, neither of which commentlint analyses.
No new findings introduced.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-S45X2X (critical, addressed), RR-AUWXN2 (critical,
addressed), RR-Q60O40 (significant, addressed), RR-D4DAH7 (significant,
addressed), RR-FB2FPJ (minor, addressed), RR-M292PZ (minor, addressed),
RR-SQ1AJR (minor, deferred with reason).

The review ran against commit aecc89c5. RR-AUWXN2 (the cache-quota defect) had
already been found and fixed independently in f9a4c52b before the review landed;
the remaining six were fixed in 549471d6.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- Reduce CI wall clock — **PASS**. Run 35337622282, 23/23 jobs green, 683s;
best green run 530s. Develop the same week: 865s, 934s, 976s (mean 925s). So
26-43% faster like for like. Warm caches show clearly per job: Lint 258->142s,
Demos 256->121s, SQLite 219->101s, Frontend 208->140s.
- No coverage or check loss — **PASS**. 18 required checks still report; only
`Build` was removed, and it compiled a strict subset of Cross-Compile
(linux/default). The reviewer independently verified no job depended on it: all
four `bin/rela` consumers build it themselves.
- Merge queue not broken — **PASS**. The `Build` required check was removed
from the develop ruleset in the same change, 19 -> 18. `Rela Tickets` also
passes now that this ticket exists.

An E2E failure during this work was misattributed twice before being traced to
the branch being 7 commits behind develop; a rebase resolved it. Recorded
because the wrong cause (Playwright worker count) had a plausible mechanism and
a three-run correlation behind it, and was still wrong.

## Documentation (enhancements only)

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: CI-internal
change, no user-facing surface)
- [x] ~~User-facing documentation updated~~ (N/A: no user-facing behaviour
changed; the reasoning lives in workflow comments where the next editor of that
file will meet it)
- [x] ~~Docs-checklist marked as done~~ (N/A: none created)

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

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
