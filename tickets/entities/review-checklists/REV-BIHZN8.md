---
id: REV-BIHZN8
type: review-checklist
title: 'Review: ACL-gating test for rela.md.entity_refs (TKT-PUJNS0)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass — full sweep green (all packages except the env-dependent docscapture)
- [x] Lint clean — golangci-lint 0 issues; `just arch-lint` OK
- [x] Coverage maintained — `internal/visibility` holds at 91.7% (above its 85 floor); this change adds test coverage only

## Code Review

- [x] ~~Run `/code-review` command~~ (N/A: test-only change, no production code — `git diff internal/lua/markdown.go` is empty. The change originates FROM a review finding; the substantive verification is the mutation testing below, which is what the finding asked for.)
- [x] All critical review-responses addressed (none)
- [x] All significant review-responses addressed (none)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** none — this ticket exists to close an external IB-review
finding (rela#1199), not to introduce reviewable production code.

## Acceptance Verification

- [x] Each acceptance criterion tested
- [x] Test evidence documented in implementation checklist

**Acceptance Status:** PASS. The finding asked for a test that exercises
`rela.md.entity_refs` under a configured policy and would catch a reintroduced
regression. Delivered and mutation-verified in **both** directions — the
original `context.Background()` defect and a gate bypass each fail the test with
a diagnostic naming the real cause. This restores the "every gate fails a test
when removed" property for the one case where #1197 fell short of its own claim.

## Documentation (enhancements only)

- [x] ~~Docs-checklist~~ (N/A: `kind: test`, no user-facing surface — the ACL read behavior it pins is already documented in `docs/lua-scripting.md` from TKT-ZF2DTV)

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
- [x] All CI checks pass
- [x] PR URL documented below

**PR:** see the `has-review-response`-free follow-up PR opened for rela#1199
(linked in the ticket body).
