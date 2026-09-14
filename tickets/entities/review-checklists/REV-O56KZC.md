---
id: REV-O56KZC
type: review-checklist
title: 'Review: The release workflow''s gate jobs cannot build rela-desktop'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`)

Delegated to CI rather than run locally, because the diff contains no Go, Vue or
markdown — it is two `apt-get install` lines in a workflow file. Running the
full local suite would exercise nothing the change touches.

CI on PR #1562 covers this: Test, Lint, Architecture, God-object lint, Comment
lint, Lint Markdown, Frontend, Fuzz, E2E, Postgres Backend, SQLite Backend,
CodeQL and all five Cross-Compile targets. All green on head `2f63f785`.

The one check that failed was **Rela Tickets**, which is what this bug entity
exists to satisfy — the gate requires a work-item entity on a non-`chore/*`
branch.

## Code Review

- [x] ~~Run `/code-review` command~~ (N/A: two `apt-get install` lines in a
workflow; the cranky-code-reviewer agent reviews source, and there is none)
- [x] All critical review-responses addressed (none raised)
- [x] All significant review-responses addressed (none raised)
- [x] Self-reviewed the diff for unrelated changes

The self-review caught a real problem worth recording. The first push of this
branch contained **159 files**, not one: the branch was cut from a local
`develop` that was four commits behind the remote, so it carried stale copies of
the worlds/BUG-HC6I2T work that had already merged via #1557–#1560. GitHub
reported `CONFLICTING`. The branch was rebuilt by resetting onto
`origin/develop` and cherry-picking the single workflow commit, which applied
cleanly — confirming the intended change was independent of everything that had
landed. The PR is now one commit touching one file.

That is exactly the "unrelated changes" this checklist item is for, and it was
found by reading the file list rather than trusting the commit count.

## Verification of the fix

- [x] Root cause identified and confirmed, not inferred
- [x] Fix addresses the cause, not the symptom
- [x] Regression risk assessed

The cause was read from the failing job logs, not deduced: `Package 'gtk4' not
found` → `[build failed]` for `cmd/rela-desktop` → Test job red → Release job
skipped via `needs:`. The same signature was confirmed on all three affected
tags.

The blame is likewise confirmed rather than assumed. `git log` shows PR #1546
(the Wails v3 migration) edited `ci.yml`, `security.yml` **and** `release.yml`,
adding the GTK packages to the first two only. That is why develop has been
green throughout while every tag since has failed.

**Regression risk: low.** Installing two additional apt packages cannot fail a
job that already succeeds; the same two packages are already installed by
`ci.yml` and `security.yml` on the same runner images. The failure mode if the
package names were wrong would be an immediate, loud `apt-get` error, not a
silent one.

**Honest limit:** this PR's own CI does not exercise `release.yml`. The change
is verified by construction (it mirrors two working workflows) and by the
diagnosis, but the proof is the next tag push. Stated here so a green PR is not
mistaken for a proven release path.

## Acceptance Verification

- [x] Each acceptance criterion tested
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

1. The release workflow's Test job can build `cmd/rela-desktop` — **fix
applied, proof deferred to the next tag**. The install step now matches
`ci.yml`, whose Test job builds the package successfully on this commit.
2. The Security job's govulncheck can load the cgo packages — **fix applied,
proof deferred**. Mirrors `security.yml`, which scans successfully.
3. The three stranded tags can be republished — **not done in this PR**.
v26.9.2, v26.9.3 and v26.9.4 remain tags without releases; republishing them is
an operator action after this merges, and the route (Release workflow's
`workflow_dispatch` escape hatch, or a fresh tag) is noted on the bug.
4. The recurrence is recorded, not just fixed — **PASS**.
`release-gates-match-ci-build-prerequisites` captures the structural fix, since
this is the second occurrence of the same shape (v26.7.1 was the first).

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: bug fix, no
user-facing surface)
- [x] ~~User-facing documentation updated~~ (N/A: `docs/releasing.md` describes
the release process, which is unchanged — only its CI prerequisites were broken)
- [x] ~~Docs-checklist marked as done~~ (N/A: bug fix)

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
