---
id: PLAN-OZ0M5H
type: planning-checklist
title: 'Planning: Split metamodel.Metamodel into focused types'
started: "2026-10-11"
completed: "2026-10-11"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** In (part 1 of 2): delete `GetVersion`, `GetNamespace`, `GetEntities`,
`GetRelations`, `GetTypes`; move `ValidateProperties`, `ValidateEntity`,
`ValidateRelationProperties`, `ValidatePropertyValue` to a
`metamodel.Validator`; move `RenderProjection` and `ShapeProjection` to
constructors `NewRenderProjection(m)` and `NewShapeProjection(m)`; lower the
plimsoll directive from 32 to the new count (21). Out (part 2, next PR):
relation queries, property-type queries, `EntityDef` review, removing the
directive. Out entirely: the rename to `schema`; any behaviour or YAML change.

**Acceptance Criteria:**
1. No behaviour change. Test: the full test suite passes unchanged except for call-site rewrites; validation and projection tests keep their assertions and table data.
2. Projection hashes unchanged. Test: the constructor bodies are byte-identical to the old method bodies (diff shows only the signature); projection tests and the version and migration suites that compare stored hashes pass unchanged.
3. `Metamodel` exports 21 methods and the directive says 21. Test: `just plimsoll`.
4. Package boundaries hold. Test: `just arch-lint`, `go build` with each tag (default, memorybackend, postgres, sqlite).

## Research

- [x] ~~For larger features: run `/research`~~ (N/A: mechanical refactor with a fixed target shape from th-0e0ed196)
- [x] ~~Searched for existing libraries~~ (N/A: internal move)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: internal move)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:** `RenderProjection` and `ShapeProjection` are already
value types with their own methods (`Hash`, `CompareShapes`); only their
construction sits on `Metamodel`. Validation reads only `GetEntityDef`,
`Relations` and `Types`.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**
- `type Validator struct{ meta *Metamodel }`; `NewValidator(m *Metamodel) Validator` panics on nil, because every caller already holds a loaded metamodel and a nil one is a wiring bug. The four exported methods and `validatePropertyValue` move onto it unchanged.
- `NewRenderProjection(m *Metamodel) RenderProjection` and `NewShapeProjection(m *Metamodel) ShapeProjection` replace the methods; bodies unchanged.
- Getter callers read the fields. `internal/cli/schema_json.go` replaces its `schemaMetamodel` interface with a plain `schemaOverview` struct filled by the caller; `internal/mcp/resources.go` and `prompts.go` read fields.
- Call sites are found by the compiler, not by text search: other types (`datamigration.State`, the store) also have a `ShapeProjection` method.

Alternatives rejected: free functions `ValidateEntity(m, ...)` (methods under
another name; no home for part 2 to build on); `m.Validator()` accessor (leaves
the behaviour reachable from Metamodel); a `New*` that returns an error (about
20 call sites gain dead error branches for a condition no caller can reach).

**Files to modify:** internal/metamodel/{validation.go, projection.go,
shapeprojection.go, schema_output.go, types.go} and tests; callers in
internal/{cli, mcp, entitymanager, store/*, conflict, importer, dataentry,
schema, datamigration, configedit, appbuild}, cmd/rela-server/configure.go.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] ~~Input validation approach defined~~ (N/A: no new input; validation logic moves unchanged)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] ~~Error handling doesn't leak sensitive information~~ (N/A: no new error surfaces)

**Input Sources & Validation:** None new.

**Security-Sensitive Operations:** Write-path validation in entitymanager and
the stores must call the same checks as before; a dropped call would admit
invalid data. Mitigation: compiler-driven rewrite, one-for-one.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** See acceptance criteria. Full `go test ./...` plus postgres
and sqlite tag builds and their store suites.

**Edge Cases:** Alias-resolved entity types in `ValidateEntity` (goes through
`GetEntityDef`); relation types without properties; custom types in `Types`.

**Negative Tests:** `NewValidator(nil)` panics (new unit test).

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** Merge churn with other open work touching validation callers: small,
one-line rewrites. Missing a tagged build's caller: build all four tags. Effort:
l (with part 2).

## Documentation Planning

- [x] ~~User-facing docs identified~~ (N/A: internal refactor)
- [x] ~~Docs-checklist will be created~~ (N/A: refactor)

**Documentation Impact:** Comments that cite `Metamodel.RenderProjection` and
similar, in code and .claude/rules.

## Design Review

- [x] ~~Run `/design-review`~~ (N/A: the target shape was reviewed and accepted in archplan th-0e0ed196; this PR moves methods without changing logic)
- [x] ~~All critical/significant findings addressed in plan~~ (N/A: see above)

**Design Review Findings:** N/A
