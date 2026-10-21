---
id: PLAN-E7QGB8
type: planning-checklist
title: 'Planning: fsstore accepts writes of undeclared types that vanish on reopen'
started: "2026-10-21"
completed: "2026-10-21"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Problem:** `fileLayout.entityFileKey` (internal/store/fsstore/layout.go) falls
back to the type name plus "s" as the directory when the type is not in the
layout's schemas. A write of an undeclared type therefore succeeds and the
entity is in the in-memory index, but the scan on the next open
(`resolveEntityType` in index.go) skips any directory that maps to no declared
type. The entity is gone on every later open, with no error at write time.

`entitymanager` already rejects an unknown type ("unknown entity type",
core.go), so the human write path is safe. The gap is direct `store.Store`
writes: fsimport, conversions, migrations, scripts or tools that bypass the
manager, and tests. The database backends keep such rows (they persist across
opens), so this is fsstore-specific: the file layout cannot represent an
undeclared type.

**Scope:**
- In: fsstore refuses an entity write whose type is not declared in its
schemas, at every write entry point that takes a caller-supplied type:
`CreateEntity`, `UpdateEntity`, `UpdateEntityIf`, `ReplaceEntityFamily` (each
replacement row), on both `FSStore` and `txStore`.
- In: fix any caller or test that relies on the fallback.
- In: refresh fsstore's schemas on a schema hot reload, so a newly declared
type is writable without a restart (needed once the gate exists).
- In: one plural rule for writing and scanning.
- Out: relations. A relation file name carries its type and is scanned from
one flat directory, so a relation of an undeclared type is not lost on reopen.
Unchanged.
- Out: the database backends. They persist undeclared types; changing their
contract is a separate decision.
- Out: entities already on disk under an undeclared directory (schema drift).
They stay invisible as today; analyze reports orphan directories.

**Acceptance Criteria:**
1. `CreateEntity` with an undeclared type returns an error naming the type,
writes no file, and leaves the entity absent from `GetEntity` and
`ListEntities`. Test in fsstore.
2. `UpdateEntity` and `UpdateEntityIf` with an undeclared type return the same
error and write nothing. Test.
3. `ReplaceEntityFamily` with a replacement row of an undeclared type keeps
returning `store.ErrInvalidFamilyReplacement` and leaves the old family intact
(existing behavior, pinned). Test.
4. The same calls through `Tx` (`txStore`) behave identically, and an update
that changes an entity's type to an undeclared one writes and removes nothing.
Test.
5. A declared type with `Plural: ""` works: write, reopen, entity present.
Test.
6. A type string such as `../relations/x` is refused and writes nothing
outside its type directory. Test.
7. Schema refresh: after `SetSchemas` adds a type with a custom plural, a
write of that type succeeds, lands in the custom directory, and survives reopen;
files already in that directory join the index. Test in fsstore, plus the MCP
reload test (`TestMCPServices_ReloadPicksUpNewType`) creating an entity of the
new type.
8. The sqlite-to-markdown export with a source row of an undeclared type
fails before writing anything. Test.
9. No existing test or caller relies on the fallback: the full
`go test ./...` passes; fsstore test schemas come from the storetest type list.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: small fix)
- [x] ~~Searched for existing libraries that solve this problem~~ (N/A: internal invariant)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A)
- [x] ~~Reviewed relevant rela concepts for prior art~~ (N/A)

**Research Doc:** N/A

**Existing Solutions:** `entitymanager` rejects unknown types with `unknown
entity type: %s`; fsstore uses the same wording so the error reads the same on
both layers. The layout already holds `schemas` for the plural lookup.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**
1. **One plural function.** `New` normalizes each schema: an empty `Plural`
becomes `<type>s`. `entityFileKey` and `buildPluralToTypeMap` then read the same
value, so a declared type always maps back on scan (design review: today an
empty plural is written to `<type>s` and skipped on reopen).
2. **Write gate in the cores.** Add `fileLayout.declared(entityType) bool` and
check it in the write cores `createEntity` and `updateEntityIf` (which `FSStore`
and `txStore` both call), before any file is written or removed, so an update
that changes the type to an undeclared one writes nothing. Error:
`fmt.Errorf("fsstore: unknown entity type %q", t)` (`%q` so an empty type is
visible). `ReplaceEntityFamily` already rejects undeclared types before any file
moves (convert.go), wrapping `store.ErrInvalidFamilyReplacement`, which
entitymanager checks; keep it.
3. **Schema refresh on reload (design review, critical).** The MCP server's
schema hot reload (`cli/mcp_wiring.go` `mcpServices.reload`) rebuilds the
entitymanager against the already-open store, but `fileLayout.schemas` is fixed
in `New`. With the gate, a type added by a reload would pass entitymanager and
be refused by fsstore until restart; today it is written to a guessed directory
and lost if its plural is custom. Add
`FSStore.SetSchemas(map[string]store.EntityTypeSchema) error`: it validates and
normalizes like `New`, swaps the layout under the write lock (writers are
serialized by that lock; readers of the layout get it through an
`atomic.Pointer` or take the same lock, following the CLAUDE.md
publish/serialize split), and scans the directories of newly declared types so
their existing files join the index. Types removed by a reload keep their index
entries until reopen (reads keep working; new writes of that type are refused).
The reload site gets the refresh through a consumer-side seam supplied at wiring
(the fs recipe in appbuild provides it; database backends need none, documented
as `Nil: accepted`), not a type assertion on the store. Find every site that
rebuilds services against an open store on a schema change and wire it there.
4. **Tests that relied on the fallback.** storetest writes many types that
fsstore's `defaultTestSchemas` does not declare, and
`storetest/differential_test.go` declares only some of the types it writes.
Export the harness's type list from storetest and build fsstore's test schemas
from it, so new harness tests cannot drift. Fix other tests by declaring the
type.
5. **sqlite-to-markdown export** (`appbuild/projectdata_sqlite.go`
`copyData`) writes into an fsstore directly. Add a preflight that lists the
source's undeclared types and fails before the first write, so the export never
leaves a partial target. (fsimport writes into a database, not fsstore, so it is
unaffected.)
6. Keep `entityFileKey`'s fallback-free form for read and delete paths: they
act on index entries, which only hold declared types. Document on
`entityFileKey` that writes are gated.

**Alternatives rejected:**
- Gate in `entityFileKey` itself (return an error): it has about 20 callers,
mostly read/delete paths on index entries; changing its signature spreads error
handling to places that cannot fail.
- Gate only in entitymanager: already done there; the defect is the direct
store writes that bypass it.
- A shared `store.ErrUnknownType` sentinel across backends: no caller
classifies this error, and the database backends do not reject, so a
cross-backend sentinel would claim a contract that does not exist.

**Files to modify:**
- internal/store/fsstore: layout.go, fsstore.go (`New` normalization,
`SetSchemas`), entity.go write cores (+ tests)
- internal/store/storetest: exported type list; fsstore test schemas
- internal/cli/mcp_wiring*.go and internal/appbuild (refresh seam wiring)
- internal/appbuild/projectdata_sqlite.go (export preflight) (+ test)

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** the entity type comes from the caller. The check
is an allowlist: the schema's declared types. It also stops a type string from
choosing a directory name under `entities/` that no schema declares.

**Security-Sensitive Operations:** none new. The error names the type, which is
config (not secret).

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** table-driven fsstore test over the write entry points
(direct and via `Tx`), each asserting the error, no file under `entities/`, and
absence from reads; plus a reopen test for the declared default plural.

**Edge Cases:**
- Empty type string: rejected by the same check.
- A type that is declared but whose plural equals another type's directory:
out of scope (schema validation's job).
- A schema reload that removes a type while the store is open: its index
entries keep working until reopen; new writes of that type are refused.
- A reload that fails to load leaves the old services and the old layout.
- A concurrent write during `SetSchemas`: serialized by the write lock.

**Negative Tests:** AC1 to AC4 are all negative tests.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**
- A caller that relied on the fallback now fails loudly. That is the intent: it
was losing data silently. Mitigation: full test run; grep fsimport,
datamigration and scripts for direct store writes.
- The sqlite-to-markdown export of a database that holds rows of a type the
schema does not declare now fails up front instead of writing files that vanish
on the next open. The error lists the types.
- The schema refresh seam touches the reload wiring; a mistake there could
leave the store on the old schema. Mitigation: the MCP reload test creates an
entity of a newly added type.

**Effort:** m

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] ~~Docs-checklist will be created when entering implementation~~ (N/A: chore, no user-facing change)

**Documentation Impact:** N/A; code comments only.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** see the RRs linked to TKT-C2YVX1. The critical
finding added the schema refresh on hot reload; the significant ones changed the
test schemas (built from the storetest type list) and added one plural rule for
writing and scanning.
