---
id: REV-1ESTYJ
type: review-checklist
title: 'Review: Bound analyze memory — content-free headers, streaming analyzers, per-section caps'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] `go test ./internal/...` — all packages pass
- [x] `just lint` — 0 issues
- [x] `just arch-lint` — OK
- [x] `just plimsoll` — clean (two load lines raised by exactly +1; see below)
- [x] `just coverage-check` — PASS (`internal/visibility` 84.8% → 87.4%)
- [x] pgstore suite against a real PostgreSQL — `ok`, 41s, including the new
      Header conformance
- [x] Frontend: 1,609 tests pass, `vue-tsc` clean, ESLint 0 errors
- [x] Both build tags compile (`go build ./...` and `-tags postgres`)

## Acceptance Verification

Measured against the original repro dataset — 20k entities, 1,974 MB of
content, real PostgreSQL — not a synthetic benchmark.

- [x] **AC1** PASS — peak RSS **2,947 MB → 51 MB** for one `_analyze`
      (criterion: < 150 MB). RSS sampled at 20 ms.
- [x] **AC2** PASS — **flat in body size**: the same 20k entities with EMPTY
      bodies peak at 50 MB, versus 51 MB with 1,974 MB of content. A 1 MB
      difference across a 2 GB swing.
- [x] **AC3** PASS — 3 concurrent requests **6,264 MB → 55 MB**
      (criterion: < 300 MB).
- [x] **AC4** PASS — existing analyze and ACL tests pass unchanged: same
      issues, same order, for fixtures under the cap.
- [x] **AC5** PASS — boundary tests at 0/99/100/101/500. Exactly 100 is
      complete; 101 returns 100 with `Truncated`. Surfaced to the wire as
      `truncatedChecks` and rendered as `100+` plus a notice.
- [x] **AC6** PASS — `analyzeDuplicates` still detects duplicates; it groups
      headers first (it cannot stream) and caps at issue-emission.
- [x] **AC7** PASS — `storetest.RunHeaderTests` runs for every backend;
      memstore (native), fsstore (generic fallback), pgstore (native) all pass.
- [x] **AC8** PASS — enforced by the type: `EntityHeader` has no Content
      field, so no header can reach a `.Content` consumer. Verified by
      compilation.

## Code Review

Self-review; no `/code-review` agent run. Findings recorded and addressed
inline rather than as separate `review-response` entities, since all were
found and fixed before the PR opened.

- **Scope was wrong in the ticket (significant, FIXED)** — A/B/C alone left
  `_analyze` at **1,454 MB**, failing AC1, while every unit test passed. Only
  running the repro found it. Profiling turned up three more whole-store body
  loads, two outside `internal/dataentry`: `tracer.FindOrphans`,
  `visibility.VisibleTracer.FindOrphans` (one full `GetEntity` per orphan to
  read `.Type`), and `analyzeOrphans` re-loading every orphan as a full entity
  (~930 MB alone). The ticket was scoped to `analyze.go`; the fix spans four
  packages.
- **The second buffer (significant, FIXED)** — `ScriptReader.ListEntities`
  materializes the ENTIRE matching set to hand to `Reader.Filter`, because the
  row gate batches per type. Fixing `analyze.go` alone would not have fixed the
  OOM.
- **ACL equivalence (significant, VERIFIED)** — header gating is asserted
  against `ListEntities` rather than hand-written expectations, so the two
  cannot drift. **Mutation-tested**: removing the gate from `FilterHeaders`
  fails with `hidden entity SEC-1 leaked through the header path`; restoring
  passes.
- **Fail-closed paths (minor, FIXED)** — `FindOrphans`'s two type-resolution
  routes are covered, including the header-scan-error fallback, asserting the
  same visible set. A divergence would be an ACL bug appearing only on stores
  lacking the header capability.
- **Truncation determinism (minor, DOCUMENTED not fixed)** — on a truncated
  section, which 100 issues survive is "the first 100 in store order", and
  store order (ascending by id) is not natural order. So truncated output is a
  bounded sample, not "the naturally-first 100". Written into the code comment
  rather than silently glossed.

## Notes

- **`plimsoll` load lines raised by +1** on `MemStore` and pgstore `Store`,
  each with the reason at the declaration site. Per the root CLAUDE.md, a store
  implementation's count is the mandated-interface exception rather than a
  ratchet target; `ListEntityHeaders` is implemented natively in both because
  the generic fallback would clone bodies only to drop them, which is the cost
  the capability exists to remove.
- **A distinct type, not `EntityQuery{OmitContent}`** — a half-populated
  `entity.Entity` satisfies every interface a real one does and lies to all of
  them. `computeEntityETag` hashes `.Content` and would return a well-formed
  ETag for the wrong bytes. ~337 `.Content` reads across 12 packages would each
  become a latent bug; the compiler now rejects the mistake.
- **Fixed an unrelated upstream YAML break** —
  `AM-feed-field-redaction.md` (from #1314) had unquoted `visible:` in its
  title and description, so the entity failed to parse and was invisible to
  every analyze scan. Quoting it re-exposed a genuine pre-existing validation
  error on BUG-E9DYW5 (done, no review checklist) that had been hidden.
- Out of scope, per the ticket: migrating list/search/kanban-card surfaces to
  `EntityHeader`, and caching/precomputing analysis.
