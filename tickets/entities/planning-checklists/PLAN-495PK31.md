---
id: PLAN-495PK31
type: planning-checklist
title: 'Planning: Push ingest find and rename dry-run lookups into the store'
started: "2026-11-13"
completed: "2026-11-13"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** In: the ingest find (`internal/ingest/ingest.go#findTarget`) and the
rename dry run (`internal/entitymanager/rename.go#idTakenByOther`) stop
scanning; their `request-path` allowlist entries go. Out: the soft-deleted id
scan (`markedIDTaken`, `SoftDelete().ListMarked`), id minting (TKT for #2245),
other allowlist entries.

**Acceptance Criteria:**
1. The ingest find reads the same rows over 100 and 1,000 rows of the find type (`TestHolderLookup_FirstEqualCostIsScaleFree`).
2. The find stays gated (row gate in the store query, redaction before the match) and picks the lowest id among duplicates (`TestHolderLookup_FirstEqual` on memstore, sqlite, postgres).
3. The rename dry run reads the same rows over 100 and 1,000 rows (`TestRenameDryRun_CostIsScaleFree`), and still refuses a case variant of another id while allowing a change of its own casing (`TestRenameDryRun_FoldedCollision`).
4. The new store capability passes conformance on all four backends (`storetest` Query/IDTakenFolded, SoftDelete).
5. Both allowlist entries are removed and `TestNoNewUnpagedReads` passes.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: two targeted lookups with an existing pattern)
- [x] ~~Searched for existing libraries that solve this problem~~ (N/A: internal store query)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: internal)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:**
- `visibility.HolderLookup` (realtime inbox) already runs a gated, paged, header-only equality query with redaction and a Go re-check; generalized to a multi-property scan.
- memstore/fsstore `idTaken`, pgstore/sqlitestore `lower(id)` probes in rename; the new capability reuses them.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**
- `store.FoldedIDReader.IDTakenFolded(ctx, id, except)` in `store.Store`; one indexed probe on pg/sqlite, the existing `idTaken` scan on fs/mem; counted by `storetest.Counting`.
- `HolderLookup.FirstEqual(ctx, type, faces, want)`: non-scalar `PropEqual` per property ANDed onto the caller's read query; page of 2 header rows; redacted re-check (string equal, or scalar `fmt` form; lists/objects never match).
- `visibility.HolderLookupFor(reader)` maps the wired reader to its tier (ScriptReader gated, UnrestrictedReader ungated, anything else refusing).
- ingest gets a consumer-side `Finder`; it reads the chosen row through `Reader.GetAddress`.

Alternatives rejected: a method on `ScriptReader` (at its plimsoll cap); a new
`lua.ReadDeps` field (wider plumbing for two call sites); putting
`IDTakenFolded` on `EntityReader` (would widen `tracer.Tracer` and its
visibility decorator).

**Files to modify:**
`internal/store/{store.go,memstore,fsstore,pgstore,sqlitestore,storetest}`,
`internal/visibility/holders.go`, `internal/ingest/ingest.go`,
`internal/entitymanager/rename.go`, `internal/dataentry/webhook_routes.go`,
`internal/scheduler/subscriptions.go`,
`internal/archguard/unpagedread_allowlist_test.go`, tests.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** Webhook/feed match values (untrusted) become
bound SQL parameters only; an empty value still means no match. Rename ids are
validated by `entity.ValidateID` before the probe.

**Security-Sensitive Operations:** Row gate: the caller's ACL read query is
composed into the store query (no per-hit callback); redaction runs before the
re-check so a hidden property never matches. An unknown reader maps to a
refusing lookup (fail closed). The rename probe is ungated, as the scan it
replaces was, and reveals only the collision that the real rename also reports.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** see acceptance criteria.

**Edge Cases:** list value holding the match value (no match); number and bool
(text form); empty match value; another type with a lower id; hidden property on
the lowest match; principal with no read grant; face-named row for the id probe;
LIKE wildcard in the id; except of the same family.

**Negative Tests:** empty type / no properties / zero face selection give
`ErrInvalidQuery`; refusing lookup gives `ErrReaderUnavailable`; finder error
surfaces from `ingest.Run`.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**
- Postgres renders some numbers differently from Go (`1e21`); such a value could miss the store pre-filter. Accepted: match values are external ids.
- A list property no longer matches its Go rendering (`[a b]`). Accepted: never a meaningful match.
Effort: m.

## Documentation Planning

- [x] ~~User-facing docs identified (skip if internal refactor)~~ (N/A: internal performance change)
- [x] ~~Docs-checklist will be created when entering implementation~~ (N/A: no user-facing change)

**Documentation Impact:** N/A - Internal change, no user-facing docs needed.

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: approach follows the existing HolderLookup pattern; reviewed in code review instead)
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** none
