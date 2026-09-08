---
id: REV-01BJAX
type: review-checklist
title: 'Review: PostgreSQL read-path performance audit: per-request query counting, seeded demo with ACL + worlds, slow-query driven optimisation'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`)

**Evidence (final run on e3c32de0, 2026-09-04):**
- `just coverage-check`: package floor (50%) PASS, total floor (65%) PASS, total 79.2%. `internal/worldreader` was at 37.8% after the batching and is at 93.2% after `relations_page_test.go`.
- `go test -race -tags postgres` over pgstore, appbuild, userstate, search, dataentry: ok (dataentry 458 s). `go test -race -tags sqlite ./internal/store/sqlitestore/`: ok.
- `golangci-lint run ./...`: 0 issues. `just plimsoll`: OK. `just comment-lint`: no unresolvable doc links across 13,236 comments. `go-arch-lint check`: OK.
- e2e (`just build-server-e2e && npm test`): 280 passed, 8 skipped, 1 failed (`create-redirect.spec.ts` "rapid create (stress)"), which passed 2/2 on an isolated rerun; it ran while the coverage and postgres suites were competing for CPU. The earlier git-crypt failure was real (lock indicator lost on header rows) and is fixed by `EntityHeader.Inaccessible`.
- Pre-existing, out of scope: `go test -tags sqlite ./internal/appbuild/` fails `TestBuild_StateRows_WarnAtStartup` on `develop` too (the test's build constraint excludes postgres/memorybackend but not sqlite, and the sqlite build never reads entity files). CI does not run sqlite-tagged tests for that package.
- Re-measured `.ignored/perf/baseline.sh` on the review-fixed build: status codes and query counts identical to the pre-review run on every endpoint (the review fixes are query-neutral); timings not comparable because the suites were running concurrently.

**Comment findings.** `just comment-report` lists the advisory rules
(duplication, nil-contract, param-contract, restatement). They are not a merge
gate, but a finding your diff *introduces* should be fixed or suppressed — don't
grow the backlog.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** cranky-code-reviewer: RR-T3GR05 (critical, `ne` relation
filter failed open), RR-LCS2Y3 (critical, JSON-null sort parity), RR-GQTAPX
(critical, requestFor partial identity match; also raised by the security
reviewer), RR-Q8488L (significant, BFS reorder unpinned), RR-9P8JYE
(significant, per-row matcher allocation), RR-VNZV71 (significant, false
redaction doc), RR-7LL8MB (significant, dead wrapper), RR-EQBQ7Q (significant,
migration 0014 rewrite cost), RR-E8XF0Y, RR-K4DLZ8, RR-E2KNIT, RR-FV3O0H,
RR-PN1STR (minor), RR-04KFJA (nit). rela-security-reviewer: RR-SR2NS0 (minor,
header gate missing faceReadable) plus the requestFor finding folded into
RR-GQTAPX. All 15 are `addressed` with resolutions; none deferred. Fixes are in
commit e3c32de0.

**Self-review:** the diff is confined to the read path, the seeder,
observability and docs. No write-path, versioning, sweep or change-feed code
changed. One deliberate cross-cutting change: `store.EntityHeader` gained
`Inaccessible` so header rows keep the git-crypt lock marker.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

1. **Observability**: PASS. `requeststats_test.go` asserts the `Server-Timing` header and `request` record under Debug, absence at Info, absence on SSE; `pgstore/tracer_test.go` and `tracer_pool_test.go` assert the count matches the statements issued, including inside `Tx`; `store/querystats_test.go` covers the ctx type. Verified live on the perf server (`-verbose` prints `request ... queries=N db_ms=...`).
2. **Budget pins**: PASS with one target missed. `querybudget_test.go` over `storetest.Counting` asserts 10-row and 50-row reads are equal and pins list page = 6 (target ≤ 6), search = 3 (target ≤ 4), view with a two-column table section = 11 (target ≤ 8). The view constant is above target because a view does entry load, two traverse passes, collection load, section columns plus target headers, entry edges and the membership walk, each one bounded query; it no longer grows with rows (was 1208 queries on the seeded project view, now 42).
3. **Seeder**: PASS. `perfseed_test.go` covers determinism (same seed → identical ids and properties), id validation, entity and relation counts, refusal of a non-empty store, attribution and the audit record. Scale 1 into local postgres: 19,117 rows, 47,549 relations in about 40 s (target < 3 min).
4. **Measured improvement**: PASS. Table in the ticket body: list page 207 → 8 queries, 180 → 3 ms; kanban 807 → 8, 215 → 10 ms; project view 1208 → 42; search `type:task` 66,004 → 4 queries, 3.2 s → 41 ms; free text 5.1 s → 1.2 s; next actions 7 queries, 12 ms. Query counts no longer depend on row count for reader, editor and manager alike.
5. **Indexes proven**: PASS. `graphquery_explain_test.go` gains `TestGraphQueryExplainPagedListUsesDerivedListIndex`; migration 0014 indexes have EXPLAIN coverage for the `(type, id)` and id-prefix scans; `derivedschema_test.go` covers list-index create and drop on config removal; `queryplan` unit tests cover the `listIndexSpec` shapes.
6. **ACL unchanged**: PASS. (a) `acl_list_test.go` hidden neighbour in a batch; (b) `acl_views_test.go` AllowAll under `published` sees no draft title through batched section and list paths; (c) `listpushdown_test.go` scoped total equals the visible count via `CountMatched`; (d) `rowcontent_test.go` redaction and `TestListRows_KeepInaccessibleMarker` on header rows. `visibleHeaderIDs` applies `faceReadable` exactly as `filterVisible` (RR-SR2NS0).
7. **Wire compatibility**: PASS. List and search rows omit `content` by default; `include_content=true` restores it (`rowcontent_test.go`, openapi updated); e2e suite green; `relationsPatch.ts` reads the detail endpoint, not a list row.

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-CF2ZL2

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

Follow-ups recorded in the ticket body rather than left as TODOs: `_position`
and gantt still load whole-type headers; free-text search on very common terms;
views with `display: content`; fs seeding is slow at scale 1.

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI

<!--
Deliberately NOT tracked here: the PR URL and whether CI passed.

Both post-date this checklist. `/pr` requires the ticket to be `done` and
validating clean before it opens the PR, and a `done` review-checklist may have
no unchecked items — so an item asking for the PR URL can only be satisfied by a
PR that does not exist yet. Checking it early would mean asserting "CI passed"
before CI ran, which turns the checklist from evidence into a formality.

GitHub records both authoritatively, and the branch and commit messages carry
the ticket ID, so the ticket-to-PR link is recoverable without duplicating it
here. See TKT-UFV01M. -->
