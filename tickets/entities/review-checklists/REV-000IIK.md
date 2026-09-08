---
id: REV-000IIK
type: review-checklist
title: 'Review: A denied ?world= short-circuits past worldCapablePath, serving default-world content on refused routes'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`)

`go test ./internal/dataentry/` ok (33s); the reviewer additionally ran the
package under `-race` (315s, green). `golangci-lint run
./internal/dataentry/...` → 0 issues. `just comment-lint` → no unresolvable doc
links across 13089 comments. `just coverage-check`, `just arch-lint`, `just
plimsoll` all pass.

**Comment findings.** `just comment-report` shows two `duplication` findings in
the touched files (`viewworld.go:309`, `worldneighbors.go:411`) — both
**pre-existing**, neither introduced here. The diff adds none; see RR-I4I8MX,
which cut the restating prose that would have.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

Two reviews ran: `rela-security-reviewer` (security invariants) and
`cranky-code-reviewer` (quality). No critical findings from either.

The security review found **no issues** and independently reproduced the
vulnerability, sharpening the ticket in the process: the grant oracle is present
on all five refused routes (each returning a *different* status to a denied
principal — 200, 200, 404, 400, 400 — against a uniform 422 when permitted),
while the content disclosure is confined to `_analyze`. The bug's description
and reproduction table were updated to record that.

Self-review caught one defect before either agent ran: the refusal was initially
hoisted *above* the duplicate/unknown 400 arms, which shadowed `no world named
"bogus" is declared` with a vaguer 422 on exactly the routes an operator is most
likely to be experimenting on. Moved below them and pinned by a test.

**Review Responses:** RR-7MSB4K (significant, addressed), RR-02A10V
(significant, wont-fix — premise incorrect, see below), RR-I4I8MX (minor,
addressed), RR-8SRRU9 (minor, addressed).

RR-02A10V is the one finding not acted on as reported. The review claimed
`requested == ""` was an unreachable branch, verified by panic-instrumenting the
function and running the suite. It is reachable: an explicit-but-empty `?world=`
sets `explicit`, survives attachWorld's early return, and arrives with
`requested == ""`. Confirmed by request (`GET /api/v1/_analyze?world=` → 200)
rather than by argument. Removing the check would 422 any client that appends an
unset parameter. The finding was still productive — the test defending that
branch *was* weak, because the sibling spelling `?world=default` is already
covered elsewhere — so it was replaced with a router-level test for the
genuinely uncovered case.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- *A denied world must not receive default-world content on a refused route* — **PASS**. `TestAttachWorld_DeniedWorldRefusedLikePermitted`, LEAK assertion on the seeded title. Fails on all five subtests when the guard is disabled.
- *A denied world must be indistinguishable from a permitted one on a refused route* — **PASS**. Same test, asserting status and body equality rather than absence of a leak, so it keeps failing for any future divergence.
- *A denied world must still reach world-capable routes* — **PASS**. `TestAttachWorld_DeniedWorldStillReachesCapableRoutes`; without it, "refuse everything" would satisfy the first two criteria while reintroducing the existence oracle the denied handle exists to close.
- *Config diagnostics must not regress* — **PASS**. `TestRefuseWorldIncapablePath_DoesNotShadowConfigErrors`, verified to fail when the guard order is inverted.
- *The default world must pass through on every route* — **PASS**. `TestRefuseWorldIncapablePath_EmptyWorldIsTheDefaultWorld` plus the pre-existing `TestAttachWorld_DefaultWorldIsUnaffected`.

Both guards mutation-tested individually; each has a test that fails when it
alone is disabled.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: bug fix, no user-facing surface change)
- [x] ~~User-facing documentation updated~~ (N/A: the 422 `world_unsupported` response is already documented; this makes an existing rule apply uniformly rather than adding one)
- [x] ~~Docs-checklist marked as done~~ (N/A: no docs-checklist needed)

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
