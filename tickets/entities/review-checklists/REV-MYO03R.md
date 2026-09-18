---
id: REV-MYO03R
type: review-checklist
title: 'Review: Insert and edit external links in the Milkdown editor (plus horizontal rule and undo/redo)'
status: in-progress
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`)

Frontend: 2805 unit tests across 171 files, `vue-tsc` clean, `eslint` reporting
no errors (two pre-existing warnings: a non-null assertion in `activeFormats.ts`
and the `max-lines` warning on `MilkdownEditor.vue`, discussed below).

E2E: 24 markdown-editor tests, including the five new link specs.

`just ci` green. One failure it DID catch on the first run was mine and worth
recording: `TestMdCorpusRoundTrip` rejected `PLAN-RY25IP.md`, because a list
item in my own planning document wrapped onto a line beginning `+ tooltip…`,
which markdown reparses as a new list item and makes the list loose. Reworded.
The corpus test earning its keep against a document written during this very
ticket is a good sign for the test, less so for my line wrapping.

**Comment findings.** Two comments were corrected because they described
behaviour the code did not have, which is worse than no comment — it tells a
reviewer not to check. Both are tracked as review-responses rather than silently
fixed (RR-EOIEPJ, RR-W4CXNN). A third, `extentOf`'s claim about "identical mark
instance", was wrong in a subtler way: the code uses `Mark.eq` value equality,
and two adjacent same-href links are merged by ProseMirror at PARSE time anyway,
before `extentOf` is reached. The comment now says that.

`MilkdownEditor.vue`'s script block sits at ~545 lines against a 500-line
warning. It was under before this ticket. Two extractions (`insertEntityRef.ts`,
`mentionTrigger.ts`) held the growth to ~45 net lines instead of ~115, and the
first removed a near-duplicate insertion routine. The remaining candidate is the
write-back/serialization block, which is welded to component lifecycle state;
extracting it would need mutable state threaded through a seam that does not
want to exist. Left for its own refactor rather than done badly here.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

Two reviewers ran: `cranky-code-reviewer` for quality and correctness, and
`rela-security-reviewer` for the URL gate specifically, since this feature's
whole risk is a value that serializes into an entity file and renders elsewhere.

**Review Responses:** RR-MYTU5V (critical, addressed), RR-EOIEPJ (significant,
addressed), RR-W4CXNN (significant, deferred), RR-IG6RNH (minor, addressed).
Plus the three design-review responses from planning: RR-VR84MY, RR-K0PGQW,
RR-Z5WLG1 — all addressed.

The critical one was three variants of a single defect: `handlePaste` returning
`true` on conditions it had not verified, so ProseMirror's own handling never
ran and the user's clipboard was lost. Worst of the three destroyed an existing
link and the prose around it. All three were reproduced against the real editor
before fixing, and each guard is **mutation-verified** — reverting it makes its
test fail.

That mutation step was not ceremony. Three of my first-draft tests passed
against the very bugs they were written for:

- The code-block paste test asserted "no link was created", which is true
whether or not the paste is swallowed. Rewritten to assert the paste still
HAPPENS as plain text.
- The overlap test asserted the same vacuous thing. Rewritten to assert the
existing link survives intact.
- The e2e geometry test reached the panel by CLICKING the link, which puts the
caret inside it — so the selection and link rects nearly coincide and any stable
tolerance also hides the bug. Rewritten to place the caret in a different
paragraph first.

**Findings checked and rejected**, rather than fixed reflexively:

- The security review reported an unbalanced `)` truncating a URL on save. It
does not reproduce under this repo's pinned `RELA_STRINGIFY_OPTIONS` — measured
through both the raw remark pair and the mounted editor,
`https://example.com/path)tok=abc` round-trips byte-identical.
- The code review reported `findLinkAt` merging adjacent same-href links. Real,
but caused upstream: ProseMirror merges them into one text node at parse time.
Nothing downstream can un-merge it. Comment corrected instead.
- My own commit message claimed the `absolute`-vs-`fixed` coordinate space was
half the tooltip misplacement. Testing showed floating-ui computes correctly for
whichever strategy it is given; the real defect was solely the anchor.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

| AC | Status | Evidence |
|---|---|---|
| 1 selection → link | PASS | unit + e2e, asserts `[documentation](https://example.com/guide)` |
| 2 panel on the link | PASS | unit (structure) + e2e (geometry, mutation-verified) |
| 3 unlink keeps text | PASS | unit + e2e |
| 4 refuse javascript:/data: | PASS | unit (pure + mounted) + e2e; asserts nothing written AND the message does not echo input |
| 5 bare host → https | PASS | unit + e2e |
| 6 paste over selection | PASS | unit; plus three new fall-through cases |
| 7 divider → `---` | PASS | unit + e2e |
| 8 undo/redo `aria-disabled` | PASS | unit + e2e, asserts the native attribute is absent |
| 9 open+save emits nothing | PASS | write-back guard test |
| 10 keyboard reachable | PASS | e2e via toolbar; panel is not a keyboard route and is no longer claimed to be |
| 11 overlap retargets | PASS | unit; mutation-verified |
| 12 stored `javascript:` untouched | PASS | unit, round-trips byte-identical |
| 13 mailto params stripped | PASS | unit; fragment-hidden form added after security review |
| 14 host:port | PASS | unit + e2e |

Manual verification against a running server is recorded in IMPL-2UKGJX,
including the one defect only a real browser could show (panel placement).

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-9Z5LKD

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

One deliberate gap is documented rather than hidden: the hover trigger chosen in
planning was never built (RR-W4CXNN, deferred). Every claim of it has been
removed from the docs and source, so nothing describes behaviour that does not
exist. The keyboard and caret routes are complete and tested, which is what the
accessibility requirement depends on.

## Pull Request

- [ ] Run `/pr` command to create PR and monitor CI

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
