---
id: REV-N7QF2M
type: review-checklist
title: 'Review: Deleting an entity panics when comments are disabled (typed-nil subscriber in alias fanout)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`)

`go test ./internal/appbuild/... ./internal/comments/... ./internal/entitymanager/...`
green, run independently by the reviewer as well as the author.
`golangci-lint run internal/appbuild/...` → 0 issues.

Full-repo `just test` and `just lint` cannot be trusted on the author's machine:
`cmd/rela-desktop` fails to build because the Xcode licence has not been
accepted, so cgo preprocessing fails. That failure is present on `develop` too
and is unrelated to this change. CI is the authority — `Test`, `E2E`, `Lint`,
`SQLite Backend`, `Fuzz`, `Architecture`, `Comment lint` and both Cross-Compile
matrices are green on the PR.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

`cranky-code-reviewer` found **no critical issues** and independently reproduced
the defect: reverting the guard to `s != nil` reproduces the production panic at
`internal/comments/service.go:185`, with the boxed
`(*comments.Service)(nil)` visible in the failure output.

Four significant findings, all addressed:

1. **`reflect.Ptr` on a Go 1.26 module** — switched to `reflect.Pointer`. Same
   constant, current spelling.
2. **The doc comment overclaimed a system-wide chokepoint.** It is a chokepoint
   for alias subscribers only; the same services also reach `Services` as
   concrete fields where each consumer nil-checks separately. Both the comment
   and the ticket's `prevention` now say so, so the next reader does not skip
   their own check on the strength of this fix.
3. **`TestAliasFanout_DisabledCommentsSurviveDelete` held a provably dead
   `if rewriter != nil` block** — unreachable after the fatal `require.Nil`
   above it, and the source of the panic that killed the test binary under the
   old code. Deleted; the test now fails with a clean assertion instead.
4. **No test that an *enabled* service survives the filter.** Added
   `TestAliasFanout_EnabledCommentsStillSubscribe`. Without it, an over-eager
   `isNilSubscriber` would silently stop comment cleanup on delete — a quieter
   bug than the panic, and precisely the hazard `EntityDeleted` documents.

Minor findings also applied: dropped the unreachable `reflect.Interface` case
(`reflect.ValueOf` always yields the dynamic type), dropped a paragraph that
restated the `switch` below it, added the house `Nil:` contract tag, and
removed shouty caps from the test comments.

**Deliberately not done.** The reviewer's leverage note — make "disabled"
unrepresentable by returning an explicit optional from `buildComments` instead
of a nil pointer with a `//nolint:nilnil` suppression — is the change that would
prevent a third occurrence. It is a refactor of the optional-subsystem
convention, larger than this bug warrants, and is recorded in the bug's
`prevention` rather than silently dropped.

**Remaining typed-nil paths audited.** The reviewer traced every consumer of
both optional services. `buildStateAndAliases` (appbuild.go:2182) returns a nil
`*caldavalias.Service` the same way; it reaches the fanout (now handled) and
`Services.caldavAliases`, which is a concrete field where `!= nil` works and is
gated at `dataentry/router.go:179` and `caldav_handler.go:31`. `commentSvc`
likewise reaches a concrete field, gated at `comments_handler.go:46`. No
reachable panic remains. Worth recording: `caldavalias.EntityDeleted` never
touches its receiver and so would not have panicked, but its `EntityRenamed`
(line 245, `s.mu.Lock()`) would have — the CalDAV half was latent, not safe.

## Quality

- [x] Change is scoped to the defect — two files, no drive-by edits
- [x] No unrelated changes in the diff
- [x] Follows project patterns — doc comments carry the reasoning, not a
      restatement of the code

**Known housekeeping issue, pre-existing.** Running the `internal/appbuild`
suite mutates a tracked file: it rewrote `status:` in this bug's own ticket
during the review. That means a plain `go test` leaves the working tree dirty.
It predates this change and was not chased here.
