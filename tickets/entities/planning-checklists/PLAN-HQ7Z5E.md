---
id: PLAN-HQ7Z5E
type: planning-checklist
title: 'Planning: Add comments.Store.Get so a single-comment read stops pulling the whole thread'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

IN: a `Get` method on `comments.Store`; implementations in all four backends;
`Service.Get` delegating to it; conformance coverage.

OUT: `Service.Add`'s identical list-to-count amplification (filed separately);
the `UpdatedAt` backend divergence this work uncovered (TKT-JZY2PM); any change
to the `MaxPerTarget` advisory-cap reasoning.

**Acceptance Criteria:**

1. `comments.Store` gains `Get`, all four backends implement it — compiler.
2. Database backends serve a single-row read — EXPLAIN against seeded rows.
3. `Service.Get` uses it — counting-store test asserting 1 get, 0 lists.
4. Absent id returns `comments.ErrNotFound` — conformance.
5. Face-scoped — conformance, both directions plus same-id-on-two-faces.

## Research

- [x] ~~run `/research`~~ (N/A: approach fixed by the ticket, one method)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A — small, and the ticket's approach sketch was correct.

**Existing Solutions:**

No library question; this is an interface method over SQL the project owns.

Prior art in-tree:
- `storetest.Counting` budget tests — the pattern for pinning that a read path
does not amplify, reused here as `countingStore` in `service_test.go`.
- `commentstest.RunKeyFidelityTests` — the existing split for contracts only the
database backends can honor, which is where the byte-exact `Get` case went.
- `pgcomments.scanComment` already took `pgx.Row`, so `QueryRow` needed no new
decoder; `sqlitecomments.scanComment` took `*sql.Rows` and was widened to a
`rowScanner` interface so both read paths share one decoder.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

`Get(ctx, target, id) (Comment, error)` on `comments.Store`, returning
`ErrNotFound` when absent — the sentinel `Update` and `Delete` already use, so
the handler's existing 404/500 branch needs no change.

- pg/sqlite: `WHERE target_key = ? AND id = ?`, served by
`PRIMARY KEY (target_key, id)`. No new index.
- file/mem: read the thread and pick. Free there — a thread is one document.

Alternative rejected: returning only the author. The one production caller
(`gateCommentMutation`) also reads `existing.Body` and `existing.Resolved` for
partial-update semantics, so a narrower read would force a second fetch. The
ticket flagged this and the answer is "no".

**Files to modify:**

`internal/comments/comments.go`, `service.go`, `service_test.go`,
`commentstest/commentstest.go`, and `Get` in each of `filecomments/file.go`,
`memcomments/mem.go`, `pgcomments/pg.go`, `sqlitecomments/sqlite.go`.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

`target` and `id` arrive from the URL path, already resolved and read-gated by
`gateCommentTarget` before `Get` is reached. Both are bound as SQL parameters,
never interpolated. `filecomments` keeps its existing unsafe-id refusal.

**Security-Sensitive Operations:**

`Get`'s result decides whether an `*-own` permission covers a mutation, so the
method is on the authorization path. Two properties matter and both are pinned:

- It must resolve within the named target only. A comment reachable across faces
would let a request name one face and have its permission decided against a
record from another, while the subsequent write used the named pair — check and
write looking at different rows.
- A missing row must be distinguishable from a failed query. Verified against
both drivers that a deferred query error is not `ErrNoRows`, so an outage cannot
be reported as a 404.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

`RunGetTests` in the shared conformance suite, so all four backends are held to
one contract: full round-trip incl. UTC location, agreement with `List` across a
thread, absent id, absent thread, scoping (5 cases), zero `UpdatedAt`, reflects
an update, deleted comment gone. Byte-exact keys in `RunKeyFidelityTests`
(database backends only). AC3 via `countingStore`. AC2 by EXPLAIN on both
engines.

**Edge Cases:**

Absent thread vs absent comment; same id on two faces; same id on two targets;
default-face-through-named and named-through-default (asymmetric under a prefix
bug); case-differing ids; NULL `updated_at`.

**Negative Tests:**

Every scoping case asserts `ErrNotFound` rather than a wrong row. Mutation
testing confirmed each: breaking face scoping, swallowing the not-found error,
ignoring `target_key`, and reverting `Service.Get` to list-and-scan each fail
the tests that claim to catch them.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- *A second read path diverging from `List`.* Mitigated by asserting the two
agree, not merely that `Get` returns something plausible.
- *An infrastructure error misreported as 404.* Mitigated by ordering the
`ErrNoRows` check before the generic wrap; verified empirically per driver.
- *`pgcomments` conformance not running in CI.* Already covered by the step
added in TKT-OGTVJW, which fails on `--- SKIP`.

Effort: s, as estimated.

## Documentation Planning

- [x] ~~User-facing docs identified~~ (N/A: internal interface, no API/CLI change)
- [x] ~~Docs-checklist created~~ (N/A: `kind=enhancement` but no user-facing surface)

**Documentation Impact:**

N/A — internal change. The wire format, routes and CLI are untouched;
`CLAUDE.md`'s comments rule ("any new one must pass `commentstest.RunAll`")
still holds unchanged and gains coverage.

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: the ticket
carried the design, settled when it was deferred from TKT-OGTVJW)
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** None at design time. A post-implementation
`/code-review` raised four; see the review checklist.
