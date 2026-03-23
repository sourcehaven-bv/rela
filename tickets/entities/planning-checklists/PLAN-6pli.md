---
id: PLAN-6pli
status: done
title: 'Planning: Add template parameter to create_entity automation action'
type: planning-checklist
---

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

**IN SCOPE:**
- Add `template` parameter to `create_entity` automation action
- Support template variants using existing `<type>--<variant>.md` naming convention
- Pass template name through automation engine to workspace

**OUT OF SCOPE:**
- Creating new template variant files (users create these manually)
- Conditional template selection based on trigger entity properties (future enhancement)
- Template inheritance or composition

**Acceptance Criteria:**

1. Can specify `template: variant-name` in create_entity action YAML
2. When template is specified, loads `<type>--<variant>.md` instead of `<type>.md`
3. When template is omitted or empty, uses default `<type>.md` (backward compatible)
4. Error reported if specified template file doesn't exist
5. Template interpolation ({{new.property}}) works in template parameter

## Approach

- [x] Codebase researched (existing patterns, related code)
- [x] Technical approach chosen and documented
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

1. Add `Template` field to `CreateEntityAction` in both packages:
   - `internal/automation/types.go` - Add `Template string` field
   - `internal/metamodel/types.go` - Add `Template string` with yaml tag

2. Add `Template` field to `EntityToCreate` result struct:
   - `internal/automation/types.go` - Add to EntityToCreate

3. Update automation engine to pass template through:
   - `internal/automation/engine.go` - Interpolate and copy template to result

4. Add `LoadEntityTemplateVariant` method to Repository interface:
   - `internal/repository/repository.go` - New method that takes (entityType, variant)
   
5. Update `createEntityNoAutomation` to accept optional template:
   - `internal/workspace/workspace.go` - Check for template, load variant if specified

**Files to modify:**

1. `internal/automation/types.go` - Add Template to CreateEntityAction and EntityToCreate
2. `internal/metamodel/types.go` - Add Template to CreateEntityAction  
3. `internal/automation/engine.go` - Pass template through in executeAction
4. `internal/repository/repository.go` - Add LoadEntityTemplateVariant method
5. `internal/workspace/workspace.go` - Use template variant in createEntityNoAutomation
6. `internal/automation/engine_test.go` - Add tests for template parameter
7. `internal/workspace/workspace_test.go` - Add integration tests

**Alternatives Considered:**

1. **Full path instead of variant name** - Rejected: breaks convention, security risk
2. **Template name includes type** - Rejected: redundant, type already known
3. **Interpolate template name** - CHOSEN: allows `{{new.kind}}` for dynamic templates

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

| AC | Test Scenario |
|----|--------------|
| 1 | Parse YAML with `template: my-variant`, verify CreateEntityAction.Template populated |
| 2 | Create automation with template, verify `<type>--<variant>.md` loaded |
| 3 | Create automation without template, verify `<type>.md` loaded (existing tests) |
| 4 | Specify non-existent template, verify error in AutomationErrors |
| 5 | Use `template: "{{new.kind}}"`, verify interpolation works |

**Edge Cases:**

- Template field empty string vs omitted (both use default)
- Template variant file missing (graceful error, not panic)
- Template name with special characters (reject or sanitize)
- Default template missing but variant specified (still works if variant exists)

## Risk Assessment

- [x] Risks assessed with mitigations
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

| Risk | Mitigation |
|------|------------|
| Breaking existing automations | Template field is optional, empty = default behavior |
| Path traversal in template name | Validate template name, reject `..` or `/` |
| Performance (extra file check) | Minimal impact, only on automation trigger |

**Effort:** S (small) - straightforward parameter threading
