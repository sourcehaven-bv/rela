---
id: REV-XWPOA4
type: review-checklist
title: 'Review: Wire the SQLite backend: sqlite build tag, appbuild recipe, release and CI isolation'
status: done
---

<!-- @managed: claude-workflow v1 -->

PR #1434 · branch `feat/sqlite-wiring-TKT-L1A3PH`.

## Automated Checks

- [x] All four build-tag combinations compile (`just build-check-tags`)
- [x] Six release targets cross-compile at `CGO_ENABLED=0`
- [x] `go test -tags sqlite ./internal/appbuild/... ./internal/cli/...` — green
- [x] Default build: store, appbuild and docscli suites all green, pgstore
against a live PostgreSQL
- [x] `golangci-lint` — 0 issues under both the default and sqlite tags
- [x] `just arch-lint` clean; `just plimsoll` clean
- [x] Docs regenerated from `docs-project/` and committed

## Code Review

- [x] Reviewed by the `cranky-code-reviewer` agent
- [x] All critical and significant findings addressed

**The review found two critical bugs I had missed, and both were real.**

I had asked it specifically to find files whose tags should have widened and did
not. It found one — and it was the worst kind, because it writes to data.

| ID | Severity | Status | Finding |
|----|----------|--------|---------|
| RR-S185UV | critical | addressed | `rela-desktop` bricks on re-selecting the already-open project |
| RR-ZGHOI1 | critical | addressed | `rela-docs` screenshot capture could seed fixtures into a real database |
| RR-EH1K2S | significant | addressed | No `PRAGMA user_version`, so a ladder could never be retrofitted cleanly |
| RR-I6GF36 | significant | addressed | `rela db migrate` exited non-zero for a no-op; CI lacked negative tag assertions |
| RR-L03E7L | minor | addressed | Shared-bleve tag did not state its own requirement; third copy of the closer |
| RR-R5RVIX | minor | addressed | Single-process invariant is per-project, not per-machine |

**The desktop bug is the one worth dwelling on.** `LoadProject` opened the new
store before closing the previous services — an ordering that was fine for every
prior backend because none held an exclusive resource. With SQLite, re-selecting
the already-open project from the recent-projects menu hits *this process's own
lock*, and the error names its own pid as the culprit. I reproduced it before
fixing rather than reasoning about it:

```
REPRODUCED — reopening the same project fails:
  another process is using .../rela.db (held by pid 71054 on Space.local ...)
```

Nothing would have caught it: `rela-desktop` appears in no goreleaser build and
no CI isolation assertion. It does now.

**The docs-capture bug is the same class as the one I did catch.** I found the
`cli/db_nonpostgres.go` tag trap myself, three sessions earlier, and fixed it.
`internal/docscli/capturer_fs.go` was the *identical* pattern — `!postgres`,
inheriting into the sqlite build — and I did not look for a second instance of a
bug I had already found once. Its doc comment even asserted "an ephemeral
fsstore that never touches real data", which is false under `-tags sqlite`.

**On the `user_version` finding, the reviewer's sharpest point was about the
future, not the present.** Shipping without a ladder is defensible; shipping
without a *version marker* is not, because a ladder retrofitted later cannot
tell v1 from v2 and would have to sniff `pragma_table_info` to guess. Stamping
it now costs nothing and is the difference between a follow-up ticket and an
archaeology project.

No findings were deferred or disputed.

## Acceptance Verification

| AC | Verdict | Evidence |
|----|---------|----------|
| 1 — `-tags sqlite` builds | **PASS** | `build-check-tags` covers all four combinations |
| 2 — six targets at `CGO_ENABLED=0` | **PASS** | linux/darwin/windows × amd64/arm64 |
| 3 — `rela db` correct under sqlite | **PASS** | all three subcommands exit 0 and report schema version 1 |
| 4 — CI isolation | **PASS** | 8 assertions now, incl. negative tag-conflict checks and rela-desktop |
| 5 — appbuild suite under the tag | **PASS** | all three packages green |
| 6 — no-ops justified | **PASS** | each carries its reasoning, now with the correct per-project scope |
| 7 — arch-lint / lint / coverage | **PASS** | clean under both tags |

Verified end-to-end through the shipped binary rather than only the suite:
created entities and a relation, confirmed the data is in SQLite with zero
markdown files, and confirmed a real second process is refused by the lock and
admitted again on release.

## Documentation

- [x] `docs/sqlite-backend.md` — generated from a guide entity, covering the
single-writer constraint, the network-filesystem refusal, and **plainly stating
that this backend has neither git history nor version history today**
- [x] `CLAUDE.md` storage-backend table gains a row plus the isolation rule
- [x] `README.md` index regenerated
- [x] Reasoning documented where it would otherwise be undone —
`releaseLoadedProject` explains the ordering requirement at the site someone
would reverse it
