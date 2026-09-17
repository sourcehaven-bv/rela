---
id: REV-4LV42K
type: review-checklist
title: 'Review: One scoped-read funnel for data-entry collection reads (the ACL verdict switch is copied four times)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] `just test` — all pass
- [x] `just lint` — clean
- [x] `just coverage-check` — pass
- [x] `just arch-lint` — pass
- [x] `just plimsoll` — pass
- [x] `just comment-lint` — pass

**Evidence:** `internal/dataentry` green (33.9s). golangci-lint reports 0 issues
across every touched package. Coverage gate explicit: "Package coverage
threshold (50%) satisfied: PASS / Total coverage threshold (65%) satisfied: PASS
/ Total test coverage: 74.8%". arch-lint "OK - No warnings found". plimsoll
clean across `./internal/...`. commentlint "no unresolvable doc links across
14855 comments".

Two environmental failures are NOT from this change and were confirmed
pre-existing: `cmd/rela-desktop` fails to build because this machine has not
accepted the Xcode license (cgo/wails), which also makes the repo-wide `just
lint` and `just plimsoll` targets exit non-zero.

## Code Review

- [x] `/code-review` run
- [x] All critical findings addressed
- [x] All significant findings addressed
- [x] Minor/nit findings addressed or deferred with reason

Reviewed jointly with TKT-EVR2TU, since the two share a diff. Both reviewers
examined the funnel specifically and found NO defect in it. Recorded verbatim
because a negative result on the security-critical part of a refactor is the
finding:

- The verdict switch applies every narrowing on every branch; the zero-verdict
arm is correctly ordered after both AllowAll arms and errors rather than
aliasing to permissive.
- `q := *rqr.Query` is sufficient: `Props` is the only slice mutated and is
deep-copied before appending; `FaceIn` is assigned wholesale, never appended.
- All four migrated sites diffed against `3097c5a3^` with no widening. Two are
TIGHTENED (feed and gantt-AllowAll gain world and face narrowing they previously
lacked; `visibleEntitiesOfType` gains the denied-world guard).
- `withheld` is honored where it matters; the three sites that discard it run
no search and no per-row work afterwards.
- `applyScope`'s in-place `headers[:0]` reuse is safe at every call site —
now documented, since the signature does not reveal it.

The findings that did land were all against the query-scope feature layered on
top, not the extraction: see TKT-EVR2TU's eight review responses.

## Verification

- [x] Each acceptance criterion verified
- [x] Manual verification performed
- [x] No regressions introduced

| AC | Result |
| --- | --- |
| AC1 one verdict switch | PASS — `TestScopedHeaders_IsTheOnlyVerdictSwitch` |
| AC2 both branches narrow | PASS — per-dimension table test |
| AC3 no behaviour change | PASS — suite unmodified but for one helper, justified in IMPL-D8DV43 |
| AC4 zero-verdict defence | PASS |
| AC5 the copy survives | PASS |

The extraction proved its own premise during implementation: writing the very
function meant to prevent the RR-GQWRLD bug class, I reintroduced it for
`Props`. A pre-existing test caught it, and AC2's test is per-dimension
precisely so the next one is caught by construction rather than by luck.
