---
id: PLAN-AN2QTJ
type: planning-checklist
title: 'Planning: Lua history API for entity versions'
started: "2026-10-07"
completed: "2026-10-07"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

In:
- `rela.history(addr)`: the version timeline of the face `addr` names, oldest first.
- `rela.get_version(addr, n)`: the entity as it was at version `n`.
- Both read through the runtime's `VisibleReader` (DEC-O59WM4), so the row gate and field redaction match `rela.get_entity`.
- On a backend without versioning (fs, memory) both raise an error naming the cause. They never return an empty timeline.

Out:
- History of a deleted entity. The HTTP API opens it only for holders of `history:read`; a script reads live entities only, and a deleted one answers nil like `rela.get_entity`.
- The `history:read-redacted` reveal. Scripts always get the redacted view.
- Copy origins (`origin`), relation history, restore, purge.

**Acceptance Criteria:**
1. On sqlite, after two captured edits, `rela.history("T-1")` returns two rows with `version`, `op`, `type`, `face`, `created_at`, `user`, `tool`, `content_hash`.
2. `rela.get_version("T-1", 1)` returns the entity table as it was at version 1, with `version = 1`.
3. A property the principal may not see (`visible:`) is absent from a `get_version` result.
4. An entity the principal may not read answers nil for both, the same as a missing id.
5. On fs and memory both raise `version history is not supported on this storage backend`, also for a missing id.
6. `get_version` with a version that does not exist answers nil; with 0, a negative number or a fraction it raises.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: small change on an existing capability)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:**
- `store.HistoryReader` (`internal/store/store.go`): `ListVersions(ref)`, `GetVersion(ref, n)`; implemented by pgstore and sqlitestore.
- The HTTP history API (`internal/dataentry/history_handler.go`, `historyworld.go`) is the reference for gating: a live face is gated by the same resolver read as the entity GET; the snapshot's type and face must match the gated row; the snapshot is redacted under `affordances.WithHistoricalSubject` so conditional `visible:` grants fail closed (TKT-73C6B2).
- `visibility.ScriptReader` (`internal/visibility/luareader.go`) already gates and redacts every script read; `UnrestrictedReader` is the ungated CLI/docs variant.
- No library applies.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

1. `store.ErrHistoryUnsupported`: a sentinel for "this backend keeps no version history".
2. `visibility` gains two methods on `ScriptReader` and on `UnrestrictedReader`:
   - `EntityVersions(ctx, addr) ([]store.VersionMeta, error)`
   - `EntityVersion(ctx, addr, n) (*entity.Entity, store.VersionMeta, error)`
Both first check that the raw store is a `store.HistoryReader` (else
`ErrHistoryUnsupported`), then resolve `addr` through the same gated
`GetAddress` path, then read history for that row's `Ref()`. `EntityVersion`
refuses a snapshot whose type or face differs from the gated row
(`ErrNotFound`), and `ScriptReader` redacts it with `RedactRow` under
`affordances.WithHistoricalSubject`. One shared helper does the work; the two
types differ only in the redactor. `DenyReader` answers `ErrNotFound`.
3. `internal/lua`: a consumer-side `HistoryReader` interface with those two methods, and a new `history.go` with free-function bindings (Runtime is at its plimsoll method cap). The binding type-asserts `deps.VisibleReader`; a reader without the methods raises the same "not supported" error.
4. Docs: Lua API reference for the two bindings, including that versions are captured by a debounced sweep, so the newest version can trail the live entity by one sweep interval.

**Alternatives rejected:**
- A separate `ReadDeps.History` field: a second read handle beside `VisibleReader` is the shape CLAUDE.md warns about (a wiring could point it at the raw store). Keeping history on the same reader means it cannot be gated differently.
- Opening deleted entities on `history:read`: not needed for sync and adds the existence-oracle handling of `resolveHistorySubject`; can follow later.

**Files to modify:**
- `internal/store/store.go` (sentinel)
- `internal/visibility/history.go` (new), `luareader.go`, `unrestricted.go`, `denyreader.go`
- `internal/lua/history.go` (new), `runtime.go` (register), `deps.go` (interface)
- Tests: `internal/visibility/history_test.go`, `internal/lua/history_test.go`, an appbuild-level sqlite test for the real store
- Docs: the Lua scripting reference

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**
- `addr` from the script: parsed by the resolver's address grammar; any miss is `ErrNotFound` → nil.
- `n` from the script: must be an integer ≥ 1, else the binding raises.

**Security-Sensitive Operations:**
- History reads are gated by the live row gate of the same reader, so a hidden entity is nil, as for `get_entity`.
- Snapshots are redacted with the historical-subject marker, so a conditional grant cannot open a field hidden at write time.
- A snapshot of another type or face is refused, matching the HTTP rule (cross-type leak).
- Store errors are raised to the script as-is only for the unsupported case; other errors become nil, matching `get_entity`.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**
- AC1, AC2: sqlite store, create and update an entity, flush the sweep, run a Lua script, assert the rows and the snapshot.
- AC3: visibility test with a `visible:`-restricted property and a fake history store.
- AC4: hidden entity through `ScriptReader` answers nil.
- AC5: memstore runtime raises for an existing and a missing id.
- AC6: version 99 → nil; 0, -1 and 1.5 raise.

**Edge Cases:**
- Faced entity: `ID@face` reads that face's lineage only.
- Renamed entity: the timeline includes pre-rename versions (store behavior; asserted once).
- Snapshot type differs from live type: nil.
- An entity the sweep has not captured yet: an empty timeline. This is a real answer on a versioned backend, unlike the unsupported case.

**Negative Tests:**
- Unsupported backend raises (AC5); bad version numbers raise (AC6); hidden and missing ids answer nil (AC4); a cross-type snapshot answers nil.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**
- Sweep lag: the newest version can trail the live entity by one sweep interval, and a new entity can have no version yet. Documented in the binding reference; the sync design keys its base on `content_hash`.
- A second read path that gates differently from `get_entity`: avoided by resolving through the same `GetAddress` path and testing a hidden entity.

Effort: s

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
- [x] The Lua scripting reference (`rela.history`, `rela.get_version`)

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** No critical or significant findings. Minor, folded
into the plan: an uncaptured entity has an empty timeline (documented, distinct
from the unsupported error); the row gate and the redactor must share one ACL
bind per call (one helper does both); timelines are unbounded, as on the HTTP
API (nit, no change).
