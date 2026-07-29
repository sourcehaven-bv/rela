---
id: TKT-DLYBWE
type: ticket
title: 'lua: bounded read API shape — uniform opts, ACL pushdown, errors raise (stage 1)'
kind: enhancement
priority: medium
effort: m
status: review
---

Stage 1 of DEC-IYHLNF. Splits out the half of TKT-YWDGZD that is NOT blocked on
store-side paging, so the API shape can land while `store.GraphQueryer` grows
`Limit`/`Cursor`.

## What lands

- **`rela.list_entities` is always bounded** at `maxReadLimit` (2000). No
spelling for "everything". `limit` lowers it, clamps above it, and `limit = 0`
is an ERROR rather than silently meaning unbounded (which is what it means on
`store.ListEntitiesPage` — a near-miss worth rejecting loudly).
- **Uniform `opts` table** across the read bindings, with the bare filter string
kept as an exactly-equivalent shorthand (it is the most common call in the tree
and a filter is genuinely worth a positional).
- **ACL pushdown**: the row gate becomes a `store.GraphQuery` composed from
`acl.Request.ReadQuery` instead of a per-batch `PermitsReadMany` probe.
- **Iterator errors RAISE** (TKT-FVQ4) instead of `break`-and-return-partial.
- **Unknown options raise** — this is what makes `cursor` *absent* rather than
inert, so stage 2 can add real paging without any script having depended on a
fake one.

## What does NOT land

`cursor` and `each_entity`. Blocked on the store; see DEC-IYHLNF staging. An
accepted-but-ignored cursor turns the idiomatic paging loop into an infinite
one, and an offset-backed stand-in corrupts under concurrent writes — a missing
parameter is a clear error, a fake one is a trap.

## Verification

Five mutations, each confirmed failing before revert:

1. Remove the ceiling clamp → `{limit = 999999}` got through.
2. Ignore unknown option keys → `{cursor = "abc"}` accepted.
3. Skip redaction on the pushdown's `AllowAll` branch → **`row TKT-1 leaked a
hidden property`**. This is RR-1W1G6K/RR-OXE47R caught by a live test.
4. Drop the iterator early-stop → 50 rows pulled for a limit of 10 (both the
row count AND the pull count fire, so slice-after-materialize cannot pass).
5. Restore the pre-TKT-FVQ4 `break` → truncated list with no error.

Live graph: 933 review-responses and 235 tickets returned, exactly matching
on-disk counts — no truncation at real scale. 119/120 validation rules pass (the
one failure is this ticket's own in-review merge gate).

`just lint`, `just lint-md`, `just arch-lint`, `just plimsoll` all clean; **no
new arch rules needed**. visibility coverage 89.2%.

## Note on a test defect found during the work

`TestListEntities_LimitLowersTheBound` initially passed the clamp case for the
wrong reason: `t.Parallel()` subtests resume AFTER the parent returns, so the
deferred ceiling restore ran first and they asserted against the real 2000.
Fixed by making those tests serial; `swapMaxReadLimit` now verifies the swap
took effect so the same mistake fails at setup rather than as a confusing
assertion miss.
