---
id: TKT-Z678TN
type: ticket
title: 'Cut MCP peak memory 24x: persistent on-disk search index reused across restarts'
kind: enhancement
priority: high
effort: s
status: done
---

## Problem

`rela mcp` peaks at ~1.15GB RSS on a 2,443-entity project (26MB of markdown on
disk) before GC settles it. On a machine running one MCP process per worktree —
routinely 10 — this dominates memory use.

Heap profiling shows rela's own code is not the cost: reading and parsing every
entity accounts for ~57MB. The remaining **2.75GB of allocation churn (75.8%)**
happens inside bleve under `bleveindex.IndexBatch`, because the index is rebuilt
from scratch on every single startup and the whole corpus goes into one batch.

## Approach (revised after measurement — see "Rejected" below)

Three changes, default (fs) build only:

1. **Persist the search index** under `.rela/search` instead of rebuilding it
in memory each start. `.rela/` is gitignored and already holds derived state
(audit log, caldav aliases, state KV).
2. **Skip the backfill when the index is already current** — the startup boost.
A watermark records the store mtime each completed backfill covers; if the store
has not advanced past it, the index is reused as-is.
3. **Chunk the backfill** (`backfillChunkSize = 100`) so the cold path no longer
holds the whole corpus in one bleve batch.

Plus a correctness fix found on the way: `Search` now sorts by `-_score` then
`id`. Equal-scoring hits previously came back in bleve's internal document
order, which depends on insertion order — `TestGolden_ToolCalls` was already
latently flaky (six runs produced six different orders; one passed by luck).

## Results

Real `rela mcp` startup, 2,443 entities:

| | Peak RSS |
|---|---|
| Baseline (develop) | 1157 MB |
| New, cold start (first ever run) | 267 MB |
| **New, warm start (every subsequent run)** | **48 MB** |

**24x reduction** on the path that actually runs. Cold start is a one-time 267MB
and writes a 38MB index.

## Concurrency

Multiple processes on one project dir are handled by design, not by luck. bbolt
takes an exclusive lock on the index directory; a second opener would otherwise
block forever, so `bolt_timeout` bounds the wait and the loser logs a warning
and falls back to an in-memory index. Verified with two concurrent MCP
processes: first got the on-disk index (35MB), second fell back cleanly (105MB),
neither hung.

## Rejected: in-memory scorch

The first attempt swapped `bleve.NewMemOnly` (legacy upsidedown/gtreap) for
in-memory scorch. It looked good on startup (1085MB → 271MB) and was nearly
merged. **Do not do this.**

`scorch.Open` starts its persister and merger goroutines only when the index
path is non-empty (`scorch.go:311`). An in-memory scorch index runs NEITHER, so
nothing ever merges the segment each write creates. Memory then grows with
(writes × distinct documents touched) and never comes back. On this corpus, 1500
single-entity updates:

| Edits spread over | Heap |
|---|---|
| 20 distinct docs | 121 MB |
| 100 distinct docs | 562 MB |
| 500 distinct docs | 1845 MB |
| all 2443 docs | 5711 MB |

Baseline upsidedown stays flat at 17MB through the same workload. So the swap
would have traded a bounded one-time startup spike for unbounded growth in
long-lived processes — making the original complaint worse. The on-disk form
does not have this problem precisely because the persister and merger run: heap
stayed at 9MB across 4500 edits.

Also rejected: `unsafe_batch` (23x faster writes, but scorch's `Close` stops the
persister rather than draining it, so a clean shutdown can silently drop recent
writes — measured: a write followed immediately by `Close` was lost). Durability
wins; writes stay on the fsync path.

## Scope

**In scope:** `internal/search/bleveindex` (`New` config + locking, `DocCount`,
`SetWatermark`/`Watermark`, `Search` sort), `internal/appbuild/appbuild_fs.go`
(index selection, chunked backfill, watermark skip).

**Not in scope:** `postgres` / `memorybackend` recipes (neither links bleve);
`GOMEMLIMIT` and `--debug-pprof` for `rela mcp` (both still worthwhile, separate
tickets).

## Acceptance criteria

1. Warm-start peak RSS under 100MB. **PASS — 48MB.**
2. Search results unchanged, same hits and same order. **PASS.**
3. No new dependency. **PASS — `go.mod`/`go.sum` unchanged.**
4. An edit made while rela is not running is picked up on next start.
**PASS — verified with an injected marker.**
5. Concurrent processes must not hang or corrupt the index. **PASS.**
6. `go test ./internal/...`, lint, arch-lint clean. **PASS.**
