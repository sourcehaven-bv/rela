---
id: REV-VIVK6R
type: review-checklist
title: 'Review: faces.<name>.messages.notice — text on a face regardless of writability'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] `just test` (full Go `./...`) — all pass
- [x] `just lint` — 0 issues
- [x] `just coverage-check` — package and total thresholds PASS (79.6%)
- [x] `just arch-lint` — OK, no warnings
- [x] `just comment-lint` — no unresolvable doc links across 13975 comments
- [x] `just plimsoll` — clean
- [x] `npm run test:run` — 2454 tests / 151 files pass
- [x] `npm run typecheck` — clean
- [x] `npm run lint` — 0 errors

## Code Review

- [x] cranky-code-reviewer run
- [x] All critical/significant findings addressed
- [x] review-response entities created and linked

**Findings: 8, all addressed. No critical.**

| ID | Severity | Finding |
| --- | --- | --- |
| RR-0HDG3I | significant | `worldAbsent` guard in `noticeNote` is dead code; comment asserted it was load-bearing |
| RR-03729J | significant | No test for a writable NON-BARE face declaring a notice |
| RR-ZF0FNT | significant | Two-note banner layout visually unverified; the CSS was deletable with no test failure |
| RR-BYKCHQ | minor | Faced type with no `bare_face` unpinned |
| RR-2KOT6F | minor | ANDed assertion did not deliver the diagnostics its comment promised |
| RR-3CW3KG | minor | `DynamicForm` exclusion rested on the wrong argument |
| RR-N7C2ZV | minor | `onScreenFace` duplicated `textVars.face`'s expression |
| RR-G2LOOD | nit | metamodel.md example comment contradicted content-states.md |

**The one I got wrong, and how.** RR-0HDG3I is a correction to my own
verification, not just a code fix. I had mutation-tested the `worldAbsent` rule
by removing the computed guard *and* the template guard together, saw a failure,
and concluded the computed guard was load-bearing. The reviewer removed only the
computed term: 67/67 passed. The guard is inert. The lesson is that a mutation
touching two sites at once cannot attribute the failure to either — I
re-verified single-term removal myself before accepting the finding. The term is
kept as belt-and-braces (dropping it would diverge from `readOnlyNote`) but the
comment now says so plainly instead of describing a mechanism that is a spare.

**Also taken:** the reviewer's leverage suggestion #8 — the two notes are now
one `bannerNotes` array, so render order lives in a single literal with its
rationale beside it rather than being encoded in template element order.
Re-verified: swapping the array's two entries still fails the ordering test.

Not taken: the `t.Parallel()` inconsistency (reviewer's #5), explicitly flagged
as "leave it or fix it; don't agonise" — the sibling tests it would match are
inconsistent with each other.

## Acceptance Verification

Each criterion from PLAN-8FWEGI, with evidence:

| AC | Result | Evidence |
| --- | --- | --- |
| 1. Notice renders on a face the reader MAY write | PASS | Unit test + live browser: draft page showed the notice beside working Edit/Publish/Delete |
| 2. Undeclared renders nothing, no empty banner | PASS | Unit test asserts `.world-banner` absent |
| 3. Both keys: both render, `notice` first | PASS | Unit test asserts index order; live browser confirmed two stacked lines |
| 4. Placeholders substitute as for `read_only` | PASS | Unit test on `{face} / {title}`; live run rendered the label and display title, not coordinate/id |
| 5. Wire carries it; empty face omits the block | PASS | `TestSchemaFaces_CarriesNoticeIndependentOfReadOnly`; live `/_schema` showed both keys on `published`, `notice` alone on `draft` |
| 6. `worldAbsent` renders no notice | PASS | Unit test with a positive control on the absent banner |

**Mutation verification.** Every load-bearing decision was checked by breaking
it and confirming a test fails, restoring the source after each:

| Mutation | Result |
| --- | --- |
| Gate `noticeNote` on `mayUpdate` | 2 failed |
| Drop the bare-face fallback from `onScreenFace` | 2 failed |
| Reintroduce `readOnlyNote`'s guard wholesale | 4 failed (incl. the new writable-non-bare test) |
| First-declared-face fallback in `onScreenFace` | 1 failed (the new no-`bare_face` test, and only it) |
| Swap the order in `bannerNotes` | 1 failed |
| Drop the `worldAbsent` template guard | 1 failed |
| Drop `Notice` from `faceMessagesWire` | 1 failed |

**Layout verification (RR-ZF0FNT).** jsdom does no layout, so the one new UI
arrangement was measured in a real browser rather than asserted. At full width:
wrapper computes to `display: block`, both notes full width, 18px each, 4px gap,
no overlap. Forced below the `24rem` flex basis (420px): wrapper wraps under the
label as that basis intends, each note takes two lines, stacked without overlap,
banner grows to 121px with nothing clipped.

**Regression risk to existing `read_only` deployments: none.** Confirmed
independently by the reviewer — `faceMessagesWire` still returns nil for an
empty struct so an existing project serves a byte-identical `_schema`;
`omitempty` on both tags means no new wire keys; and `ShapeProjection` carries
only face NAMES and `BareFace`, not `Messages`, so the data migration shape hash
is unperturbed and no deployment is spuriously gated at startup.
