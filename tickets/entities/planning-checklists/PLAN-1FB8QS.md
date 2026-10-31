---
id: PLAN-1FB8QS
type: planning-checklist
title: 'Planning: GET /api/v1/_analyze runs thousands of statements per request on Postgres'
started: "2026-10-24"
completed: "2026-10-24"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Problem:** One `_analyze` request on atlas issues about 12,000 statements (see
the ticket's measurements). The count grows with entities × rules × role
relations. No single statement is slow.

**Scope:**
- In:
  - A. Prime local roles per page on the pushdown list path
(`visibility.listPushdown`), as `PolicyReader.Filter` already does. This fixes
every gated `ListEntities` caller, not only analyze.
  - B. `analyzeValidations`: stop loading violations once the section holds
one issue past the cap, and resolve each rule's violations in one gated batch
(`ResolveHeadersErr`) instead of one `GetAddress` per violation.
  - C. `analyzeOrphans`: the same batch for its up-to-101 ids.
  - D. Budget tests that pin the statement count of these paths.
- Out (follow-ups, to be filed separately):
  - pgstore: the world-ranked single-id load costs about 0.8 ms of planning
per call (OR/CASE predicate, 39 indexes on `entities`). Affects every
`GetAddress`, also outside analyze.
  - atlas: rewrite the two procedure Lua rules (`get_relations` +
`get_entity` per edge) as declarative `related(...)` conditions.
  - SPA: the dashboard waits for `_analyze` before it renders its cards, and
only shows two (capped) counts.
  - Loading validation candidates once per type instead of once per rule.
With A in place a reload costs one list page, so the gain is small.
  - Skipping the remaining rules once the Validations section is full (see
open questions).

**Acceptance Criteria:**
1. On the atlas-dev copy as jvloothuis, `_analyze` issues at most 2,300
statements (develop: 12,046), and the response is byte-identical to develop.
2. A gated `ScriptReader.ListEntities` over N rows of a type with a
`role_relations` policy issues the same number of statements at N=10 and N=50,
and no `GetRelation`.
3. `_analyze` (no Lua rules, ACL with a conferring role relation) issues the
same number of statements at 10 and 50 entities per type.
4. ACL behaviour is unchanged: a hidden entity that violates a rule or is an
orphan produces no issue; a visible entity whose display property is hidden is
titled by its id.
5. A Validations section with more than 100 violations still reports
`Truncated` and exactly 100 issues.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: a performance fix on existing paths; the cause was measured, not chosen between options)
- [x] ~~Searched for existing libraries that solve this problem~~ (N/A: internal query pattern)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A.

**Existing Solutions:**
- `acl.PrimeLocalRoles` / `batchLocalRoles` (`internal/acl/localroleprime.go`,
BUG-E2W1LS): batch local roles per page. Reached through
`visibility.PrimeTraversals`.
- `PolicyReader.Filter` and `FilterHeaders`
(`internal/visibility/policyreader.go:69,109`) prime before redacting;
`listPushdown` (`internal/visibility/pushdown.go:158`) does not.
- `Resolver.ResolveHeaders(Err)` (`internal/visibility/batch.go`): gated,
redacted batch read whose cost does not depend on the number of refs.
- `safeHeaderTitle` (`internal/dataentry/analyze.go`) is the header twin of
`safeDisplayTitle`.
- `.claude/rules/collection-reads.md`: batch per page, pin with a
`storetest.Counting` budget test at 10 and 50 rows.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**
- A. `listPushdown` takes the reader's primer next to its row redactor
(`PolicyReader` exposes `PrimeRows(ctx, rows) ctx`, wrapping
`PrimeTraversals(ctx, r.redact, rows)`). `redactingSeq` buffers rows up to the
store page size (500), primes the chunk, redacts and yields it, then continues.
Errors still end the sequence after the buffered rows. A failed prime keeps the
per-entity probes, so the result cannot change. Prototype: 12,046 → 3,356
statements, identical output.
- B. In `analyzeValidations`, per rule: take only as many violations as the
section still has room for (cap + 1), resolve them with one `ResolveHeadersErr`
call on the gated reader, and build issues from the redacted headers with
`safeHeaderTitle`. A miss is dropped as today; when misses leave room, resolve
the next slice. A resolve error fails the section loudly (like cardinality)
instead of reading as "no violations". Script and load errors are still
collected for every rule. Prototype (A+B): 2,207 statements, identical output.
- C. `analyzeOrphans` resolves its sorted ids in slices of cap + 1 with the
same call.
- `analyzeReader` gains `ResolveHeadersErr`; `lateGatedReader` forwards it.
`GetAddress` leaves the interface if nothing else uses it.

**Alternatives:**
- Use the candidate entities the validator already loaded and redacted,
instead of resolving again: fewer statements, but it couples analyze to
validator internals and changes the `Validator` interface. The batch re-read
keeps the gated re-load that TKT-3FL2S6 relies on.
- Memoize `computeForEntity` on the `Request`: rejected; the prime is
deliberately ctx-scoped so a write never decides on a stale answer
(`localroleprime.go`).
- Cache the whole analysis result: rejected; results are per principal
(gated) and would be stale after any edit.

**Dependencies:** none new. Packages touched: `visibility`, `dataentry`.

**Files to modify:**
- `internal/visibility/pushdown.go`, `internal/visibility/luareader.go`,
`internal/visibility/policyreader.go` (fix A)
- `internal/dataentry/analyze.go`, `internal/dataentry/app.go` (fixes B, C)
- Tests: `internal/visibility/pushdown_test.go` (or a new
`pushdown_budget_test.go`), `internal/dataentry/analyze_budget_test.go` (new),
`internal/dataentry/acl_analyze_test.go`,
`internal/dataentry/analyze_cap_test.go`

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** No new input. The endpoint takes no parameters;
`acl.yaml` and `schema.yaml` are loaded and validated as today.

**Security-Sensitive Operations:**
- Field redaction (A): the prime only answers the same local-role question in
a batch, for the Request bound at the edge (`PrimeLocalRoles` checks the
binding). Each row is still redacted.
- Gated re-load (B, C): `ResolveHeadersErr` runs the same row gate, face gate
and redaction as `GetAddress` (godoc, `batch.go`). Titles come from redacted
headers through `safeHeaderTitle`, so a partly hidden title still falls back to
the id.
- Errors: a resolve failure becomes a generic section error; the cause goes to
the server log only.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**
1. AC1: manual run of `.ignored/scripts/bench.sh` against the atlas-dev copy,
before and after, plus `cmp` of the two responses.
2. AC2: `storetest.Counting` budget test on a `ScriptReader` with a policy
that has a conferring role relation: equal counts at 10 and 50 rows, zero
`GetRelation`. Runs on memstore; the pgstore variant runs in the DB-gated job.
3. AC3: `_analyze` budget test through the HTTP handler with rules that use
properties and `related(...)` (no Lua), equal counts at 10 and 50.
4. AC4: extend `acl_analyze_test.go`: hidden violator and hidden orphan
absent; hidden display property titled by id.
5. AC5: `analyze_cap_test.go` with 150 violations: 100 issues, Truncated set.

**Edge Cases:**
- Type with more rows than one chunk (501, 1,000): every row redacted, order
kept.
- Consumer stops early (`break` in range): no further page loaded, no panic.
- Store error mid-stream: rows before it are yielded, then the error.
- Prime fails (graph without `EdgesBetween`): per-entity probes, same result.
- Violation id that no longer exists, or a faced type: dropped / resolved in
the reader's world, as `GetAddress` does today.
- Section reaches the cap in the middle of a rule.

**Negative Tests:** `ResolveHeadersErr` returns an error → the Validations
section reports "could not be checked" instead of fewer issues.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl): m

**Risks:**
- Chunk buffering in `redactingSeq` holds up to 500 entity bodies. The store
already fetches 500-row pages, so peak memory does not grow. Mitigation: a
`storetest.BodyWatch` / heap check in the budget test.
- Header-based titles could differ from entity-based titles. Mitigation: the
title is built from properties only; the prototype output was byte-identical,
and AC4 pins the hidden-property case.
- Production may still be slow after A–C: about 1,500 statements remain,
mostly atlas Lua rules, and the per-`GetAddress` planning cost remains.
Mitigation: the out-of-scope follow-ups above.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:** N/A: internal performance change. Godoc on
`listPushdown` / `redactingSeq` and on `analyzeReader` (which still says the
per-id `GetAddress` stays) is updated.

## Design Review

- [x] Run `/design-review` before starting implementation (run after the user answered the open questions)
- [x] All critical/significant findings addressed in plan (one deferred to the user, see below)

**Design Review Findings:** 13 findings, recorded as review-responses on
TKT-QM833U. Eleven are addressed. Two are deferred:

- Running every rule after Validations is full gives no visible output:
  `capIssues` cuts every issue past 100, so later script and load errors do
  not reach the wire. Open question 1 below claimed the opposite. The
  approved behaviour is kept and the finding goes back to the user.
- Per-face violations report the bare id twice. Same as develop; follow-up.

Plan changes from the review: page sizes 64 then 512 instead of a 500-row
buffer; `primePage` primes local roles itself; a re-load failure is a
per-rule error issue; `Served()` is checked; buffered rows are dropped on
error.

## Open questions

1. Once the Validations section holds 101 issues, should the remaining rules
still run? Running them keeps later script and load errors visible (2,207
statements, about 0.85 s locally). Skipping them gave 708 statements and about
0.22 s, with identical output on atlas-dev, but would hide a broken rule that
sorts after the cap.
2. Should the pgstore planning cost and the dashboard coupling be follow-up
tickets, or part of this one?
