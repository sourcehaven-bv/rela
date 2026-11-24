---
id: PLAN-9OSS0CJ
type: planning-checklist
title: 'Planning: Give the whole-set scopedHeaders callers their own read function'
started: "2026-11-23"
completed: "2026-11-23"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Problem:** `scopedHeaders` (`internal/dataentry/scopedread.go`) serves bounded
reads (by ids, or one window) and whole-type reads through one function, so its
allowlist entry is one `request-path` line for a dozen callers and nobody can
see which caller pays for a scan.

**Scope:**

In scope:
- `scopedHeaders` serves bounded reads only: `ByIDs`, or a window with
`Limit > 0`. A request with neither is refused with an error. Its allowlist
entry and `TestUnpagedReadAllowlist_ScopedHeadersIsDebt` are deleted.
- A new query helper `scanHeaders(ctx, svc, q, errClass, req)` drains a stamped
query built by `scopedQuery`, then applies the query scope. The archguard
matcher charges a helper's read to its caller, so each whole-set caller gets its
own allowlist entry and category.
- Whole-set callers and their classification:
  - `commandHandler.visibleListRows` (list-context command payload):
requestPath, TKT-F69VAYJ.
  - `feedEntitySource.listType` (calendar feed, CalDAV static collections,
driver list): requestPath, TKT-SMUO8CS. CalDAV dynamic collections read their
members by id instead.
  - `newGanttHandler` (gantt and sankey node load, AllowAll arm):
requestPath, TKT-DIDCQS6.
  - `actionOps.collectCandidates` for a non-lowering condition: requestPath,
TKT-08JN7C3.
  - `listPipeline.scopedSortedEntitiesScoped` (list Go path for shapes
`listpushdown` declines, `_position`, export, gantt scoped arm, next action, nav
status): stays requestPath, naming TKT-5SXPBL1.
  - `visibleEntitiesOfType` (structured search Go path, shapes `planSearch`
declines): stays requestPath, naming TKT-PSBGQLY.
- Paged instead of whole-set:
  - LLM candidates with no condition: one bounded page, through the same
read a lowered condition uses.
  - `queryService.firstNByTypes`: its windows go through `scopedHeaders`; its
tail goes through `visibleEntitiesOfType` with an offset.

Out of scope:
- Pushing the list Go path or the declined search shapes into the store
(TKT-PSBGQLY and the new follow-up).
- Splitting export's cost from the list pipeline: export must mirror the
list's filters and sort, so it reads through the same pipeline.

**Acceptance Criteria:**
1. `scopedread.go#scopedHeaders` is not on `unpagedReadAllowlist`;
`TestNoNewUnpagedReads` passes; `TestUnpagedReadAllowlist_ScopedHeadersIsDebt`
is deleted.
2. `scopedHeaders` with neither ids nor a window returns an error.
3. Each whole-set caller has its own allowlist entry with category and reason.
4. LLM candidate reads are bounded: a budget test shows the rows read do not
grow with the type when the candidate list fills early.
5. Existing ACL, world, face and scope tests still pass on every branch.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: internal refactor of one read function)
- [x] ~~Searched for existing libraries that solve this problem~~ (N/A: internal read split)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: internal refactor)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:**
- `collectHeaders` already is a query helper; `scanHeaders` forwards to it.
- `collectLoweredCandidates` (actions_llm.go) already reads one bounded page.
- `firstNByTypes` (queryservice.go) already reads growing windows.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**
- `scopedHeaders`: refuse `!ByIDs && Limit <= 0`, and a negative offset.
Window branch sets `q.Limit, q.Offset` on the stamped query before
`collectHeaders`, so the matcher sees the Limit. Drop the AllowAll type-scan
fast path from the bounded function (it only served unwindowed reads).
- `scanHeaders`: `collectHeaders` then `applyScope`. Callers call
`scopedQuery` first and return on `withheld`. AllowAll reads use the GraphQuery
with type and faces, which returns the same rows as the type scan.
- `headerEntities` converts headers to entities for scan callers.
- Update callers, allowlist entries, comments that name "scopedHeaders' type
scan", and test helpers.

**Alternatives rejected:**
- One `scopedScan` function for every whole-set caller: one allowlist entry
again, so the per-caller cost stays hidden.
- Chunked keyset reads for the list Go path: still reads the whole type, only
in more calls.

**Files to modify:**
`internal/dataentry/{scopedread,list_pipeline,helpers,queryservice,commands,feed_handler,gantt_handler,actions_llm}.go`,
tests in `internal/dataentry`,
`internal/archguard/unpagedread_allowlist_test.go`,
`internal/archguard/unpagedread_test.go`, `.claude/rules/collection-reads.md` if
wording changes.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** No new input. Window sizes are constants.

**Security-Sensitive Operations:** The row gate, world and face stamping stay in
`scopedQuery`, which both functions use. `scanHeaders` keeps `applyScope`, so a
scope is never skipped. Windows are cut in the store query after the row gate,
so a hidden row never takes a window slot. The LLM truncation flag stays
post-gate and post-condition.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**
1. archguard tests (`TestNoNewUnpagedReads`, allowlist shape) pass.
2. New unit test: unbounded `scopedHeaders` request is refused.
3. Existing scopedread tests run through `scanHeaders` and `scopedHeaders`
for both verdict branches.
4. New LLM budget test: rows read are the same over 100 and 1,000 padded
rows when the candidate list fills in the first window
(`storetest.AssertScaleFree`).
5. `TestFirstN_SparseMatchReadsBoundedWindows` still reads 4 windows.

**Edge Cases:** empty type; candidate list never fills (reads to the end in
windows); subject row inside a window; window boundary exactly at the type size.

**Negative Tests:** unbounded request; window with scope (existing refusal).

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** A concurrent write can shift rows between LLM windows (a row read
twice or skipped). Mitigation: dedupe by id; a skipped row is the same guarantee
`firstNByTypes` gives. Effort: m.

## Documentation Planning

- [x] ~~User-facing docs identified (skip if internal refactor)~~ (N/A: internal refactor, no user-facing change)
- [x] ~~Docs-checklist will be created when entering implementation~~ (N/A: chore, no docs)

**Documentation Impact:** N/A - internal change.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** RR-75K7M04 (critical), RR-Q1N5EI6, RR-YC298WN,
RR-H3MM3O3, RR-DVXPRLC (significant), all addressed. The plan changed as
follows: `scanHeaders` refuses a query with no type or another type, and a
source guard pins its call shape; feed, commands, gantt and the LLM scan stay
request-path with follow-ups TKT-SMUO8CS, TKT-F69VAYJ, TKT-DIDCQS6 and
TKT-08JN7C3; CalDAV dynamic collections read their members by id; the LLM read
with no condition reuses the bounded lowered read; `firstNByTypes` keeps its
three windows and reads its tail through `visibleEntitiesOfType`; a test pins
every request-path entry to its ticket.
