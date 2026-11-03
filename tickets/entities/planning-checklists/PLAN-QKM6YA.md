---
id: PLAN-QKM6YA
type: planning-checklist
title: 'Planning: /_search: client cancels are logged as failures and answered 500; measure and reduce per-keystroke search cost'
started: "2026-10-24"
completed: "2026-10-24"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Findings (verified, see the ticket body for the measurements):**

- The hypothesis is half right. The SPA does abort superseded searches
(AbortController in CommandPaletteModal, EntityPickerModal and the mention
menu), but its debounce is only 150 ms, so an ordinary typist (200–300 ms
between keys) sends one search per keystroke and aborts all but the last.
Cancellations are expected; their number is the client's doing.
- The 5xx are NOT pratique reporting a canceled upstream. pratique's Caddy
maps a client cancel of a proxied request to 499 and does not log it as "request
failed". The "request failed status=502" lines come from pratique's own
pre-proxy step: `resolver.Resolve` (UserByID, OrgByID, ActiveMembership with
`r.Context()`) and `MintFor`. A cancel during that step returns an error, which
`failPage` answers with 502 and logs. Its `ErrorPage` passes no `LogErr`, so the
log line does not show the cause. This is code reading (pratique 0d764bf, caddy
v2.11.4); not reproduced.
- rela: a cancel DOES stop the database work. pgx sends a cancel request
(postgres logs "canceling statement due to user request"). But pgx's default
context watcher then closes the pooled connection, so the next request opens a
new one: 12 aborts gave 11 new backend sessions.
- rela logs a cancel as a failure at some stages only. At most stages the
handler returns silently and the access log records status=200 for a request
nobody received. When the cancel lands in `loadHitHeaders` (and on 26.10.10 also
in the per-hit field check), the error is wrapped without ErrScope, falls to the
default branch of `writeListPipelineError`, logs WARN "dataentry: list free-text
search failed ... context canceled" and answers 500. So production's 1,103
"context canceled" lines undercount the real cancels.
- Each search is expensive for what the UI shows. The SPA sends no `limit`,
so the server ranks up to 1000 hits (`maxFreeTextSearchResults`) in 500-row
keyset pages, with one full ranking statement per page, and sends them all (366
KB for one keystroke on the seed). The UI shows 50 (palette, picker) or 20
(mention). `?limit=` is applied after the search and does not reduce the
database work.
- In a non-default world (atlas: `default_world: actueel`) the text
predicate is applied after `DISTINCT ON (id)`, so the trigram GIN index is never
used: every search seq-scans and detoasts `search_text` for every candidate row.
On the seed (19k entities) a search that matches 1 entity costs 200–320 ms of
database time.

**Scope:**

In scope (rela repo):
1. Server cancel handling: a canceled `/_search` (and every list pipeline
that uses `writeListPipelineError`) logs nothing and writes nothing; the access
log records it as canceled (status 499, `canceled=true`) instead of 200 or 500.
2. Limit pushdown: `executeQuery` takes the caller's limit; the gated
search runs one page of `limit` rows when no Go-side property filter remains.
3. SPA: every `/_search` caller sends `limit` = its MAX_RESULTS; one shared
debounce constant of 300 ms for palette, picker, mention menu and RelationCards.
4. World-scoped search SQL uses the trigram index: a `MATERIALIZED` CTE of
`(id, face)` rows whose `search_text` matches, used to restrict the `DISTINCT
ON` input to matching families, and an `(id, face) IN` test on the resolved
prime in place of the second LIKE. Only for needles of 3 or more characters;
shorter needles keep today's shape. Applied in lockstep to
`buildVisibleSearchSQL` and `buildSearchSQL`.

Not in scope (follow-ups, other repos):
- pratique: answer a canceled request in Resolve/MintFor with 499 and
without "request failed"; pass the error to the log in `ErrorPage`.
- devops perf-report: count cancels from rela's access log (status 499)
rather than from "context canceled" warnings; count pratique 499 separately from
5xx.
- pgx context watcher (`CancelRequestContextWatcherHandler`) to keep the
pooled connection on cancel. Open question; measure first.
- RelationCards has no abort and no stale-response guard. Noted, not fixed
here beyond the debounce.
- `?limit=` on `/_search` for API callers keeps its meaning (post-gate
truncation); only the implementation changes.

**Acceptance Criteria:**
1. A `/_search` whose client disconnects at any stage produces no WARN or
ERROR log line and no 500. The access-log line has `status=499 canceled=true`.
2. Other list pipelines that call `writeListPipelineError` behave the same
on cancel. DeadlineExceeded keeps its current mapping.
3. `/_search?q=gateway&limit=50` on the seed (scale 1, alice) runs one
ranking statement instead of three and returns at most 50 rows; total database
time drops from about 620 ms to about 200 ms.
4. Typing 14 characters at 200 ms ±50 % sends at most 2 searches (today:
13).
5. On the seed in world `published`, a search for a 1-hit term ("carol",
"tsk-0367") costs under 10 ms database time (today 200–320 ms); an unselective
term ("gateway", 9.5k hits) is no slower than today (±10 %); results and order
are identical.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: the options are few and measured directly; see Approach)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:**
- Caddy reverse_proxy `statusError` (v2.11.4): maps context.Canceled to 499,
"using 5xx is problematic for users" (caddy #3748). rela adopts the same status
in its access log.
- pgx v5.11 `pgconn`: default `DeadlineContextWatcherHandler` closes the
connection on cancel; `CancelRequestContextWatcherHandler` keeps it.
- Codebase: `logInternalError` and `writeGateError` already treat
context.Canceled as "nobody to answer" (api_v1.go). The fix makes
`writeListPipelineError` consistent with them.
- `mentionMenuState.ts` already has a shared debounce + abort + generation
guard; palette and picker duplicate the same logic with their own constants.
- `*_explain_test.go` in pgstore pin index use; `storetest.RunVisibleSearchTests`
and the gated/ungated ordered-subsequence contract pin semantics.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

1. Cancel handling (`internal/dataentry`):
   - `writeListPipelineError`: first case `errors.Is(err, context.Canceled)`
returns without logging or writing.
   - `requestStats`/`requestAttrs`: when `r.Context().Err()` is
context.Canceled at the end of the handler and nothing was written, record
status 499; add `canceled=true` whenever the context was canceled before the
handler returned.
2. Limit pushdown: `handleV1Search` passes `limit` to a new
`executeQueryLimit(ctx, query, limit)`; `runVisibleFreeTextSearch` uses
`min(limit, maxFreeTextSearchResults)` as `search.Query.Limit` when there are no
property filters and no `type` post-filter is needed (the `type` parameter is
already spliced into the query). Callers without a limit (`_position`,
next-action) keep 1000.
3. SPA: `searchEntities` gains an optional `limit`; palette and picker
pass 50, the mention menu 20. One exported `SEARCH_DEBOUNCE_MS = 300` used by
all four callers.
4. SQL: in the non-trivial world branch of `buildVisibleSearchSQL` and
`buildSearchSQL`, when the needle has 3 or more runes: `WITH m AS MATERIALIZED
(SELECT id, face FROM entities WHERE search_text LIKE ...)`, inner `AND e.id IN
(SELECT id FROM m)`, outer `(e.id, e.face) IN (SELECT id, face FROM m)` instead
of the LIKE. Semantics stay "resolve the prime first, then test the prime's
text". Merged with the existing `withClause(withParts)`.

**Alternatives considered:**
- Only raise the debounce: removes about 90 % of requests, but each search
still costs 200–600 ms and cancels stay noisy. Rejected as the only fix.
- Stop aborting superseded requests and only drop stale responses: no
cancels in the logs, but every keystroke's search runs to completion. More
database load. Rejected.
- Plain `id IN (SELECT ... LIKE)` prefilter, keeping the outer LIKE:
measured +40 % on unselective terms (double detoast). Rejected in favour of the
matched-set test.
- Lower `maxFreeTextSearchResults`: changes `_position` and next-action
semantics. Rejected; pushdown of the caller's limit is enough.

**Files to modify:**
- internal/dataentry/api_v1.go (writeListPipelineError, handleV1Search)
- internal/dataentry/queryservice.go (limit parameter)
- internal/dataentry/requeststats.go (499 / canceled)
- internal/store/pgstore/visiblesearch.go, internal/store/pgstore/search.go
- frontend/src/api/entities.ts, components/ui/CommandPaletteModal.vue,
components/forms/EntityPickerModal.vue,
components/forms/milkdown/mentionMenuState.ts and useMentionMenu.ts,
components/forms/RelationCards.vue
- tests next to each; docs/ for the access-log field

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**
- `q` and `limit` from the request. `limit` is already validated by
`parseSearchLimit` (1..100). The needle stays a bound parameter; LIKE escaping
is unchanged.

**Security-Sensitive Operations:**
- The visible search IS the read gate (ACL clause inside the SQL). The CTE
only narrows the candidate set to families with a textual match; the visibility
clause and the world ranking still run on every candidate face, so no hidden row
can enter. Pinned by `storetest.RunVisibleSearchTests` and
`RunFieldScopeSearchTests`.
- Limit pushdown must apply after the gate (it does: the SQL limit counts
gated rows; field-gated scopes keep Go-side truncation).
- 499 in the access log leaks nothing: the log goes to the operator.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**
- AC1/AC2: handler tests with a searcher and a store that cancel the
context at each stage (search, header load, linkable, owners); assert no log
record at WARN+, nothing written, access-log attrs have 499 and canceled=true.
Postgres-tagged test that cancels a real query.
- AC3: `storetest.Counting` budget test: `/_search?limit=50` on a
broad term runs one search statement; response has ≤ 50 rows.
- AC4: vitest with fake timers for each caller: 14 inputs 200 ms apart →
1 request; input then pause 300 ms → request carries `limit`.
- AC5: pgstore `*_explain_test.go` asserting a Bitmap Index Scan on
`entities_search_trgm_idx` for a 3+ char needle in a non-default world;
RunVisibleSearchTests and the gated/ungated subsequence contract unchanged;
manual re-run of `.ignored/repro` measurements.

**Edge Cases:**
- Needle of 1–2 characters (no trigrams): old shape.
- Needle with `%`, `_`, `\`: escaping unchanged, test kept.
- Multibyte needle: count runes, not bytes, for the 3-character rule.
- Entity whose non-prime face matches but prime does not: must not appear
(existing test, re-run on the new shape).
- Cancel after the response is fully written: status stays 200,
canceled=true.
- `limit` with property filters in `q`: Go-side truncation, as today.

**Negative Tests:**
- DeadlineExceeded still answers 504/500 and logs.
- A real store error still logs and answers 500.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**
- SQL planner choices differ on production data. Mitigation: EXPLAIN test
on the seed, and the shorter-needle fallback; measure on atlas after deploy via
the perf report.
- 300 ms debounce feels slower. Measured: final results arrive about
100–150 ms later after the last key (500–640 ms vs 380–470 ms locally).
Mitigation: keep stale results visible (already the case).
- Access-log consumers that count 5xx see fewer; that is the intent, but
the devops perf report must learn 499.

**Effort:** m overall (server cancel s, limit s, SPA s, SQL m). Possible split:
SQL prefilter as its own ticket.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
- docs for `--access-log` fields: new `canceled` attribute and status 499.
- No data-entry UI docs change (debounce is internal).

## Design Review

- [x] Run `/design-review` before starting implementation (run after the user answered the open questions)
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** 20 findings, 10 significant, recorded as
review-responses on TKT-T2I7P2 (RR-6FVU7QL, RR-4BU9BIX, RR-DG6117E,
RR-2R2CIS2, RR-3LZMYJ0, RR-K3ZDZ5B, RR-31ZGWPQ, RR-PKPP2FN, RR-8COE77M,
RR-ML64UL2). Plan changes they caused:

- Limit pushdown: `executeQueryWith(ctx, q, queryOptions{prefilters, limit,
  exactType})`. The cap drops to `limit` only with no property filter, no
  `sort:` clause and no type the caller drops afterwards. The store already
  sizes its page by `Query.Limit`, so nothing changes in pgstore for this.
- Matched-set CTE: only when the needle has a run of 3 or more ASCII letters
  or digits; filtered by the world candidate, the visibility clause and the
  types; restricts the inner query by `id` only. A storetest case pins
  "term only on a non-prime face is not a hit".
- Cancel: one `requestCanceled(r, err)` helper (error chain or request
  context); also silent in identity resolution and local-role priming.
- Access log: canceled requests log 499 `canceled=true`, plus
  `written_status` if a status was written; a panic stays 500; event
  streams excluded.
- Frontend: the picker sends `limit` only for at most one type; the mention
  menu asks for 100; RelationCards gets the debounce only.

Minor and nit findings accepted without a separate entity: shared debounce
constant in a dependency-free module, `searchEntities` options object, one
state snapshot in `handleV1Search`, a 280 ms typist row in the vitest, and
documenting the deleted-hit and face-drop shortfall.
