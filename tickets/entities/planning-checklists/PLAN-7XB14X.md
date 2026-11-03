---
id: PLAN-7XB14X
type: planning-checklist
title: 'Planning: Split metamodel.Metamodel part 2: relation and property-type queries'
started: "2026-10-11"
completed: "2026-10-11"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** In: move the `migration.MetamodelProvider` implementation
(`GetPropertyType`, `IsPropertyRequired`, `GetPropertyDefault`,
`GetTypeDefault`, `GetRelationLabel`, `GetRelationFrom`, `GetRelationTo`,
`ResolveWidgetFromType`) off `Metamodel` into an adapter at its only wiring
site, `internal/projectsetup`; make `ResolveWidgetFromType` a package function
(dataentry uses it too); delete `IsEnumType` (no callers) from `Metamodel`, the
interface and its mock; move `ValidateRelation` to `Validator`; delete the
`EntityDef` and `RelationDef` getters that return a field (`GetLabel`,
`GetAliases`, `GetIDPatterns`, `GetProperties`, `GetRDFType`, `GetColor`,
`GetBorderColor`, `GetFrom`, `GetTo`, `GetDescription`, `GetInverse`,
`GetMin*`/`GetMax*`, `IsSymmetric`, `IsOwning`) and point
`internal/cli/schema_json.go` at the concrete types; remove both plimsoll
directives. Out: `ResolveAlias`, `GetEntityDef`, `GetRelationDef`,
`EntityTypes`, `RelationTypes`, `HasEntityType`, `InverseOwner`,
`InferEntityType`, `DisplayTitle` stay: they are the lookup catalog the type is
meant to keep. The rename to `schema` (TKT-ZTGLU4). Any behaviour or YAML
change.

**Acceptance Criteria:**
1. No behaviour change, except one fix: when the schema fails to load, `rela migrate` hands the migrations a nil interface instead of an interface around a nil `*Metamodel`, so their no-metamodel checks work. Test: the full suite passes; migration tests keep their mock and assertions; `TestLoadMigrationMetamodel_MissingSchemaIsNilInterface` fails without the guard.
2. `Metamodel` exports 11 methods and `EntityDef` 17; both directives are gone. Test: `just plimsoll`.
3. `rela schema` JSON output unchanged. Test: existing cli schema JSON tests, plus a byte comparison of `rela schema --json` for overview, entities, relations, one entity and one relation on the tickets project before and after.
4. Boundaries hold. Test: `just arch-lint`; tests compile under default, memorybackend, postgres, sqlite.

## Research

- [x] ~~For larger features: run `/research`~~ (N/A: mechanical refactor with a fixed target shape from th-0e0ed196)
- [x] ~~Searched for existing libraries~~ (N/A: internal move)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: internal move)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:** Part 1 (#1855) set the pattern. TKT-OBC7QI (backlog)
diagnosed the same methods as a consumer interface implemented on the producer
and proposed an adapter inside `internal/migration`; that placement is an import
cycle (`metamodel` imports `migration`), so the adapter goes to the wiring site
instead. `StringShaped` and `IsOwning` are the precedent for package functions
over `*Metamodel`.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**
- `projectsetup.migrationSchema{meta}` implements `migration.MetamodelProvider` with the method bodies moved unchanged. `loadMigrationMetamodel` returns it, or a nil interface when the schema does not load.
- `metamodel.ResolveWidgetFromType(m, propType)` package function; nil `m` accepted (built-in types only). dataentry calls it; dataentryconfig reads `meta.Types[t].Default` directly.
- `Validator.ValidateRelation`, body unchanged.
- Field getters on `EntityDef`/`RelationDef`: deleted; callers read fields (`GetIDPatterns` callers use `GetIDPrefixes`). `schema_json.go` takes `*metamodel.EntityDef` and `*metamodel.RelationDef`; its tests build real definitions.
- Call sites are rewritten from compiler errors across all four build tags.

Alternatives rejected: a `metamodel.Catalog` type (tried first; keeps a
migration-only surface in the producer); the adapter inside `internal/migration`
(import cycle); make `migration` take `*Metamodel` (same cycle).

**Files to modify:** internal/metamodel/{schema_output.go → widget.go,
metamodel.go, validation.go, types.go} and tests;
internal/migration/{migration.go, dataentry_cleanup_test.go};
internal/projectsetup/{migrate.go, migrationschema.go};
internal/dataentry/helpers.go; internal/dataentryconfig/validate_caldav.go;
internal/cli/schema_json.go and tests; other getter callers found by the
compiler.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] ~~Input validation approach defined~~ (N/A: no new input; validation logic moves unchanged)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] ~~Error handling doesn't leak sensitive information~~ (N/A: no new error surfaces)

**Input Sources & Validation:** None new.

**Security-Sensitive Operations:** Relation validation on the write path must
call the same check as before. Mitigation: compiler-driven rewrite, one for one.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** See acceptance criteria. Full `go test ./...` plus postgres
and sqlite tag builds and their store suites.

**Edge Cases:** Unknown entity type, property or relation (empty or false, as
before); `GetInverse` returned an untyped nil for a nil `Inverse`, so the JSON
output must still print `null`.

**Negative Tests:** A missing schema yields a nil provider (new test);
`ResolveWidgetFromType(nil, …)` resolves built-ins only.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** A getter that did more than return a field (`GetInverse`,
`GetIDPatterns`): checked one by one. Missing a tagged build's caller: compile
all four tags. Effort: m.

## Documentation Planning

- [x] ~~User-facing docs identified~~ (N/A: internal refactor)
- [x] ~~Docs-checklist will be created~~ (N/A: refactor)

**Documentation Impact:** Comments that cite `Metamodel.ResolveWidgetFromType`
(dataentryconfig) and the `MetamodelProvider` doc in migration.

## Design Review

- [x] ~~Run `/design-review`~~ (N/A: the target shape was accepted in archplan th-0e0ed196 and part 1 set the pattern; this PR moves methods without changing logic)
- [x] ~~All critical/significant findings addressed in plan~~ (N/A: see above)

**Design Review Findings:** N/A
