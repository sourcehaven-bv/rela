---
id: PLAN-CI6GC
type: planning-checklist
title: 'Planning: Push markdown imports behind repository boundary'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

**In scope:**
- Move `EntityTemplate` + `TemplateRelation` to `model` package
- Move `NormalizeHeaders` to `model` package
- Consolidate duplicated `checkValidationRule` via workspace
- Move entity-type rename orchestration to workspace
- Keep `Document` in `markdown` as parse-internal type
- Change Store template methods to return `model.EntityTemplate` instead of `*markdown.Document`
- Remove `markdown` from consumer arch deps

**Out of scope:**
- Full Store interface extraction (separate ticket)
- Alternative backends (zip, WebDAV)
- Lazy content loading (noted for future optimization)
- Snapshot API (#1 from database-lessons.md)

**Acceptance Criteria:**
1. `internal/cli` has zero imports of `internal/markdown`
2. `internal/dataentry` has zero imports of `internal/markdown`
3. `internal/mcp` has zero imports of `internal/markdown`
4. `.go-arch-lint.yml` forbids `markdown` from `cli`, `dataentry`, and `mcp`
5. `go-arch-lint check` passes
6. `go test -race ./...`, `just lint`, `just coverage-check` all pass
7. No behavioral changes — all existing tests pass

## Research

- [x] ~~Searched for existing libraries that solve this problem~~ (N/A: internal refactor)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A)
- [x] Reviewed relevant rela concepts for prior art

**Existing Solutions:**
- Pattern: workspace already re-exports `ChangeEvent = repository.ChangeEvent` for consumers
- Pattern: `workspace.Rename` (entity ID rename) moved from `internal/rename` to workspace in TKT-GO601
- `validation.Service` already handles `CheckContentRule` internally — consumers should go through it

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

### Change A: Move types to `model`

Move `EntityTemplate` + `TemplateRelation` from `markdown/template.go` to
`model/template.go`. Keep `Document` in `markdown` — it's a parse-internal type.
Template Store methods will return `model.EntityTemplate` directly instead of
`*markdown.Document`.

### Change B: Move `NormalizeHeaders` to `model`

Move `markdown.NormalizeHeaders` → `model.NormalizeContentHeaders` in
`model/content.go`. Only uses regexp/strings, no goldmark dependency. CLI calls
`model.NormalizeContentHeaders()`.

### Change C: Consolidate validation via workspace

Export `validation.Service.CheckRule` (currently unexported `checkRule`). Add
`Workspace.CheckValidationRule(rule, entities) []*model.Entity` that
creates/uses `validation.Service` and converts `[]Violation` →
`[]*model.Entity`. Replace duplicated `checkValidationRule` in dataentry and mcp
with `ws.CheckValidationRule(...)`.

### Change D: Move entity-type rename to workspace

Add `Workspace.RenameEntityType(oldType, newType, plural string) error` in
`workspace/rename_type.go`. Moves the full operation from
`cli/rename.go:applyRenameEntity`: metamodel update, directory rename, file
type-field rewrite, template rename, cache invalidation.

### Change E: Arch lint

Remove `markdown` from `cli.mayDependOn`, `dataentry.mayDependOn`,
`mcp.mayDependOn`.

**Alternatives considered:**
- Thin wrappers on Workspace for every function: rejected per design review — papers over problems
- Moving `Document` to `model`: rejected — it's parse-internal, and keeping it in markdown enables
future lazy content loading where Store returns entities without parsing the
body
- Adding `validation` as dependency to dataentry/mcp: rejected — arch rules only allow workspace→validation

**Files to modify:**

| File | Change |
|---|---|
| `model/template.go` | **New** — `EntityTemplate`, `TemplateRelation` |
| `model/content.go` | **New** — `NormalizeContentHeaders` |
| `markdown/template.go` | Remove types, use `model.EntityTemplate` |
| `markdown/normalize.go` | Delegate to or keep alongside `model.NormalizeContentHeaders` |
| `markdown/parser.go` | Keep `Document` here (parse-internal) |
| `repository/repository.go` | Store interface: template methods return `model.EntityTemplate` |
| `validation/validation.go` | Export `CheckRule` |
| `workspace/workspace.go` | Add `CheckValidationRule`, update `DiscoverEntityTemplates` return |
| `workspace/rename_type.go` | **New** — `RenameEntityType` (moved from cli) |
| `cli/normalize.go` | Use `model.NormalizeContentHeaders` |
| `cli/rename.go` | Call `ws.RenameEntityType(...)` |
| `dataentry/analyze.go` | Call `ws.CheckValidationRule(...)` |
| `dataentry/app.go` | Use `model.EntityTemplate` |
| `mcp/tools_helpers.go` | Call `ws.CheckValidationRule(...)` |
| `.go-arch-lint.yml` | Remove `markdown` from consumer deps |

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] ~~Input validation approach defined~~ (N/A: no new input surfaces)
- [x] ~~Security-sensitive operations identified~~ (N/A: pure refactor)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** N/A — internal refactor, no new input surfaces.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**
1. AC1-3 (zero markdown imports): verified by `go-arch-lint check` + grep
2. AC4 (arch config): manual inspection of `.go-arch-lint.yml`
3. AC5: `go-arch-lint check` passes
4. AC6: `go test -race ./...`, `just lint`, `just coverage-check`
5. AC7: all existing tests pass without modification (behavioral no-op)

**Edge Cases:**
- Entity-type rename with no entities of that type (empty dir) — handled by existing code
- Entity-type rename with no template — handled by existing code
- Validation rule with Lua component — workspace method must handle this (validation.Service already does)
- Empty content in NormalizeHeaders — returns empty string (existing behavior preserved)

**Negative Tests:** All existing negative test cases continue to work
(behavioral no-op refactor).

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**
- `NormalizeHeaders` implementation may have hidden goldmark dependency — **verified: only uses regexp/strings**
- Entity-type rename move may break CLI tests — **mitigation: CLI rename tests exercise `runRenameEntity` which calls `applyRenameEntity`; these will need to go through workspace now**
- `validation.Service` needs Lua for some rules — **workspace already has script executor, can wire it**

**Effort: m** (medium — touches many files but each change is mechanical)

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] ~~Docs-checklist will be created when entering implementation~~ (N/A)

**Documentation Impact:**
- [x] N/A — Internal refactor, no user-facing docs needed

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:**
- RR-4RR0Y (critical, addressed): Store interface leaks markdown.Document — resolved by keeping Document in markdown, changing template methods to return model.EntityTemplate
- RR-5LY67 (critical, addressed): Entity-type rename belongs on Workspace — resolved by moving full operation
- RR-0K2KG (significant, addressed): NormalizeHeaders doesn't belong on Workspace — resolved by moving to model
- RR-N4FGY (significant, addressed): CheckContentRule should route through validation — resolved by consolidating via workspace→validation
- RR-REYDV (significant, addressed): Duplicated checkValidationRule is root cause — resolved by consolidation
