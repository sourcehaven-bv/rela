---
id: PLAN-5GABRFV
type: planning-checklist
title: 'Planning: rela test v1 (1/4): untagged in-memory appbuild recipe with MemFS write / disk config split'
started: "2026-11-07"
completed: "2026-11-07"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

Part 1 of TKT-0MCLX9S. The full design is in PLAN-G8YMBDB (Approach step 1) and
RR-A44DAAX, RR-TV6X80O, RR-ESY8MA1.

In scope:
- Untagged `appbuild.NewInMemory(root, opts...)` in `internal/appbuild/memrecipe.go`: memstore, `search.LinearSearch`, `Config.FS = storage.NewMemFS()`, `projectConfig = rootfs.New(root)`, `audit.NewMemory()`, a fresh `script.NewEngine()`, full `prepare` + `assemble`.
- `appbuild_memory.go` (`memorybackend`) `New` delegates to the shared recipe body `newInMemory(cfg, opts)`.
- Route every disk writer that bypasses `Config.FS` to memory when the FS is a MemFS.
- CI dependency-gate line for the recipe.

Out of scope (parts 2 to 4): the `rela test` command, the Lua module, the clock,
mail/HTTP/AI seams, output dir, fixtures.

**Acceptance Criteria:**
1. Tree hash unchanged after building a world on a project with comments, piles and a pending data migration and writing through the entity manager, comments, piles, leases, state KV and migration record: `TestNewInMemory_LeavesProjectTreeUntouched`.
2. `go list -deps -tags postgres` links no bleve; CI gate line on `./internal/appbuild`.
3. 100 build+Close cycles leave no goroutines: `TestNewInMemory_CloseStopsEveryGoroutine` (goleak).
4. Automations, validation and ACL run on writes: `TestNewInMemory_WritesRunTheWritePath`.

## Research

- [x] ~~For larger features: run `/research`~~ (N/A: design settled in PLAN-G8YMBDB)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A (see PLAN-G8YMBDB)

**Existing Solutions:**
- `appbuild_memory.go` already pairs memstore and LinearSearch with prepare/assemble.
- `Config.projectConfig` is the sqlite recipe's config-read seam (`appbuild_sqlite.go`).
- MemFS-aware lock selection already exists for comments, piles, leases, channels, realtime and IdP roles (`*storage.MemFS` checks).
- `goleak` is already a module dependency.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**
- `NewInMemory` resolves paths with `project.At(root, OsFS)` (read only) and builds a hermetic Config. `newInMemory(cfg, opts)` is the recipe body shared with the memorybackend `New`.
- Writer audit: state KV, migration record (filemigstate), comments, piles, leases, channels and IdP roles all go through `Config.FS`, and their locks are already process-private on a MemFS. The one writer that bypassed the FS is the data-migration lock (`datamigration.LockFor` writes `.rela/migration.lock` with `os` calls). New helper `migrationLockDir(cfg)` returns "" for a MemFS, which selects the process lock.
- Open choice resolved: the memorybackend `New` keeps the caller's Config (Discover's OsFS and filesystem audit) instead of becoming hermetic, so that build's behaviour does not change. Only `NewInMemory` is hermetic.
- Open choice resolved: host config (`.rela/mail.yaml`, `ai.yaml`, secrets) is still read from disk; part 3 replaces it with `WithHostConfig` and the mail/HTTP/AI seams.

Alternatives rejected: delegating the memorybackend `New` to the hermetic
`NewInMemory` (would silently drop `Discover`'s audit log and state for that
build); a read-only project dir in tests (would turn writes into swallowed
warnings, not failures).

**Files to modify:**
- New: `internal/appbuild/memrecipe.go`, `internal/appbuild/memrecipe_test.go`
- `internal/appbuild/appbuild_memory.go`, `internal/appbuild/appbuild.go` (`migrationLockDir`)
- `.github/workflows/ci.yml` (dependency gate)

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**
- `root`: an operator-supplied directory. `project.At` requires a schema or `.rela/` there; an empty root is rejected. Config is read through `rootfs`, which contains reads per area with `os.Root`.

**Security-Sensitive Operations:**
- No disk writes: every runtime-state writer goes to the MemFS; the migration lock is process-local. The tree-hash test (with directory mtimes, so transient lock files count) is the backstop.
- ACL is the operator's real policy, built by the same `assemble` as every recipe; the write-path test checks a denial.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** see Acceptance Criteria; all are integration tests over a
full world in `internal/appbuild/memrecipe_test.go`.

**Edge Cases:**
- Entities on disk are not loaded (store starts empty).
- Pre-existing `.rela/` state (comments, piles, applied.json) is untouched and invisible to the world.

**Negative Tests:**
- Empty root and a directory that is not a project are rejected.
- A viewer's write is denied and not applied.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**
- A writer outside appbuild bypasses the FS: the tree-hash test covers every subsystem the world starts.
- The recipe is untagged and links into the postgres build: CI gate line.

Effort: l.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] ~~Docs-checklist will be created when entering implementation~~ (N/A: internal wiring, no user-facing change; the guide is part 4)

**Documentation Impact:**
- [x] N/A - Internal change, no user-facing docs needed

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** done for the umbrella: RR-A44DAAX, RR-TV6X80O,
RR-ESY8MA1 on TKT-0MCLX9S cover this part.
