---
id: REV-GMOHUS
type: review-checklist
title: 'Review: Make face→world resolution total: a tie is the chain head alone'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] `go test ./...` passes — except `cmd/rela-desktop`, which needs a cgo
toolchain this machine lacks (Xcode licence unaccepted). Unrelated to the
change; CI builds it.
- [x] `golangci-lint` clean on the changed packages
- [x] `just arch-lint` clean
- [x] `just comment-lint` clean (no unresolvable doc links across 15067 comments)
- [x] ~~`just coverage-check`~~ (N/A: fails on the same cgo build, not on a
threshold. No production code was added — the diff is a struct field removal,
comments and tests.)
- [x] Frontend: `test:run` on the schema store (47 pass), `typecheck`, `eslint`
all clean

## Code Review

- [x] `/code-review` run (cranky-code-reviewer)
- [x] All critical findings addressed — none raised
- [x] All significant findings addressed — RR-Y0UN58, RR-BF9Q6G, both fixed

## Verification

- [x] Change verified against in-tree fixtures: all three prototypes still
validate.
- [x] New test mutation-checked — reverting `primacyKey` to its three-field form
fails `TestFacePrimacy_SameHeadDifferentOtherwiseIsATie`.
- [x] The `docs/metamodel.md` example verified executable in both directions:
loads with `primary_for: nl`, fails the load without it.
- [x] The reviewer's central objection independently reproduced before acting on
it (see RR-Y0UN58) rather than accepted on report.

## Out-of-scope findings filed

The review surfaced four defects outside this diff. Filed rather than fixed
here, to keep the change reviewable:

- BUG-UA3BK3 — `analysis.faceDeclared` treats a bare row on a faced type as
declared, so `rela analyze` cannot see stranded rows. Narrowed from the
reviewer's description after checking the code: the function is not
unconditionally `true`, only its default-face early return is wrong.
- BUG-TOX8U4 — `migrateFaceStep.Run` is untransacted and
`renameEntityTypeStep.Validate` does not compare face sets. Filed unverified,
with that stated on the ticket.
- The perf fixture's stale schema comments were fixed here, since they are
one line each and directly contradict the model this change reasons about.

## Notes

One reviewer sub-claim was **not** accepted as reported: that
`analysis.faceDeclared` returns `true` unconditionally. It does not — the body
does a real `def.Faces[...]` lookup. Only the `IsDefault()` early return is
stale. BUG-UA3BK3 records the accurate version, since filing the overstated one
would have sent the next person looking for a bug that is not there.

An inconsistency the reviewer flagged at
`internal/worlds/worlds_test.go:150-208` was left as-is: the fixture gains a
`primary_for:` claim because two worlds head `published` for `note`, while the
subtest is about `memo`, whose chain neither world satisfies. The change is
incidental to what the test asserts and masks nothing. The reviewer reached no
conclusion there; this is my reading, recorded so a future reader can disagree
with it.
