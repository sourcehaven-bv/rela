---
id: IMPL-GX8LQ7
type: implementation-checklist
title: 'Implementation: Realtime 2/1a: lease storage (kv with file lock, postgres, conformance)'
started: "2026-10-11"
completed: "2026-10-11"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (lease validators, kvlease, pglease, filelock)
- [x] Integration tests written (test full flow, not just units) (leasetest.RunAll against kvlease on MemFS and disk, and pglease on a real schema-pinned postgres)
- [x] Happy path implemented
- [x] Edge cases from planning handled (lazy expiry, prune, caps at the boundary, deleted and corrupt document, rekey onto live/expired, sub-second times)
- [x] Error handling in place (errors surfaced, not swallowed)

## Test Quality

- [x] Using fixture builders or factories for test data (leasetest holder/hash helpers, fake clock)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] ~~Feature manually tested end-to-end~~ (N/A: storage only, not wired to any surface; covered by the conformance suite over real backends)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified (pg suite run three times against a local database with RELA_TEST_DATABASE_REQUIRED=1)

**Verification Evidence:** AC1: leasetest.RunAll passes on kvlease (MemFS, disk)
and pglease. AC2: 100-goroutine single-winner and per-identity-cap tests, two
kvlease stores with separate file locks over one directory, mixed concurrent
writes. AC3: store API takes hashes only (format-checked); lock file O_NOFOLLOW
0600; corrupt document reset under lock, persisted once, contents not logged.
`go test -race` on lease, filelock, piles, state, appbuild: pass.

## Quality

- [x] Code follows project patterns (check similar code) (piles/kvpiles/pgpiles/pilestest layout)
- [x] Checked for DRY opportunities (file lock moved to internal/filelock and shared with kvpiles; shared validators used by both backends)
- [x] No security issues introduced (security review: no critical/significant; both minor findings fixed)
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
