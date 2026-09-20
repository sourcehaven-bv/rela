---
id: REV-VHXNX9
type: review-checklist
title: 'Review: Add comments.Store.Get so a single-comment read stops pulling the whole thread'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`)

Tests run under `-race` on BOTH builds and against a live PostgreSQL 18, because
the default `go test ./...` skips `pgcomments` silently — a green run without
`RELA_TEST_DATABASE_URL` proves nothing about the backend this ticket is for.
All four backends pass `commentstest.RunAll`; both database backends also pass
`RunKeyFidelityTests`.

`golangci-lint` clean on `./internal/comments/...` (`funlen` caught
`RunGetTests` twice as cases were added; resolved by extracting
`runGetFidelityTests` and `runGetScopingTests` along real seams rather than by
raising a threshold or suppressing).

Coverage gate exits 0. `internal/comments` 80.0%, filecomments 81.0%,
memcomments 95.5%, sqlitecomments 77.0%, pgcomments 77.6% with the live DB (2.6%
when skipped, which is why its exclusion added in TKT-OGTVJW stays).

**Comment findings.** `just comment-report` shows 2 advisory findings under
`internal/comments/`, both pre-existing (`sqlite.go:255 selectFaces` and the
package doc). Verified the diff introduces none by running the report against a
stashed tree: identical count before and after. No suppressions added.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-P465GP, RR-PTX67Y, RR-40OP0D, RR-7971FD

No critical findings. Three significant and one minor, all addressed. The three
significant ones were all about the TESTS rather than the implementation — the
suite carried a confident doc comment about catching timezone drift and
cross-face resolution while omitting the `UpdatedAt` assertion, byte-exact keys,
and the same-id-on-two-faces case that carries the actual security weight.

One reviewer claim was checked and did not hold: that a backend ignoring
`target_key` would pass the cross-face subtest. It fails on the cross-target
case. Recorded in RR-PTX67Y rather than silently accepted, and the underlying
point (nothing pinned "the right row" directly) was valid and is now fixed.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

1. **PASS** — `comments.Store` has `Get`; all four backends implement it, held
by the compiler and `var _ comments.Store` assertions.
2. **PASS** — by EXPLAIN, not inspection. PostgreSQL: `Index Scan using
comments_pkey`, 3 buffers, against the 117 buffers + 69 kB quicksort the
replaced `List` cost on a 100-comment thread. SQLite: `SEARCH comments USING
INDEX sqlite_autoindex_comments_1 (target_key=? AND id=?)`.
3. **PASS** — `TestGet_DoesNotReadTheWholeThread` asserts 1 get, 0 lists; fails
when `Service.Get` is reverted to list-and-scan.
4. **PASS** — two conformance cases (absent id, absent thread), on all four
backends.
5. **PASS** — five scoping cases, both face directions plus the two
colliding-id cases.

Every new assertion was mutation-tested: four separate bugs introduced, each
confirmed to fail the test claiming to catch it, each reverted.

## Documentation (enhancements only)

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: no user-facing surface)
- [x] ~~User-facing documentation updated~~ (N/A: internal interface only)
- [x] ~~Docs-checklist marked as done~~ (N/A)

**Docs Checklist:** N/A — `kind=enhancement`, but the change is an internal
interface method. No API shape, route, CLI flag or config key changes; the wire
format is byte-identical. `CLAUDE.md`'s comments rule ("any new one must pass
`commentstest.RunAll`") holds unchanged and gains coverage, so it needed no
edit.

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

Two follow-ups were filed rather than folded in, both out of scope for a
read-amplification fix:

- **TKT-JZY2PM** — `UpdatedAt` is stamped by the database backends and not by
file/memory, so the same edit yields a different record per build. Found by a
new assertion in this work.
- **TKT-RQCH12** — `Service.Add` has the identical list-to-count amplification on
the write path, raised by the reviewer and verified in the code.

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
