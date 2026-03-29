---
id: PLAN-5R3L
status: done
title: 'Planning: Sort frontmatter by metamodel order'
type: planning-checklist
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**
- IN: Entity frontmatter property ordering based on metamodel definition
- IN: `rela fmt` CLI command with `--dry-run` and `--check` flags
- OUT: Relation file formatting (future work)
- OUT: Automatic formatting on save (would require hooks)

**Acceptance Criteria:**
1. Properties appear in order: `id`, `type`, then metamodel-defined order, then extras alphabetically
2. `rela fmt` formats all or specific entity types
3. `rela fmt --dry-run` shows what would change without writing
4. `rela fmt --check` exits 1 if files need formatting (for CI)

## Research

- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Existing Solutions:**
- gopkg.in/yaml.v3 provides `yaml.Node` for preserving/controlling YAML key order
- Existing `FormatDocument` in `markdown/parser.go:70` used as base pattern
- Go maps don't preserve order, so need explicit key order tracking
- `WriteEntity` already supports property order parameter (added in this work)

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**
1. Extract property definition order from metamodel YAML using `yaml.Node` during loading
2. Store order in `EntityDef.PropertyOrder` field
3. Add `FormatDocumentOrdered` that builds yaml.Node with explicit key order
4. Update `WriteEntity` to pass property order through the layers
5. Add `rela fmt` command using workspace.FormatEntity

**Alternatives considered:**
- Using YAML comments to preserve order: Rejected - comments not semantic
- Alphabetical order: Rejected - metamodel order is more meaningful
- Third-party YAML library: Rejected - yaml.v3 Node support is sufficient

**Files to modify:**
- `internal/metamodel/types.go` - Add PropertyOrder field
- `internal/metamodel/loader.go` - Extract order via yaml.Node
- `internal/metamodel/entity_def.go` - GetPropertyOrder method
- `internal/markdown/parser.go` - FormatDocumentOrdered, marshalOrdered, valueToNode
- `internal/markdown/entity.go` - FormatEntity, update WriteEntity signature
- `internal/repository/repository.go` - Pass property order to WriteEntity
- `internal/repository/transaction.go` - Pass property order to WriteEntity
- `internal/workspace/workspace.go` - Add FormatEntity method
- `internal/cli/fmt.go` - New fmt command
- `internal/cli/root.go` - Handle ExitError
- `internal/errors/errors.go` - Add ExitError type

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**
- Entity files: Already validated by metamodel during load
- Metamodel YAML: Parsed by existing loader with validation
- No direct user input - command operates on existing project files

**Security-Sensitive Operations:**
- File reads: Through workspace/repository layers with SafeFS
- File writes: Through workspace/repository layers, same paths as existing files
- No network access, no auth, no crypto

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**
1. FormatDocumentOrdered produces correct key order (unit test)
2. GetPropertyOrder returns defensive copy (unit test)
3. ExitError works correctly (unit test)
4. `rela fmt --check` exits 1 when files need formatting (manual/integration)
5. `rela fmt` then `--check` exits 0 (manual/integration)

**Edge Cases:**
- Entity with no properties defined in metamodel: Uses alphabetical order
- Entity with extra properties not in metamodel: Extras sorted alphabetically at end
- Empty entity content: Should work (just frontmatter)
- Metamodel without property order: Falls back gracefully

**Negative Tests:**
- Invalid entity type argument: Returns error
- yaml.Node parse failure: Now returns error (was silent, fixed per RR-XD6H)

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**
- Risk: yaml.Node API complexity → Mitigated by thorough testing
- Risk: Breaking existing frontmatter → Mitigated by --dry-run flag and idempotent formatting
- Effort: **S** (small) - Straightforward YAML manipulation with existing patterns

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:**
- RR-MWZR (critical): valueToNode marshal/unmarshal inefficiency → Fixed: use yaml.Node.Encode()
- RR-XD6H (significant): extractPropertyOrder silently swallows errors → Fixed: returns error
- RR-8B12 (minor): GetPropertyOrder aliasing → Fixed: returns defensive copy
