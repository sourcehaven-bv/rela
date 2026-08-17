---
id: REV-26TGG6
type: review-checklist
title: 'Review: Cut MCP peak memory 24x: persistent on-disk search index reused across restarts'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Coverage maintained (`just coverage-check`)

Locally: `go test ./internal/...` clean; `golangci-lint` **0 issues**; `just
arch-lint` **OK - No warnings found**; `just coverage-check` package floor
**PASS**, total **PASS** (77.7%).

On CI (PR #1369) every code check passed: Test, Lint, Architecture, Fuzz,
God-object lint, Frontend, E2E, Postgres Backend, CodeQL, Vulnerability Check,
Lint Markdown, Analyze (go / actions / javascript-typescript), and all six
Cross-Compile jobs (darwin/linux/windows × default/postgres).

Also verified by hand, since these are the invariants this change could
plausibly break:

- `go build -tags memorybackend ./...` and `-tags postgres ./...` both build.
- CI dependency invariants hold: 0 bleve packages in the postgres build,
0 pgx packages in the default build.
- `go.mod` / `go.sum` unchanged — no new dependency (scorch and the zapx
segment libraries were already linked).

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-3JA0ZZ (critical), RR-PEZH8H (critical), RR-240CE1
(significant), RR-TGTS8U (significant) — all `addressed`.

The two criticals are worth reading together, because both were cases where a
change that measured well was in fact wrong:

- **RR-PEZH8H** — the original in-memory-scorch fix would have replaced a
bounded startup spike with unbounded growth (5.7GB after 1500 edits vs a flat
17MB baseline), making the very problem this ticket was raised for worse. Found
by testing the long-lived write path rather than only startup.
- **RR-3JA0ZZ** — the chunked backfill's error path left the chunk undrained,
reinstating the ~1.1GB behaviour on any index error. Found by the reviewer.

Minor/nit findings not separately tracked, and why:

- *`_id` instead of `id` as the sort key (possible DocValues saving)* —
deferred. Plausible but unmeasured, and the sort is on the correctness path; not
worth bundling into this change unverified.
- *Error-message substring assertion; `indexedCount` conflating indexed with
findable* — accepted as-is. Both are test-readability points, not defects.
- *Unused on-disk `New(path)` constructor is a maintenance liability* —
resolved by this change: `New` is now the production path.
- *Chunk size 100 is unmeasured* — it was measured (50/100/500 compared;
500 is materially worse, 50 marginally better than 100), but the constant now
matters much less because the cold path runs once per index lifetime rather than
on every start.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

1. **Warm-start peak RSS under 100MB — PASS (48MB).** Real `rela mcp`,
2,443 entities, RSS sampled every 250ms. Baseline 1157MB, cold start 267MB, warm
start 48MB. **24x** on the path that actually runs.
2. **Search results unchanged — PASS.** Same query through the real MCP
`search_entities` tool on both binaries returns identical IDs in identical
order. Separately confirmed the new sort does not distort relevance: an entity
whose id sorts last (`ZZZ-999`) but which is the strongest match still ranks
first, and exact-id queries still return their target first.
3. **No new dependency — PASS.** `go.mod`/`go.sum` diff empty.
4. **Offline edits are picked up — PASS.** Injected a unique marker into an
entity file while rela was NOT running; the next start reindexed and the marker
was searchable. This is the one that matters for the watermark optimization
being safe rather than merely fast.
5. **Concurrent processes neither hang nor corrupt — PASS.** Two MCP
processes on one project: the first took the on-disk index (35MB), the second
hit the bbolt lock, logged the fallback warning, and ran on an in-memory index
(105MB). Neither hung. `bolt_timeout` is what bounds this; bbolt's default is an
unbounded block, which would have hung startup.
6. **Tests / lint / arch-lint clean — PASS.** See Automated Checks.

New tests are mutation-checked rather than assumed effective: disabling chunking
fails `TestBackfillBleve_BoundsBatchSize` ("largest batch was 317 entities, must
not exceed chunk size 100"); removing the error-path `break` fails
`TestBackfillBleve_StopsReadingAfterIndexError` ("read 1000 entities after an
index error"); removing the trailing `flush()` fails five boundary subtests.

## Documentation (enhancements only)

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: no
user-facing surface changes — no CLI flag, config key, API or metamodel change;
behaviour is identical apart from resource usage)
- [x] ~~User-facing documentation updated~~ (N/A: same reason)
- [x] ~~Docs-checklist marked as done~~ (N/A: same reason)

The non-obvious parts are documented where the next person will actually hit
them: godoc on `NewMem` explains why it grows without a merger and points at
`New`; godoc on `New` explains the bolt lock and why `unsafe_batch` was
rejected; `indexIsCurrent` explains why the store's mtime and the index's
`LastModified` are different clocks and must not be compared.

**Docs Checklist:** N/A

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

One trade-off is deliberately left open rather than hidden: entity writes now do
a durable index write (~40ms) inside the fsstore write lock, because
`unsafe_batch` — which makes writes 23x faster — loses recent writes when
`Close` races the persister (measured: a write immediately followed by `Close`
was lost). Durability won. If interactive writes feel slow in practice,
debouncing `EntityPut` is the fix, and it deserves its own ticket rather than
being smuggled in here.

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
- [x] All CI checks pass
- [x] PR URL documented below

**PR:** https://github.com/sourcehaven-bv/rela/pull/1369

The only red check was `Rela Tickets`, which fails by design while this
checklist is `in-progress` and the ticket sits in `review` — completing them is
what clears it.
