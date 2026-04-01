---
id: PLAN-8Q6K
type: planning-checklist
title: 'Planning: Add Lua action type to automation engine'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

**In scope:**
- New `lua` action type for inline Lua code in automations
- New `lua_file` action type for referencing script files
- Access to triggering entity context (`entity`, `old_entity`) as Lua globals
- Integration with existing rela Lua bindings (get_entity, update_entity, etc.)
- Safe template interpolation for non-entity values only (`{{today}}`, `{{user.name}}`)

**Out of scope:**
- New Lua bindings (use existing ones from `internal/lua/`)
- Async/background execution
- `rela automation test` command (follow-up ticket)

**Acceptance Criteria:**

1. **AC1: Inline Lua execution** - A `lua:` action in metamodel.yaml executes inline Lua code when the automation triggers
2. **AC2: Script file execution** - A `lua_file:` action executes a script from `scripts/` directory
3. **AC3: Entity context available** - The triggering entity is accessible as `entity` global in Lua with id, type, properties
4. **AC4: Old entity context** - For update events, `old_entity` global is available with previous state
5. **AC5: Safe interpolation** - Only safe values (`{{today}}`, `{{user.name}}`) are interpolated; entity properties are accessed via Lua globals
6. **AC6: Mutation via bindings** - Lua can call `rela.update_entity()`, `rela.create_entity()`, `rela.create_relation()` and changes are applied
7. **AC7: Error handling** - Lua errors are captured and returned as automation errors (not panics)
8. **AC8: Security** - Sandbox restrictions preserved; script files must be in `scripts/` directory with os.OpenRoot validation

## Research

- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Existing Solutions:**

- **Lua runtime**: Already exists at `internal/lua/runtime.go` with full rela bindings
- **Script file loading**: `internal/mcp/tools_lua.go` has secure path handling with os.OpenRoot
- **Automation engine**: `internal/automation/engine.go` has clear pattern for action execution
- **Action types**: `set`, `create_relation`, `create_entity` show the pattern in `types.go:32-60`
- **Template interpolation**: `template.go` provides `Interpolate()` function

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach (Revised after design review):**

### Key Design Decision: Lua execution in workspace layer

Due to circular import constraints (`automation → lua → workspace → automation`), Lua execution must happen in the workspace layer, not the automation engine. The automation engine will return `LuaToExecute` in its Result, and `workspace.applyAutomationSideEffects()` will execute it.

### Key Design Decision: No template interpolation in Lua code

To prevent injection attacks, entity properties are NOT interpolated into Lua code. Instead:
- Entity context is provided as Lua globals (`entity`, `old_entity`)
- Only safe values (`{{today}}`, `{{user.name}}`) are interpolated

### Implementation Steps:

1. **Add Lua fields to Action struct** (`internal/automation/types.go`):
   ```go
   type Action struct {
       // ... existing fields ...
       Lua     string  // Inline Lua code
       LuaFile string  // Path to script in scripts/
   }
   ```

2. **Add LuaToExecute to Result** (`internal/automation/types.go`):
   ```go
   type LuaToExecute struct {
       Code     string  // Inline code (already interpolated for safe values)
       FilePath string  // Script file path (mutually exclusive with Code)
   }
   
   type Result struct {
       // ... existing fields ...
       LuaToExecute []LuaToExecute
   }
   ```

3. **Update executeAction** (`internal/automation/engine.go`):
   - For `lua:` action: interpolate safe values only, add to `result.LuaToExecute`
   - For `lua_file:` action: validate path format, add to `result.LuaToExecute`

4. **Execute Lua in workspace** (`internal/workspace/workspace.go`):
   - In `applyAutomationSideEffects()`, process `LuaToExecute` entries
   - Create Lua runtime with workspace access
   - Set `entity` and `old_entity` globals using exported `EntityToTable()`
   - For file paths: use `os.OpenRoot` pattern from `tools_lua.go`
   - Execute and capture errors

5. **Export entityToTable** (`internal/lua/runtime.go`):
   - Rename `entityToTable` → `EntityToTable` for use by workspace

**Example Usage:**

```yaml
automations:
  # Inline - entity context via Lua globals, NOT template interpolation
  - name: cascade-status
    on:
      entity: [feature]
      property: status
      becomes: done
    do:
      - lua: |
          local rels = rela.get_relations({to = entity.id, type = "implements"})
          for _, rel in ipairs(rels) do
            rela.update_entity(rel.from, {properties = {status = "done"}})
          end

  # Script file for complex logic
  - name: auto-assign
    on:
      entity: [ticket]
      created: true
    do:
      - lua_file: automations/auto-assign.lua
```

**Files to modify:**

1. `internal/automation/types.go` - Add `Lua`, `LuaFile` fields to Action; add `LuaToExecute` to Result
2. `internal/metamodel/types.go` - Add `Lua`, `LuaFile` fields to AutomationAction
3. `internal/automation/engine.go` - Handle Lua actions, add to result (no execution)
4. `internal/lua/runtime.go` - Export `EntityToTable`
5. `internal/workspace/workspace.go` - Execute Lua in `applyAutomationSideEffects()`
6. `internal/automation/engine_test.go` - Add tests for Lua action result generation
7. `internal/workspace/workspace_test.go` - Add tests for Lua execution

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

| Input | Source | Validation |
|-------|--------|------------|
| Inline Lua code | metamodel.yaml (trusted) | Sandboxed execution |
| Script file path | metamodel.yaml (trusted) | os.OpenRoot traversal-resistant access |
| Entity properties | Graph (untrusted) | NOT interpolated; accessed via Lua globals |

**Security-Sensitive Operations:**

- **Lua sandbox**: Reuses existing hardened sandbox from `internal/lua/runtime.go`
- **Path validation**: os.OpenRoot + `.lua` extension required (pattern from tools_lua.go)
- **No injection**: Entity properties never interpolated into Lua code strings
- **Entity mutations**: Go through workspace layer which validates against metamodel

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

| AC | Test |
|----|------|
| AC1 | Automation with `lua:` action fires on property change, code executes |
| AC2 | Automation with `lua_file:` loads and executes script from scripts/ |
| AC3 | Lua code can read `entity.id`, `entity.type`, `entity.properties.X` |
| AC4 | On update event, `old_entity.properties.X` has previous value |
| AC5 | `{{today}}` interpolates; `{{new.title}}` does NOT (use `entity.properties.title`) |
| AC6 | `rela.update_entity()` in Lua changes entity, observable after automation |
| AC7 | Syntax error in Lua returns error in Result.Errors, doesn't panic |
| AC8 | `lua_file: ../secret.lua` rejected; symlink escapes blocked |

**Edge Cases:**

- Empty Lua code string - should be no-op
- Empty lua_file path - error 'lua_file path cannot be empty'
- Script file not found - clear error message
- Script file without `.lua` extension - rejected
- Entity with nil properties map - properties global is empty table
- Malicious property value (Lua code fragment) - harmless, accessed as string via global

**Negative Tests:**

- Invalid Lua syntax → error in Result.Errors
- Runtime Lua error (nil access) → error in Result.Errors
- Path traversal attempt (`../`) → error via os.OpenRoot
- Symlink escape → blocked by os.OpenRoot
- Non-existent script file → error in Result.Errors

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| Infinite loop in Lua | Medium | High (hangs) | Follow-up: add instruction limit |
| Circular automation triggers | Medium | Medium | Existing maxAutomationDepth limit |
| Path traversal | Low | High | os.OpenRoot API |
| Injection via properties | Low | Medium | Properties as globals, not interpolated |

**Effort:** m (medium)

## Follow-up Tickets

- `rela automation test` command for dry-run testing (with in-memory workspace that captures changes without persisting)

## Documentation Planning

- [x] User-facing docs identified
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] CLAUDE.md - Add `lua` and `lua_file` action types to automation syntax
- [ ] User guide - Document Lua automation actions (rela-docs)

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:**

| ID | Severity | Title | Status |
|----|----------|-------|--------|
| RR-1PWJ | significant | Template interpolation injection | addressed - use globals instead |
| RR-PDIR | significant | Circular import | addressed - execute in workspace layer |
| RR-NRK1 | minor | Use os.OpenRoot for paths | addressed |
| RR-PI27 | minor | Export entityToTable | addressed |
| RR-XJYE | nit | Empty lua_file handling | addressed |
