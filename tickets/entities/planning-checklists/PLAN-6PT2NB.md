---
id: PLAN-6PT2NB
type: planning-checklist
title: 'Planning: Move the version sweep out of the store into a versioning service'
started: "2026-10-11"
completed: "2026-10-11"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** In: `VersionSweep` service in pgstore and sqlitestore (Start/Stop,
validating constructor); appbuild builds, starts and stops it; remove
`StartVersionSweep` and `store.VersionSweeper`; rename pgstore `mu` to
`changeFeedMu`, guarding only change-feed state. Out: event-driven wake-up;
shared fan-out type; any change to capture semantics, cadence, advisory lock or
one-connection tick.

**Acceptance Criteria:**
1. No new imports per build. Test: `go list -deps` diff per tag; `just arch-lint`.
2. Version capture unchanged. Test: `storetest.RunVersionTests` on sqlite and postgres.
3. Existing sweep tests pass with intent unchanged (lock scope, drain, BUG-1DWMYO).
4. `Services.Close` stops its own sweep only. Test: two sqlite tenants on one SharedBase; close A, A's goroutine ends and A captures nothing; B still captures; `CloseAssembly` stops the replaced sweep. Under -race.

## Research

- [x] ~~For larger features: run `/research`~~ (N/A: M-size refactor with a fixed target shape)
- [x] ~~Searched for existing libraries~~ (N/A: internal lifecycle move)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: internal lifecycle move)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:** TKT-N0IKN9 moved versioning to `VersionStore` ("a store
just stores"); the job queue and listener lifecycles in appbuild
(`stopBackgroundServices`) are the pattern for start/stop ownership.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** `NewVersionSweep(store, provider, cfg)` per backend; it
reads only the handle (and sqlite `versionMu`, shared with purge). appbuild
declares a consumer-side `versionSweep interface{Start(); Stop()}`; tagged files
build it (`versionsweep_postgres.go`, `versionsweep_sqlite.go`,
`versionsweep_nosweep.go`). `assemble` builds it early; `finishAssembly` starts
it after every fallible step; `stopBackgroundServices` stops it. Alternatives
rejected: constructor from a raw pool (public `SharedBase.Assemble` and MCP
reload have no pool, so they would run without a sweep); a store-level factory
method (recreates the removed capability).

**Files to modify:** internal/store/{store.go, pgstore/pgstore.go,
pgstore/sweep.go, sqlitestore/sqlitestore.go, sqlitestore/sweep.go};
internal/appbuild/{appbuild.go, versionsweep_*.go}; internal/cli/mcp_wiring.go;
.claude/rules/{storage,versioning}.md; tests.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] ~~Input validation approach defined~~ (N/A: no new input; constructor validates collaborators)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] ~~Error handling doesn't leak sensitive information~~ (N/A: no new error surfaces to users)

**Input Sources & Validation:** Only `RELA_VERSION_SWEEP_*` cadence env, read as
before.

**Security-Sensitive Operations:** Mutual exclusion with version purge (pg
advisory lock, sqlite `versionMu`) must be unchanged; one tenant must not stop
another's sweep.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** See acceptance criteria.

**Edge Cases:** Stop before Start; Stop twice; Start after Stop; nil sweep; MCP
reload overlap (old and new sweep briefly both alive; ticks serialize on the
lock).

**Negative Tests:** Constructor rejects nil store, nil provider, a pgstore not
over a pool, a sqlite transaction view.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** A path stops running the sweep (history loss): the sweep is built in
`assemble`, which every path uses. Close ordering: Services stops the sweep
before closing the handle. Effort: m.

## Documentation Planning

- [x] ~~User-facing docs identified~~ (N/A: internal refactor)
- [x] ~~Docs-checklist will be created~~ (N/A: refactor)

**Documentation Impact:** .claude/rules/storage.md and versioning.md.

## Design Review

- [x] ~~Run `/design-review`~~ (N/A: shape fixed in archplan threads th-0d00a35a and th-6c2cf2b3; M-size lifecycle move. Code review covers it.)
- [x] ~~All critical/significant findings addressed in plan~~ (N/A: no design review)

**Design Review Findings:** N/A
