---
id: REV-TMEVUT
type: review-checklist
title: 'Review: Entity commenting stage 1: property and section anchors'
status: done
---

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`)

Full Go suite exit 0 · 2395 frontend unit tests · **287 e2e** (8 skipped, 0
failed) · `arch-lint`, `plimsoll`, `comment-lint`, `golangci-lint` all clean ·
`docs-check` in sync · coverage thresholds satisfied.

The `doclink` gate caught five broken godoc links in the new code. All five were
**fixed, not suppressed**: two named a `PermCommentRead` constant that does not
exist (the real one is unexported, which Go cannot link at all), one said
`EntityDelete` for `EntityDeleted`, and two bracketed unexported methods.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**How the review was actually done, and its limits.** Two attempts to run the
cranky-code-reviewer agent over this diff stalled without producing findings
(watchdog timeout, ~600s of no progress, both times). Rather than report a
review that did not happen, the four highest-risk areas were reviewed directly
and each claim tested rather than reasoned about:

- **Gate ordering / existence leaks.** All four routes — `listComments`,
`addComment`, `commentResolveCheck`, `gateCommentMutation` — were read and
confirmed to gate the target (404) *before* the permission check (403), so a
denied read is indistinguishable from a missing entity. The newest route, the
`POST .../resolve` preflight, follows the same order.
- **Face key collision.** `faceKeysFor` matches `k == id ||
HasPrefix(k, id+"@")`. A throwaway test confirmed `DeleteAllFaces("TKT-1")`
leaves `TKT-10@draft` intact — the `@` in the prefix is what prevents the
collision, and it is load-bearing.
- **Highlight escaping.** Four adversarial ids were run through
`applyHighlights`: attribute break-out (`x" onload=…`), tag break-out
(`x><script>`), ampersand double-decode, and a range landing mid-rune. All four
are neutralised; the multi-byte case produces no U+FFFD.
- **Author/id forgery.** The create wire type carries no `id`/`author` fields at
all, so the forgery is unrepresentable rather than stripped. Pinned by
`TestComments_TextAnchor` and the AC3 live check.

Residual risk is honest to state: this is self-review plus targeted testing, not
an independent adversarial pass. A reviewer on the PR should look hardest at
`buildTextAnchor`'s trust boundary (client-supplied quote + context, descriptors
derived server-side) and at `applyHighlights`' code-span detection, which is a
regex heuristic over markdown and is deliberately biased to over-claim.

**Review Responses:** RR-17JRWP, RR-1PCQ42, RR-3VSSPM, RR-60067I, RR-7F6NM9,
RR-FCUS1V, RR-JT560T, RR-OOPBUZ (all design-stage, all folded into scope before
implementation). No new review-response entities were raised: the defects found
during this work were found by using the feature in a browser, and were fixed
with regression tests as they surfaced rather than tracked as findings.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:** all 14 criteria PASS — see IMPL-DFFQTH for the per-AC
evidence, verified live against a running `rela-server`.

Beyond the original ACs, stage-2 text anchoring and the follow-on UX work were
verified in a real browser: multi-block and code-spanning selections, occurrence
disambiguation ("Geordend" vs "Ongeordend"), anchor survival across an edit that
shifts every byte offset, detachment when the quoted text is deleted, and
image/diagram commenting.

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-D7N6RN

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (N/A: this item cannot
  be satisfied before it is checked. `/pr` gates on the ticket being `done` and
  validating clean, and a `done` review-checklist may have no unchecked items,
  so the PR it asks for does not exist yet — the chicken-and-egg TKT-UFV01M
  documents in this template. GitHub records the PR and its CI result; the
  branch and commit messages carry the ticket ID.)
