---
id: PLAN-KQCSH3
type: planning-checklist
title: 'Planning: _nav_status and _nav_items probe local roles per row (slow on atlas postgres)'
started: "2026-10-24"
completed: "2026-10-24"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**What the endpoint computes per request.** `handleV1NavStatus`
(`internal/dataentry/nav_status.go`) walks every sidebar entry with `status:`
rules in the requested space. Per entry, `navStatusRows` reads the whole list
through the Go list pipeline (`scopedSortedEntitiesScoped`: every header of the
type under the ACL row gate, then `filters:` and sort in Go), then
`applyViewCondition` for the list's `condition:`, then one more
`applyViewCondition` per rule over the kept rows. `applyViewCondition` redacts
every candidate row first (`redactedForSuggestion` → `copyVisibleProperties` →
`FieldVerdicts` → `acl.Request.ForEntity`).

**Root cause.** `navStatusRows` never calls `primeVerdicts` before those
redactions. `listPage` does (`api_v1.go:398`), and BUG-E2W1LS added the prime
for exactly this cost. Unprimed, `ForEntity` runs `localRoles`, which probes
`aclstore.Graph.HasEdge` → `pgstore.GetRelation` once per (role relation ×
member of the principal's closure × ancestor) per row. Goroutine dump confirms
the stack: `handleV1NavStatus → navStatusFor → navStatusRows →
applyViewCondition → redactedForSuggestion → … → localRoles → HasEdge →
GetRelation`.

Atlas has one status entry per space that matters: "Mijn taken" (list
`mijn_taken`, `taak` with `status != gereed`, condition `related(entity,
'heeft_verantwoordelijke', { id = current_user.id })`, two rules on
`vervaldatum`). So every request redacts every open task of every user, about 9
point lookups per task.

**Why "most time is in Go".** The access log's `db_ms` is summed in the pgx
query tracer, which starts after the pool hands out a connection. Time spent
waiting for a connection counts as Go time. Atlas runs on a cx23 (2 vCPU) and
`RELA_DATABASE_URL` sets no `pool_max_conns`, so pgxpool's default is max(4,
NumCPU) = 4 connections. A few concurrent `_nav_status` requests, each issuing
~870 sequential statements, occupy all four connections and every other request
queues behind them. Under load locally, 13 of 16 handler goroutines were parked
in `pgxpool.(*Pool).Acquire`; `db_ms` dropped to 18 % of wall time.

**Why it is called so often.** `useNavStatus` refetches on every route change
and, debounced by 1 s, after every `entity:changed` SSE event of ANY type, in
every open tab. One write therefore fans out to one `_nav_status` per open tab.

**Contention with the other slow endpoints.**
- `/api/v1/taken`: the `mijn_taken` list request alone is cheap (14 statements,
~5 ms; it primes). It suffers as a bystander of the exhausted pool.
- `/api/v1/_analyze` (called by every dashboard load): its own N+1 of the same
kind, 11,234 statements per request (8,274 `GetRelation`), via
`validator.loadCandidates → visibility.PolicyReader.RedactRow` (a streaming
per-row redaction with no prime). 1.4–1.8 s alone; 5–6 s p50 and up to 9.4 s
while `_nav_status` runs at 40 req/s on a 4-connection pool.
- `/api/v1/_nav_items` (same sidebar, same refetch triggers): 184 statements,
168 of them the same unprimed `GetRelation` (`navItemsFor` redacts rows from
`listPage` without a prime). Bounded by the item limit, not by the type.
- No shared Go lock was found on these paths. `store.Tx` (write mutex /
advisory lock) is not taken by any of them. The shared resource is the pgx pool.

**Scope (IN):**
1. Prime local roles and grant traversals in `navStatusRows` before the list
condition and keep the primed ctx for the rule passes.
2. The same one-line prime in `navItemsFor` before it redacts (same sidebar,
same trigger, same defect).
3. A budget test for `_nav_status` and `_nav_items` under the realistic ACL
harness (`readsFor`), so the count cannot grow with rows again.

**Scope (OUT), proposed as follow-up tickets:**
- `_analyze` N+1 in the validator's streaming redaction (different shape: it
needs a per-page prime inside `visibility.listPushdown.redactingSeq`).
- Pushing a list `condition:` that lowers exactly (`queryplan.LowerScope`) into
the store, so `_nav_status` and the list stop reading the whole type. At atlas's
size (~155 tasks) this is not the bottleneck; at 20× it is ~50 ms.
- A larger default pool size in rela, or `pool_max_conns` in the atlas DSN
(devops `roles/atlas`). An operator mitigation; it does not remove the N+1.
- SPA: recount only for `entity:changed` events of relevant types. Risky,
because a rule or condition may traverse relations of other types.
- Caching counts. Rejected, see Approach.

**Acceptance Criteria:**
1. `_nav_status` issues a number of store reads that does not depend on the
number of rows in the list, under an ACL with role relations and inheritance.
Test: budget test at 10 and 50 rows, equal counts.
2. `_nav_items` likewise. Test: same harness.
3. Responses are unchanged. Test: the existing `nav_status_test.go` and
`spaces_test.go` cases pass unchanged; a new case with a local role that confers
a `visible:` grant gives the same count primed as unprimed.
4. On the local atlas copy (postgres, GOMAXPROCS=2, pool 4) a single
`_nav_status` costs at most ~15 statements (was 867) and the response body is
byte-identical. Test: manual run with `--access-log`, recorded in the
implementation checklist.

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A (small change; measurements are in the ticket).

**Existing Solutions:**
- `acl.PrimeLocalRoles` / `primeVerdicts` (`internal/acl/localroleprime.go`,
`internal/dataentry/affordances.go:307`), added for BUG-E2W1LS. Used by
`listPage` (`api_v1.go:398`), export (`export_list.go`, via
`visibility.PrimeTraversals`), next actions and sections.
- Budget harness `readsFor` + `realisticBudgetPolicy`
(`internal/dataentry/querybudget_test.go`), which already fails when reads grow
with rows under role relations.
- No library needed.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**
1. `navStatusRows`: after `scopedSortedEntitiesScoped` and before the list
`applyViewCondition`, `ctx = primeVerdicts(ctx, a.fieldResolver, rows)`. Return
that ctx (the function already returns one), so the rule passes in
`navStatusFor` read the same prime. Prime unconditionally, not only when the
list has a condition: a rule condition redacts the rows too.
2. `navItemsFor`: `ctx = primeVerdicts(ctx, a.fieldResolver, rows)` before the
redaction loop.
3. Budget tests for both endpoints with `readsFor`.

**Alternatives considered:**
- Prime inside `applyViewCondition`. Rejected for now: it would need a primer
argument on every caller, and it returns rows, not ctx, so the rule passes would
prime again. The explicit call matches `listPage`.
- Load all of the principal's conferring edges once per `acl.Request`, so no
path needs a prime. Rejected: the prime is deliberately ctx-scoped so a write
never decides on a snapshot taken before it (`localroleprime.go` doc). Changing
that is a larger ACL design change.
- Cache counts per principal (key: principal, space, store sequence, config
generation). Rejected: refetches are triggered BY writes, so the key changes on
almost every refetch; the root CLAUDE.md forbids caching aggregates across
principals; and after the fix a request costs ~5 ms.
- Incremental maintenance of counts. Rejected: counts are per principal and
depend on `today()`, ACL and conditions; far more machinery than the problem
needs.
- Push the list condition into the store. Useful later, out of scope (above).

**Dependencies:** none new.

**Files to modify:**
- `internal/dataentry/nav_status.go`
- `internal/dataentry/nav_items.go`
- `internal/dataentry/querybudget_test.go` or a new
`internal/dataentry/nav_status_budget_test.go`
- `internal/dataentry/nav_status_test.go` (equality case with a local role)

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** `?space=` (resolved by `resolveSpace` against
permitted spaces, unchanged) and operator config. No new input.

**Security-Sensitive Operations:** row gating and field redaction. The prime is
documented to make a check cheaper, never different: a failed batch falls back
to per-entity probes. The prime is bound to the request's `acl.Request` and
ignored for any other. The endpoint does no writes, so the "snapshot before a
write" concern of `WithoutLocalRolePrime` does not apply. Redaction still
happens before any count (aggregate rule in the root CLAUDE.md).

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**
- AC1/AC2: `readsFor` ops calling `handleV1NavStatus` and `handleV1NavItems`
on a list with a `related(... current_user ...)` condition and two rules. The
harness fails if reads differ between 10 and 50 rows under the realistic ACL.
- AC3: existing nav status and spaces tests; new case where a role conferred
by a relation unlocks a `visible:` field a rule condition reads. Count must be
the same as without the prime (run once with a non-batching graph or compare to
the list endpoint's total for the same list).
- AC4: local atlas copy on postgres, before/after, access log.

**Edge Cases:**
- Empty list: `primeVerdicts` returns ctx unchanged for zero rows.
- List without condition, rules with conditions: still primed.
- Principal with no role relations: batch is a no-op.
- Prime batch read fails: warning logged, per-entity probes answer, same
result.

**Negative Tests:** an entry whose list is unknown or whose condition is not
compiled is still skipped as today (existing tests).

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**
- The fix removes the N+1 but not the whole-type read; a much larger
deployment still pays O(rows) in Go. Mitigation: follow-up pushdown ticket.
- `_analyze` keeps loading the pool on every dashboard load until its own
fix. Mitigation: follow-up ticket; `pool_max_conns` in the atlas DSN as an
interim operator step.

**Effort:** s for the in-scope fix (the ticket carries m to leave room for the
`_nav_items` case and tests).

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:** N/A. Internal performance fix, no behaviour change.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** self-review, no review-response entities. Checked:
prime staleness (read-only endpoint, not applicable), result equality (AC3),
fallback on batch error (same result), rule passes reusing the primed ctx
(approach step 1), redaction before count (unchanged). No critical or
significant findings.
