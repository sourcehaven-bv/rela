---
id: IMPL-845VR0
type: implementation-checklist
title: 'Implementation: Declarative webhook routes: map an inbound HTTP request onto entity create / find-or-create / update'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

All three workflows driven end-to-end through the REAL production router
(`app.NewRouter().ServeHTTP`), not the handler directly —
`TestWebhookRoutes_ThreeWorkflows`:

- always-create: two identical payloads produce 2 entities (no accidental dedup)
- find-or-create: first delivery `created`, second `updated`, 1 entity, both
notifications accumulated in one body
- find-and-update-only: a miss returns `no_match` and creates 0 entities; after
seeding, the same delivery updates

**Concurrency verified against REAL PostgreSQL 15.17**, schema-pinned DSN
(`options=-c search_path=<schema>,public`, per the CLAUDE.md rule that the bare
DSN resolving to `public` is the one case that trivially works). File:
`internal/dataentry/webhook_conflict_postgres_test.go`, `//go:build postgres`,
run under `-race`, all passing:

- `ConcurrentCreateLosesOnUnique` — 8 goroutines released together create with
the SAME `unique:` value: exactly 1 wins, 7 get `store.UniquePropertyError`,
exactly 1 row stored. This is the derived partial unique index
(`rela_derived_uniq__*`) doing the work; `Reconcile` is invoked in the test so
the index actually exists.
- `LoserRefindsAndProceeds` — the loser's re-find locates the winner, so the
delivery proceeds as an update rather than being dropped.
- `PipelineAppendsAllLand` — 6 concurrent deliveries through the production
router against pgstore; all 6 notifications present at the end.
- `BlindUpdateLosesAppends` — characterizes WHY the body is re-read: blind
concurrent `UpdateEntity` calls silently lose writes (no compare-and-swap).
- `SchemaPinnedDSNIsIsolated` — proves the harness is genuinely pinned (schema B
cannot see schema A's row; the row is not in `public`).

**Security requirements:**

- Body cap — `TestWebhookRoutes_BodySizeCap`: oversized body → 413 and **0
entities written** (asserted, because a truncated form body would parse cleanly
and store a quietly-wrong entity). Under-cap body still succeeds.
- Header allowlist — `TestWebhookRoutes_HeadersAreAllowlisted`: allowlisted
header resolves; `Authorization` / `Cookie` / non-listed `X-Secret` all resolve
to empty, plus a belt-and-braces scan asserting no secret value appears anywhere
in the stored entity.
- Forbidden header names refused at config load —
`TestValidateWebhooks_ForbiddenHeadersRefused` (incl. lowercase, so case cannot
evade).
- `PatchEntity` not read-modify-write —
`TestWebhookRoutes_PatchPreservesUnnamedProperties`: a `set:` step leaves
unnamed properties and body intact.
- Route reachability — `TestWebhookRoutes_ReachableThroughRouter`, using the
BUG-F3ADZO oracle ("not the SPA shell"), since `/hooks/` is not an `/api/` path
and an unregistered route would answer 200 HTML.

**Honest limitation, reproduced not hidden:**
`TestWebhookConflict_CrossProcessAppendsCanBeLost` — two Apps on separate
pgstores over one schema (the documented multi-writer deployment). Observed **1
of 2 appends landed**. `writeMu` is per-process and nothing in the append path
is a compare-and-swap across processes. The test asserts the weaker property
that actually holds and logs the loss; `docs/webhooks.md` states it plainly in
the guarantees table. Server-side append mode on `entity.Patch` is the named
follow-up.

**Not verified:** nothing was left unverified due to a missing database —
PostgreSQL was available and used. The fs/memstore tiers cannot exhibit the
create race at all (single-writer), which is precisely why the postgres tests
above exist rather than fs-tier stand-ins.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

**Gates:** `go test ./...` full suite passes (zero failures); `go test -race
./internal/dataentry/` passes (271s); `just arch-lint` OK; `just lint` 0 issues;
`just comment-lint` clean; `just plimsoll` OK; `just coverage-check` PASS
(package floor + total, 78.5%).

**Design decisions made on the ticket's open questions:**

- *sha256 encoding* — lowercase hex. Matches the existing `crypto.sha256_hex`
Lua binding, matches Icinga DB's equivalent content-hash key, and is unambiguous
in URLs/filenames/logs/SQL. Pinned by NIST/RFC vectors AND a structural shape
test, since the value is stored+indexed and the choice is effectively permanent.
- *Missing section* — created (`## <name>` at end of body), not an error. An
error discards an alert from a producer that does not retry; an unplanned
heading is visible and trivially edited. Also removes the need to keep the
entity template and hook config in sync.
- *Retry budget* — 4 attempts, then 409. Contention requires two deliveries
concerning the same entity in one request window; a loser that has re-read fresh
state succeeds next attempt essentially always.
- *Response semantics* — 2xx configurable via `respond.status` (validated as
2xx); 400 malformed body, 404 unknown hook, 409 retries exhausted, 413 over cap,
500 pipeline failure, 504 timeout. Errors never echo internals (they can carry
stored property values).

**Architecture notes:**

- `internal/markdown` added to dataentry's `mayDependOn` in `.go-arch-lint.yml`
with a written justification: markdown is a near-leaf (depends only on
`frontmatter`) and dataentry already parses markdown with goldmark directly, so
this reuses the reviewed parser rather than hand-rolling a second heading
scanner.
- The pipeline lives on a focused `webhookRouter` type, NOT as 7 more methods on
`App` — `App` is at its plimsoll load line (92) and stayed there. Route
registration is a free function, matching the `dispatchWebhookAction` precedent.
- `dataentryconfig.Config` gained `//plimsoll:max-fields=21` with a rationale:
each exported field is one top-level YAML key, so grouping to satisfy the lint
would break every project's config format.
