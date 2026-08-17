---
id: IMPL-V842VT
type: implementation-checklist
title: 'Implementation: Cut MCP peak memory ~4x: scorch in-memory index + chunked bleve backfill'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (full flow, not just units)
- [x] Feature implemented
- [x] All edge cases from planning handled

**What changed:**

1. `internal/search/bleveindex/bleveindex.go` — `NewMem` builds the in-memory
index with `scorch` (`bleve.NewUsing("", mapping, scorch.Name, scorch.Name,
nil)`) instead of `bleve.NewMemOnly`, which hardcodes the legacy `upsidedown`
engine. Mapping, analyzers, boosts and query construction are untouched.

2. `internal/appbuild/appbuild_fs.go` — `backfillBleve` streams entities
through `backfillChunkSize` (100) batches instead of materializing the whole
corpus into a single `IndexBatch` call. `clear(chunk)` before reslicing so
flushed entities are not pinned by the backing array. Error accounting preserved
via a running `total` and accumulated `indexed`, with first-error fail-fast.

3. `internal/search/bleveindex/bleveindex.go` — `Search` now sorts explicitly
by `-_score` then `id`. **Not originally planned**; see below.

**Unplanned change (3): deterministic tie-break.**

Switching engines surfaced a latent bug rather than causing one.
`TestGolden_ToolCalls` failed, and the planning risk register had flagged
exactly this ("ranking or tokenization differs between engines"), so it was
investigated rather than re-baselined.

Finding: a query like `requirement` matches every entity of that type with an
*identical* score (verified: all three fixture entities score 0.4113). With no
secondary sort key, bleve falls back to internal document order, which depends
on the order documents were indexed. The golden fixture overwrites titles in Go
**map iteration order**, which is randomized per run — so the search result
order varied run to run. Sampling the test six times produced six different
orders, and one run passed by luck.

So the golden was pinning upsidedown's incidental tie-break, and the test was
already latently flaky; scorch merely changed which arbitrary order appeared.
Re-recording the golden would have hidden a real nondeterminism bug that would
intermittently fail CI. Adding `req.SortBy([]string{"-_score", "id"})` makes
equal-scoring hits stable regardless of engine or insertion order.

The committed golden file was **not** modified — the fix restores the expected
order rather than overwriting the expectation.

**New tests:** `internal/appbuild/backfill_fs_internal_test.go`

- `TestBackfillBleve_IndexesEveryEntityAcrossChunkBoundaries` — 7 table-driven
subtests covering empty store, single entity, one under/over a chunk, exactly
one chunk, exact multiples, and straddling several chunks. Asserts every entity
is indexed exactly once.
- `TestBackfillBleve_ReportsListErrorsAndIndexesTheRest` — injects an
unreadable entry mid-chunk; asserts the error is reported *and* every good
entity is still indexed (pins the reworked skip accounting).

## Manual Verification (REQUIRED)

- [x] Feature tested end-to-end manually
- [x] Each acceptance criterion verified
- [x] Verification evidence documented below

**AC1 — peak RSS under 400MB.** Both binaries built from the same commit
(baseline = changes stashed), run as real `rela mcp` against the tickets
project, RSS sampled every 250ms:

- baseline: **PEAK 1099MB**
- fixed: **PEAK 269MB**, and **291MB** after the tie-break sort was added
- under sustained query load (300 search calls): 978MB -> 249MB

PASS (291MB < 400MB, ~3.8x reduction).

**AC2 — search results unchanged.** Same query driven through the real MCP
`search_entities` tool over stdio on both binaries:

- baseline: `TKT-VFJKMB TKT-9INY0Y TKT-9TQ6I TKT-92JL8P TKT-VC27L3`
- fixed: `TKT-VFJKMB TKT-9INY0Y TKT-9TQ6I TKT-92JL8P TKT-VC27L3`

Identical IDs in identical order. Separately verified the tie-break does not
distort relevance: an entity whose id sorts last (`ZZZ-999`) but which is the
strongest title match still ranks first, and an exact-id query still returns its
target first. PASS.

**AC3 — no new dependency.** `git diff go.mod go.sum` is empty; scorch and the
zapx segment packages were already linked as indirect deps. PASS.

**AC4 — tests pass.** `go test ./internal/...` clean, no failures. The golden
test specifically was run 6x consecutively and passed 6/6 (it previously varied
every run). PASS.

**AC5 — on-disk index unchanged.** `bleveindex.New` is untouched in the diff;
`internal/search/...` suite passes. PASS.

## Quality

- [x] Code follows project patterns
- [x] No silent failures (errors surfaced, not just logged)

- `just lint` (golangci-lint): **0 issues**.
- `just arch-lint`: **OK - No warnings found**.
- `just coverage-check`: package floor **PASS**, total **PASS** (77.7%).
- Tests are table-driven with `t.Run` subtests per project convention.
- Error surfacing unchanged: `backfillBleve` still returns a wrapped error
naming counts and causes; `openBackend` still degrades to a warning + usable
store rather than failing startup.
- Mutation-checked the new tests: removing the trailing `flush()` fails 5
subtests with precise counts, so they genuinely constrain the chunking
arithmetic rather than merely executing it.
