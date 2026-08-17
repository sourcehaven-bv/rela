---
id: PLAN-LDRNRP
type: planning-checklist
title: 'Planning: Cut MCP peak memory ~4x: scorch in-memory index + chunked bleve backfill'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

IN scope (default fs build only):
- `internal/search/bleveindex.NewMem` — build the in-memory index with the
`scorch` index type instead of `bleve.NewMemOnly` (which hardcodes the legacy
`upsidedown` engine).
- `internal/appbuild.backfillBleve` — stream entities through fixed-size
batches rather than materializing the whole corpus into one bleve batch.

NOT in scope:
- `bleveindex.New` (on-disk index) — untouched; existing on-disk indexes and
their format are unaffected.
- `postgres` / `memorybackend` recipes — neither links bleve.
- `GOMEMLIMIT` and `--debug-pprof` for `rela mcp` — both worthwhile, neither
needed for this win. Deferred to separate tickets.
- Any change to search semantics, mapping, boosts, or query construction.

**Acceptance Criteria:**

1. Peak RSS for `rela mcp` startup on a ~2.4k-entity project is under 400MB.
Test: launch `rela mcp`, sample RSS every 250ms, record max.
2. Search results are unchanged — identical hits and identical ranking order.
Test: same query through the MCP `search_entities` tool on both binaries, diff
the returned IDs and their order.
3. No new dependency added. Test: `go.mod` diff is empty.
4. `go test ./internal/...` passes.
5. On-disk index behaviour unchanged. Test: `bleveindex.New` is not modified;
`internal/search/...` suite passes.

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A — cause was isolated directly by profiling; the fix is two
small edits, so a separate RES entity would add ceremony without insight.

**Existing Solutions:**

- No new library needed. `scorch` ships inside the already-vendored
`bleve/v2 v2.6.0`, and the zapx segment packages (`zapx/v11`..`v17`) are already
listed in `go.mod` as indirect deps — so this is a configuration change, not a
dependency addition.
- bleve upstream already treats scorch as the default:
`bleve/config.go:79` sets `Config.DefaultIndexType = scorch.Name`. Only the
`NewMemOnly` constructor opts out, at `bleve/index_impl.go:165`, where a missing
index type falls back to `upsidedown.Name`. So the fix aligns rela with bleve's
own default rather than inventing a configuration.
- scorch in-memory mode is a supported, documented path: `scorch.openBolt`
(`index/scorch/scorch.go:321`) treats an empty `path` as memory-only and sets
`unsafeBatch = true`. Reached via `bleve.NewUsing("", mapping, scorch.Name,
scorch.Name, nil)`.
- Prior art for batching in this repo: `bleveindex.IndexBatch` already exists
precisely to avoid O(N) single-document transactions during backfill; this
ticket keeps that method and only bounds how much is handed to it at once.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

Two independent changes; both are needed because they fix different halves of
the problem.

1. `bleveindex.NewMem`: replace `bleve.NewMemOnly(buildMapping())` with
`bleve.NewUsing("", buildMapping(), scorch.Name, scorch.Name, nil)`. Empty path
= in-memory, no disk I/O. The mapping, boosts and all query construction are
untouched, which is what keeps results identical.

2. `appbuild.backfillBleve`: introduce `backfillChunkSize = 100` and stream
the `ListEntities` iterator into a reusable chunk slice, flushing via the
existing `idx.IndexBatch` whenever the chunk fills, then once more at the end.
`clear(chunk)` before reslicing so the flushed entities are not kept alive by
the backing array. Error/skip accounting is preserved by tracking `total` and
accumulating `indexed`; the first index error is retained and stops further
flushes, matching the previous fail-fast behaviour.

Why both: GC tuning alone plateaus at ~910MB (measured with GOGC=25 and
GOMEMLIMIT=128MiB), because the single batch holds *live* memory — GC cannot
reclaim what is still referenced. scorch alone reaches 775MB but then plateaus
instead of falling, because the whole corpus is still resident in one batch.
Combined: 271MB.

**Files to modify:**

- `internal/search/bleveindex/bleveindex.go` (NewMem + scorch import)
- `internal/appbuild/appbuild_fs.go` (backfillBleve + backfillChunkSize)

**Alternatives considered:**

- *GC tuning only (GOGC / GOMEMLIMIT).* Rejected as a solution: measured floor
is ~910MB, and it trades CPU for memory without addressing the cause. Still
worth doing later as belt-and-braces — separate ticket.
- *Chunking only, keep upsidedown.* Gets to 511MB, but leaves the 4.5GB churn
and, more importantly, leaves gtreap's missing compaction — which is what makes
long-lived MCP processes grow unboundedly.
- *scorch only, keep one batch.* 775MB and plateaus; see above.
- *Disk-backed scorch index.* Rejected: introduces index files, invalidation
and cleanup for a process whose index is naturally ephemeral.
- *Smaller chunk (50) / larger (500).* 50 gives marginally lower peak, 500
noticeably higher. 100 sits at the knee of the curve.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

No new input surface. The indexed content is the same entity corpus already read
from the store by the same `ListEntities` call; this change only alters how many
of those entities are held in memory at once and which bleve engine stores them.
No parsing of new formats, no new file paths, no user-supplied configuration
(`backfillChunkSize` is a compile-time constant, not tunable from config or
env).

**Security-Sensitive Operations:**

- File access: unchanged. Notably the in-memory scorch index uses an empty
path, so it writes nothing to disk — no new files, no temp-dir exposure, no
cleanup obligation. This is worth stating explicitly because scorch is normally
a disk-backed engine.
- No auth, crypto, or network involvement.
- Error handling: the existing error message reports counts and wrapped
errors only. `skipped` is now derived from a `total` counter rather than
`len(entities)`; no entity content is added to any message.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

- AC1 (memory): launch `rela mcp` against the tickets project on both the old
and new binary, sample RSS at 250ms, compare peaks. Expect <400MB new.
- AC2 (result parity): drive `search_entities` over real MCP stdio on both
binaries with the same query; compare returned IDs *and their order*.
Additionally compare raw hit counts at the index layer.
- AC3 (no new dep): `git diff go.mod go.sum` empty.
- AC4: `go test ./internal/...`.
- AC5: `internal/search/...` and `internal/appbuild/...` suites pass; confirm
`bleveindex.New` is untouched in the diff.

**Edge Cases:**

- Empty store (0 entities): `flush()` is a no-op on an empty chunk; the final
`flush()` after the loop must not emit an empty batch. Covered by existing
appbuild tests that build projects with no entities.
- Corpus smaller than one chunk (<100): single trailing flush only.
- Corpus exactly a multiple of the chunk size: the in-loop flush empties the
chunk, so the trailing `flush()` sees `len(chunk) == 0` and does nothing — no
duplicate final batch, no empty batch.
- Entities that fail to parse: `ListEntities` yields an error, which is
appended to `listErrs` and the entity skipped — unchanged behaviour. The tickets
project has exactly such a file, so this path is exercised in the manual run.
- Index error mid-corpus: `indexErr` is retained and `flush()` becomes a
no-op for the remaining chunks, preserving fail-fast; `skipped` is then `total -
indexed`, which correctly counts everything not written.

**Negative Tests:**

- If `IndexBatch` returns an error, backfill must not silently succeed:
`backfillBleve` returns a wrapped error and `openBackend` logs a warning while
still returning a usable store (existing, deliberate degradation — search is
unavailable rather than startup failing).
- A nil index remains non-fatal: `openBackend` returns `ErrSearcher` before
backfill is reached. Unchanged.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- *Ranking or tokenization differs between engines.* This is the main risk —
a silent relevance change would be worse than the memory win. Mitigated by
keeping the mapping/analyzers/boosts byte-identical and verifying result parity
through the real MCP tool, including order. Verified: identical IDs, identical
order, same top hit.
- *scorch in-memory sets `unsafeBatch = true`.* This disables bolt durability
for batches, which is irrelevant for a memory-only index that is rebuilt on
every start, but would matter if this constructor were ever reused for a
persistent index. Mitigated by leaving `New` (on-disk) alone.
- *Chunking changes error accounting.* Mitigated by tracking `total`
separately and preserving first-error semantics; reviewed in the diff.
- *Other build tags regress.* `postgres` and `memorybackend` do not link
bleve, but arch-lint and a tagged build should still be run before merge.

**Effort:** s

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] N/A — internal change. No CLI flag, no config key, no API or metamodel
surface changes. Behaviour is identical apart from resource usage, so nothing in
`docs/` describes the altered behaviour. The rationale (why scorch, why chunked)
is captured in code comments at both sites and in this ticket, which is where
the next person will look.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** None — the approach was derived from heap profiles
and validated by measurement across five configurations before any code was
written, and both changes are local and reversible. Findings from `/code-review`
in the review phase are tracked as RR entities on the ticket.
