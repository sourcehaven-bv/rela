---
id: PLAN-HMJA45
type: planning-checklist
title: 'Planning: One scoped-read funnel for data-entry collection reads (the ACL verdict switch is copied four times)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** Extract the four duplicated ACL read-verdict switches in
`internal/dataentry` into one funnel. Behaviour-preserving; no new feature. In
scope: the four collection-read sites (list/search, feed, gantt × 2). Out of
scope: changing what any surface returns, and the paged pushdown path
(`planListPushdown`), which builds the same narrowed query in a shape the store
serves directly and so cannot route through a function returning a slice.

**Acceptance Criteria:** AC1-AC5 on TKT-VAKI0Q.

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A — the survey that produced this ticket IS the research. It
enumerated the four sites from the call sites of the read APIs and read the
comments each carried, which is where the two historical bugs came from.

**Existing Solutions:**

- **`ceilingguard_test.go`** is the prior art for AC1: a guard test that scans
its own package for a forbidden pattern and fails on a new occurrence, with an
exemption list so a legitimate exception is declared rather than silently
tolerated. Copied directly, including the "a new file must be clean or
explicitly exempted" property.
- The ACL layer's own `ReadQueryResult` already carries the composed
`GraphQuery`, so the funnel consumes an existing contract rather than inventing
a narrowing representation.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

One `scopedHeaders(ctx, svc, rqr, req)` owning the verdict switch, with the
narrowings as fields on a `scopeRequest` struct so adding one is a compile-time
prompt at a single site. `scopedEntities` adapts to the header-entity shape via
the existing `headerEntity`. Returns `(headers, withheld, err)` — see below.

Migrated: `scopedSortedEntities` (api_v1.go), `visibleEntitiesOfType`
(helpers.go), `feedEntitySource.listType` (feed_handler.go), and both gantt
paths.

**`withheld` was not in the original design and had to be added.** The first
migration lost `scopedSortedEntities`' early return, which let a DenyAll
principal reach the search backend and probe its latency through `?q=`
(RR-X56H). The funnel therefore distinguishes "refused outright" from "permitted
but matched nothing" — identical on the wire, different internally so a caller
can skip downstream work. Caught by `TestACLList_DenyAllSearchShortCircuit`.

**Alternative rejected:** leaving the four sites and adding a shared helper for
the narrowings only. That keeps four verdict switches, which is the thing that
actually went wrong twice — the narrowings were never the hard part, threading
them through every branch was.

**Files modified:** `internal/dataentry/{scopedread.go (new), api_v1.go,
helpers.go, feed_handler.go, gantt_handler.go, app.go}`.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** No new external input. The funnel's inputs are
an `acl.ReadQueryResult` (produced by the ACL layer) and a `scopeRequest` (built
by the calling handler from already-validated request state).

**Security-Sensitive Operations:** This IS the security-sensitive operation — it
is the point where an ACL verdict becomes a store query. Four properties are
load-bearing and each has a test:

- **Every narrowing rides on EVERY branch.** The RR-GQWRLD / TKT-O7R2A1 shape.
A per-dimension table test replaces "remember to check this per feature".
- **A zero `ReadQueryResult` is REFUSED, not treated as AllowAll.** The zero
value would otherwise alias the most permissive branch, making a
misconfiguration a silent full disclosure.
- **The ACL query is COPIED before stamping.** The ACL layer may cache a result
per principal, so stamping in place leaks one request's narrowing into the next
caller's.
- **`withheld` ≠ empty.** The two are identical on the wire and must stay so;
the distinction exists only so a caller can skip work a denied principal must
not be able to induce (RR-X56H).

**Error handling:** A store error fails the whole read rather than returning a
partial slice — a truncated collection read is indistinguishable from a
genuinely small one.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

| AC | Test |
| --- | --- |
| AC1 | `TestScopedHeaders_IsTheOnlyVerdictSwitch` — regexp scan with an exemption map |
| AC2 | `TestScopedHeaders_NarrowingsApplyToBothVerdictBranches` — per-dimension table |
| AC3 | the existing data-entry suite, unmodified |
| AC4 | `TestScopedHeaders_ZeroVerdictIsRefused` |
| AC5 | `TestScopedHeaders_DoesNotMutateTheACLQuery` |
| (RR-X56H) | `TestScopedHeaders_DenyAllReportsWithheld` + the pre-existing `TestACLList_DenyAllSearchShortCircuit` |

**Integration test approach:** AC3 IS the integration test — the whole
data-entry suite exercises all four migrated surfaces end to end over HTTP, and
it must pass unmodified. A unit test of the funnel alone could not detect a
migration that dropped a narrowing at a call site.

**Edge cases:** zero `ReadQueryResult`; DenyAll; a denied world (blocks all
reads before the verdict switch); AllowAll with and without extra conjuncts
(different store call); shared `ReadQueryResult` across two calls.

**Negative tests:** the zero-verdict refusal and the no-mutation test are both
negative. The guard test is negative by construction — it fails when a SECOND
verdict switch appears.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** as documented on the ticket. The first — "an extraction that quietly
changes behaviour is worse than the duplication" — is the one that bit, twice,
during implementation:

1. `Props` were applied on the ACL-gated branch but not on AllowAll. This is
the EXACT bug class the funnel exists to prevent, reintroduced while writing the
function meant to prevent it. Caught by
`TestNextAction_ConditionPrefiltersReachTheStore`, and it is why AC2's test is
per-dimension rather than a single assertion.
2. The lost DenyAll early return (RR-X56H above).

Both were caught by pre-existing tests, which is AC3 working as designed.

Effort: M (confirmed).

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] ~~User-facing docs~~ (N/A: internal refactor, no API, config or CLI
surface changes — AC3 requires that no observable behaviour changed)
- [x] Code documentation: `scopedread.go`'s doc comment carries the rationale
that was previously restated by hand at four sites — the two historical bugs,
the withheld-vs-empty distinction, and the one sanctioned duplicate
(`planListPushdown`).

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: the design
was reviewed with the user at the point the survey was presented — they chose
"extract the funnel first, as its own ticket" over threading query scopes
through four sites)
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** The decision this ticket exists to record was made
conversationally: presented with the four-site survey and the two historical
fail-open bugs, the user chose extraction first rather than adding a third
narrowing dimension to the existing duplication. The extraction then proved its
own premise during implementation (see Risk Assessment). A `/code-review` is
still to be run before `done`.
