---
id: PLAN-VZXHRJ
type: planning-checklist
title: 'Planning: lua: rela.list_entities has no limit/paging — unbounded materialization, worsened by visibility filtering'
status: done
---

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

IN:
- Route `rela.list_entities` through the ACL-composed `store.GraphQuery`
(`acl.Request.ReadQuery`) instead of post-filtering with `PolicyReader.Filter`.
- **`list_entities` is ALWAYS bounded.** A default cap applies; `limit` may
lower it but never remove it. There is no "give me everything" spelling.
- Expose `count` / `total` / `truncated` so a capped result is never silent.
- Fold in TKT-FVQ4 for the SAME loops: surface iterator errors instead of
`break`-and-discard.
- `rela.get_relations` gets the same error handling (TKT-FVQ4). Its bounding is
deferred — relations have no `GraphQuery` equivalent, so it is a different
problem, not a copy of this one.

OUT:
- Store-side paging. `GraphQueryer` has no `Limit`/`Cursor` — handed to the
store dev separately. This ticket is written so adopting it later is a
one-call-site change (see Approach).
- `rela.search` (already bounded at 20).
- Backwards compatibility. Explicitly waived by the user (2026-07-28/29):
"I dont care about backwards compat", "there should be only a bounded version -
bound can be relatively high".

**Acceptance Criteria:**

1. With no ACL policy, `list_entities("ticket")` returns the same rows it does
today **up to the cap**. Test: existing suite + explicit parity assertion.
2. With a policy, a hidden entity is absent — and absent because it was never a
candidate, not because it was filtered after. Test: a `GraphQueryer` spy
asserting the ACL predicate reached the store, plus zero `PermitsReadMany` calls
on the list path.
3. The default cap applies with no `limit` given. Test: seed cap+1 (with the
`var` lowered), assert `count == cap` and `truncated == true`.
4. `limit` lowers the cap but cannot raise it above the default. Test:
`limit = cap*2` still yields at most `cap`.
5. Truncation is never silent: `truncated == (total > count)`, `total` clamped
up to `count`. Test: includes a deliberately under-reporting stub.
6. A mid-iteration store error RAISES rather than returning a short list
(TKT-FVQ4). Test: reader yielding N rows then `context.Canceled`; assert the
script sees an error, not `count=N`.
7. `get_relations` likewise raises rather than returning `0, nil` on failure.
Test: the empirical repro already written for TKT-FVQ4.
8. Bad `limit` (negative, non-number) raises naming the offending type, per the
TKT-9FKX8X shape. Test: table-driven.
9. **Field redaction survives the pushdown (RR-1W1G6K).** A `visible:`-hidden
property is absent from every row `list_entities` returns. Test: the existing
ACL fixtures, asserting on properties not just row identity — the old test
suite gated rows, so it would pass against an un-redacting implementation.
10. **Redaction applies on the `AllowAll` branch too (RR-OXE47R).** Principal
with global read + a hidden field sees the row WITHOUT the field. Dedicated
test, not shared with the Query branch.
11. **`total` never exceeds what the principal may see (RR-SSPCCI).** With N
visible of M total (M > N), `total == N`. Test: asserts the raw type count is
NOT observable through the binding.
12. **A gate/store failure raises (RR-4DUSO1)**, rather than yielding an empty
list that reads as "you may see nothing".

## Research

- [x] ~~For larger features: run `/research`~~ (N/A: the design question was
settled by reading two shipped precedents; no option space left to survey)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A — prior art in-tree was decisive.

**Existing Solutions:**

- **`acl.Request.ReadQuery`** (`internal/acl/readquery.go:27`, exported at
`request.go:92`, documented "for list-style reads"). Returns `AllowAll | DenyAll
  | *store.GraphQuery` — **candidate-independent**. This is the whole basis of the
approach: the ACL is a predicate over a type, not a post-filter over rows.
Already used by SIX dataentry call sites (`api_v1.go:288`, `:2196`,
`readgate.go:83`, `:99`, `feed_handler.go:130`, `watcher.go:504`). **The Lua
binding is the outlier that never adopted the established pattern** — this is
adoption, not invention.
- **`store.GraphQuery`** godoc names "ACL read filtering" as an intended
consumer. pgstore is SQL-native (recursive CTE + `WHERE EXISTS`);
fsstore/memstore use `graphquerynaive`. NOTE: `store/graphquery.go:17` claims
SQL pushdown is "a future follow-up" — **stale, it shipped**.
- **TKT-95XU13 / PR #1241** — `internal/lua/listmode.go`: `Len()/At(i)` over
`iter.Seq` (a script may walk twice; `iter.Seq` is single-shot), stateful
closure for the Lua-side iterator, `truncated` DERIVED from `total > count`, "do
not memoize" as an explicit instruction, cap applied post-gate.
- **`listExportCap = 5000`** (`export_list.go:22`) — the cap value and its
`var`-not-`const` rationale are both reused here; see Approach.
- **`PolicyReader.Filter`** (`policyreader.go:66`) — the thing being replaced on
this path. Batch-shaped, one probe per TYPE; paging it naively would have turned
1 ACL query into N. Avoided entirely by pushing down.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

```
ReadQuery(ctx, type) → AllowAll → GraphQuery(unfiltered type query)
                     → DenyAll  → empty result
                     → Query    → GraphQuery(*rqr.Query)   ← ACL IS the query
```

then **gate → cap → redact**, exposing `count`/`total`/`truncated`.

**The pushdown replaces the ROW GATE ONLY (RR-1W1G6K, critical).**
`PolicyReader.Filter` does two jobs — the batched row gate AND `redacted()` on
every survivor. `store.GraphQuery` expresses only the first; it has no concept
of field-level `visible:` policy. Dropping `Filter` wholesale would return every
hidden property to scripts — the exact #1188 CISO finding this arc exists to
close. **Redaction still runs per returned row, on every branch including
`AllowAll` (RR-OXE47R).** A principal with global read on a type may still be
denied individual fields; "may read every row" is not "may see every field".

**Order is gate → cap → redact (RR-DSINTY)**, not gate → redact → cap.
Redacting first pays a per-row copy for up to `(matched - cap)` rows that are
then discarded. Both orders produce identical OUTPUT, so no test will catch
this — it has to be specified.

**`total` must be the count of VISIBLE rows, never the raw type count
(RR-SSPCCI, significant).** `GraphCount` returns `(matched, total)` where its
`total` deliberately IGNORES the predicates — and the predicates ARE the ACL.
Reporting that to a script is a count-based existence oracle over hidden rows,
violating "hidden is indistinguishable from nonexistent" (DEC-ZBI39P). Use
`matched`; `truncated` is `matched > count`.

Revised benefit claim: this removes the `PermitsReadMany` probe and the
short-page problem. It does NOT remove the per-row redaction copy, and (until
store paging lands) does not remove the full-type materialization either.

**Cap = 5000**, matching `listExportCap`. Rationale, not a round number: rela's
own largest dogfood type is 909 (review-responses), so 5000 is ~5.5x real-world
headroom; and this codebase already chose 5000 for the *same* problem (bound a
whole-type read so a huge graph can't OOM). Two different high caps for one
concept would drift. Declared as a `var` for the same stated reason #1241 gives
— so tests can lower it without seeding thousands of rows.

Why bounded-only, no escape hatch:
- The unbounded path being the DEFAULT is the defect. Opt-in safety leaves the
footgun as the path of least resistance.
- An unbounded *option* re-creates the same problem for anyone who reaches for
it, and would have to be supported forever.
- With `count`/`total`/`truncated`, a capped result is honest — a script can
detect it and react, which is strictly better than a silent full scan.

Why this and not the alternatives:
- **Rejected: page `ListEntitiesPage` + `MatchingIDs` per page.** No store
change needed, but one ACL query per page AND pages come back short after
filtering — reintroducing the refill-vs-report problem #1241 deliberately
avoided. Pushing the ACL down dissolves both.
- **Rejected: `iter.Seq` for the row seam.** Single-shot; #1241 documents why a
script must be able to walk twice.
- **Rejected: unbounded default / `limit = 0` escape.** Superseded by the user
decision above.

**Interim limitation, stated honestly:** without store-side paging the
unfiltered `GraphQuery` still materializes the type before the cap. So this
bounds LUA memory and fixes CORRECTNESS (ACL as predicate, no short pages,
visible truncation) — the headline 3x Go-side allocation in the ticket summary
does NOT close until `GraphQueryPage` lands. Do not mark that part done.

**The later refactor is one call site:** `GraphQuery(...)` →
`GraphQueryPage(..., limit, cursor)` + delete the post-cap slice. The ACL
composition, branching, truncation reporting, Lua surface and tests all survive
untouched. That is why proceeding now is safe.

**Files to modify:**
- `internal/lua/runtime.go` — `luaListEntities` (rewrite), `luaGetRelations`
(error handling only)
- `internal/lua/deps.go` — the read seam needs graph-query access; extend the
consumer-side interface rather than passing `*acl.Declarative` (keeps
"interfaces at the call site")
- `internal/visibility/` — the reader gains the pushdown path
- wiring: `appbuild`, `dataentry` script deps
- in-tree scripts that must now handle the cap (see Risks)
- `docs-project/entities/guides/GUIDE-lua-scripting.md`

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- `type` (script-supplied): already `CheckString`-validated, raises on non-string.
- `limit` (script-supplied): must be a positive integer, clamped to the cap.
Reject negatives and non-numbers explicitly — TKT-9FKX8X just fixed the exact
silently-ignored-bad-argument bug in the sibling binding; do not reintroduce it.
**0 is NOT "unbounded"** here (it is in `ListEntitiesPage`) — that divergence
must be documented at the binding or it will be assumed.
- `filter` (script-supplied): unchanged, existing parser.

**Security-Sensitive Operations:**

- **This IS the read-ACL path.** The load-bearing property: the ACL predicate
must reach the STORE, so an entity the principal cannot read is never a
candidate. A regression here is a read-ACL bypass, not a perf bug.
- `AllowAll`/`DenyAll` must be handled explicitly. A missed `DenyAll` returning
an unfiltered list is the worst failure mode available — test it directly.
- Fail-closed on a nil/zero `ReadQueryResult` (mirroring `PermitsReadMany`,
which errors on that state). Never fall back to unfiltered.
- Errors from the gate must RAISE, never degrade to an empty list — an empty
list reads as "nothing here", which is precisely the TKT-FVQ4 defect.
- The cap is also a DoS bound: a hostile script can no longer force an
unbounded materialization by naming a large type.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** AC1-8 above each name their test. Integration: run the real
120-rule validation suite and `scripts/dev-status.lua` against the live ticket
graph — the same end-to-end check that caught the broken docs example in
TKT-9FKX8X. With 909 review-responses the cap is NOT hit at 5000, so the
integration run also proves the cap doesn't disturb normal operation.

**Edge Cases:**
- `limit` = 1, exactly cap, cap+1, negative, zero, non-integer, huge.
- Empty type; unknown type; policy hiding EVERY row (empty + coherent `total`,
not an error).
- `AllowAll` and `DenyAll` branches (no `GraphQuery` built at all).
- Script walking the result twice.
- Under-reporting `total` → clamp holds.
- Exactly-at-cap: `truncated` must be FALSE at count == total == cap (classic
off-by-one; `listExportCap` uses `>` not `>=` — match it).

**Negative Tests:**
- Store error mid-iteration → Lua error, NOT a short list (AC6).
- `get_relations` failure → Lua error, NOT `0, nil` (AC7).
- Gate error → raise, not empty.
- Bad `limit` → raise naming the offending type (AC8).

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

1. **Every whole-set caller is now silently capped.** VERIFIED, not assumed:
`scripts/dev-status.lua` counts entities (and lists review-response, the 909-row
type), `scripts/generate-docs.lua` renders ALL entities of each type,
`tickets/scripts/related.lua` scans for similarity, plus `stale-review.lua` and
`examples/view-affected.lua`. At 5000 none of them hit the cap today, but
`generate-docs.lua` silently omitting entities is a real future failure. →
**Mitigation: audit each in-tree caller and make it check `truncated`**,
erroring loudly rather than producing partial output. A doc generator that
quietly drops pages is worse than one that fails. This is now IN scope; it was
the part backwards-compat was protecting.
2. **Read-ACL regression** (see Security). → Mitigation: dedicated tests for
AllowAll/DenyAll/Query, plus a spy asserting the predicate reached the store.
3. **Behavior drift between `GraphQuery` and the old `Filter` path** — a policy
shape one expresses and the other does not. → Mitigation: parity test over the
existing ACL fixtures; if any case diverges, STOP and report rather than paper
over it. This is the one that could quietly change who sees what.
4. **Interim memory expectation.** The ticket's headline is 3x allocation; the
interim does not fix it. → Mitigation: stated in Approach and to be repeated in
the PR description. Do not claim the memory win until paging lands.
5. **5000 may be wrong for a bigger deployment.** → Mitigation: `var`, so it is
trivially tunable; a config surface is explicitly v2, matching the same call
   #1241 made for `listExportCap`.

**Effort:** m → **l**. The in-tree caller audit (risk 1) is new work that the
backwards-compat assumption previously avoided.

## Documentation Planning

- [x] User-facing docs identified
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
- `GUIDE-lua-scripting.md` — the cap and its value, `limit`, the
`count`/`total`/`truncated` triple, that `limit = 0` is NOT unbounded, and that
errors now raise instead of truncating. **This is a breaking change to a
documented binding and must be called out as such**, not slipped into a table.
- Not metamodel/CLI/data-entry/README.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** RR-1W1G6K (critical), RR-SSPCCI (significant),
RR-OXE47R (significant), RR-4DUSO1 (minor), RR-DSINTY (minor).

All five addressed in Approach + AC9-12 above before implementation started.

The critical one is worth stating plainly: **the first draft of this plan would
have reintroduced the #1188 CISO finding.** It proposed replacing
`PolicyReader.Filter` with a `GraphQuery` pushdown without noticing that
`Filter` also performs field redaction, which `GraphQuery` cannot express. The
plan read as a performance change and was actually a read-ACL regression. That
is the single strongest argument for running `/design-review` on plans that
touch the ACL, and for AC9 asserting on PROPERTIES rather than row identity —
the pre-existing row-gate tests would all have passed.
