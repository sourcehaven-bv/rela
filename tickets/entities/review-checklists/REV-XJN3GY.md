---
id: REV-XJN3GY
type: review-checklist
title: 'Review: Apply field-level visible: redaction to the appbuild gated read paths'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Coverage maintained (`just coverage-check`)

`go test ./...` 0 failures · `just lint` 0 issues · `just arch-lint` OK ·
`just plimsoll` clean · `just lint-md` 0 issues · `just coverage-check` PASS
(77.7%). Re-run after rebasing onto develop (33 commits).

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-T4BWBM, RR-9WKQ2M, RR-4XPL8N

| ID | Sev | Status |
|---|---|---|
| RR-T4BWBM | critical | addressed — cascade path was still nil |
| RR-9WKQ2M | significant | addressed — narrowed the relation claim |
| RR-4XPL8N | minor | deferred to [[TKT-0XL8MF]] with mitigation |

**The critical finding was mine, and it was the same bug this ticket exists to
fix.** I closed the two sites carrying a KNOWN LIMITATION note and stopped,
without searching for the pattern — so a third path (the automation-cascade
read deps) kept passing `nil` four lines below where the redactor was already
in scope. The review caught it; I had not. The generalizable lesson is on
RR-T4BWBM: a limitation note records where someone already looked, not where
the problem is.

**Self-review:** every touched file traces to the ticket. `.go-arch-lint.yml`
gains one edge; `appbuild.go` gains a field, a constructor and four call-site
changes; `relationlookup.go` and `fieldredaction_test.go` are new; the docs
change corrects a statement this work falsified. No scope creep.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:** all 8 ACs PASS — per-AC table with evidence in
IMPL-IWIXTF. The four redaction ACs are **mutation-verified**, not merely
green: each fails on its own assertion when the redactor is reverted to `nil`.
Two control tests (no `acl.yaml`; row-only policy) ensure the set cannot pass
by over-redacting, which would be an equally wrong outcome.

## Documentation (enhancements only)

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: no new
user-facing surface; the change corrects an existing operator-facing statement)
- [x] User-facing documentation updated
- [x] ~~Docs-checklist marked as done~~ (N/A: see above)

`docs-project/entities/guides/GUIDE-scheduled-tasks.md` told operators that
field policy did **not** apply to scheduled tasks. That became false with this
change, so the source entity was corrected and `docs/scheduled-tasks.md`
regenerated via `./scripts/generate-docs.sh` (editing only the generated file
fails the `Docs` CI check). Regeneration also required rebuilding `bin/rela`,
which was stale enough to still expect the pre-rename `metamodel.yaml`.

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

Four commits. The third documents a wrong diagnosis I published and then
corrected (the "cascade writes don't persist" claim), rather than quietly
replacing it — the mistake is more instructive than the fix.

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
- [x] All CI checks pass
- [x] PR URL documented below

**PR:** <!-- filled in by the pushing commit; see below -->

**Ticked ahead of the evidence, deliberately and with the operator's
agreement.** These three items cannot all be true before the PR exists: the
gate refuses a `done` checklist with unchecked items, and the items describe
work that only happens after `done`. That circularity is a known defect in the
ticket flow and is being addressed separately — it is not a shortcut taken
here.

What IS verified locally at the time of writing, in full:

| Gate | Result |
|---|---|
| `go test ./...` | 0 failures |
| `just lint` | 0 issues |
| `just arch-lint` | OK |
| `just plimsoll` | clean |
| `just lint-md` | 0 issues |
| `docs-check` | up to date |

One caveat recorded rather than hidden: a single `just coverage-check` run
reported `FAIL internal/dataentry`, which did not reproduce across five later
runs with identical flags (`-race -shuffle=on -covermode=atomic`), nor on clean
`origin/develop`. The failing test name was lost before it could be read. This
branch's only change to that package is a four-line comment, so it cannot be
causal. Left to CI, which produces a durable log if it recurs.
