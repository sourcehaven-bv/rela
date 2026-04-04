---
id: PLAN-2AMA
type: planning-checklist
title: 'Planning: Add Lua validation rules to metamodel'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

IN SCOPE:
- Add `lua` field to `ValidationRule` struct for inline Lua code
- Add `lua_file` field for referencing scripts in `scripts/` directory
- Execute Lua in sandboxed runtime with entity context (`entity` global)
- Provide read-only workspace access (`rela.get_entity()`, `rela.list_entities()`, `rela.get_relations()`, `rela.trace_from/to()`)
- Integrate with existing `validation.Service.checkRule()`
- Return `true` (valid) or `false`/`nil`/no-return (violation) from Lua

OUT OF SCOPE:
- Workspace mutations (create/update/delete entities/relations)
- Graph reload via `rela.refresh()` (blocked in read-only mode)
- Custom error messages from Lua (use existing `description` field)
- Caching/precompilation of Lua code
- Breaking changes to existing when/then validation rules

**Acceptance Criteria:**

1. Lua validation rule with inline code detects violations
   - Define rule with `lua: |` that checks entity property
   - Run `rela analyze validations`
   - See violation for entities where Lua returns `false`

2. Lua validation rule with script file works
   - Define rule with `lua_file: scripts/validate-dates.lua`
   - Script returns `false` for invalid entities
   - Run `rela analyze validations`, see violations

3. Entity context available in Lua
   - Access `entity.id`, `entity.type`, `entity:prop("name")`
   - Return validation result based on entity state

4. Read-only workspace access works
   - `rela.get_entity(id)` returns entity or nil
   - `rela.list_entities(type)` returns entities of type
   - `rela.get_relations({from=id})` returns relations
   - `rela.trace_from(id)` / `rela.trace_to(id)` work
   - Mutation functions NOT available (no `create_entity`, etc.)
   - `rela.refresh()` blocked (would mutate in-memory state)

5. Lua rules coexist with declarative rules
   - Rule with both `when`/`then` AND `lua` applies both
   - Rule with only `lua` (no when/then) applies Lua to all matching entities

6. Errors in Lua don't crash validation
   - Syntax error in Lua → logged warning, rule skipped
   - Runtime error → logged warning, entity passes (fail-open for safety)

## Research

- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Existing Solutions:**

- **Automation engine** (`internal/automation/engine.go:66-70`): Already supports `Lua` and `LuaFile` action fields - reuse this pattern
- **Script executor** (`internal/script/executor.go`): Provides `ExecuteCode()` and `ExecuteFile()` with entity context injection
- **Lua runtime** (`internal/lua/runtime.go`): Sandboxed runtime with `entity` global and `rela.*` bindings
- **WorkspaceInterface** (`internal/lua/workspace.go`): Defines read + write operations - we'll use read-only subset

Key insight: `lua.Runtime` already registers all bindings including read-only
ones. We need to either:
1. Create a read-only wrapper that implements `WorkspaceInterface` but panics/errors on mutations
2. Or create a new `ReadOnlyWorkspaceInterface` with only read methods

Option 1 is simpler - wrap the real workspace and block mutations at runtime.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

1. **Add fields to `ValidationRule`** (`internal/metamodel/types.go`):
   ```go
   type ValidationRule struct {
       // ... existing fields ...
       Lua     string `yaml:"lua,omitempty"`      // Inline Lua code
       LuaFile string `yaml:"lua_file,omitempty"` // Path to script in scripts/
   }
   ```

2. **Create read-only workspace wrapper** (`internal/validation/readonly_workspace.go`):
   ```go
   type readOnlyWorkspace struct {
       ws lua.WorkspaceInterface
   }

   // Read methods delegate to underlying workspace
   func (r *readOnlyWorkspace) GetEntity(id string) (*model.Entity, bool) {
       return r.ws.GetEntity(id)
   }

   // Mutation methods return errors
   func (r *readOnlyWorkspace) CreateEntityLua(...) (*model.Entity, error) {
       return nil, errors.New("mutations not allowed in validation scripts")
   }

   // SyncLua is also blocked (mutates in-memory state)
   func (r *readOnlyWorkspace) SyncLua() error {
       return errors.New("refresh not allowed in validation scripts")
   }
   ```

3. **Create validation Lua executor** (`internal/validation/lua.go`):
   - Create `lua.Runtime` with read-only workspace wrapper
   - Pass `io.Discard` as stdout to suppress `rela.output()` noise
   - Inject `entity` global using `lua.EntityToTable()`
   - Execute code, check return value:
     - `true` → pass (valid)
     - `false`, `nil`, or no return → violation
   - Handle errors gracefully (log + skip)

4. **Use functional options for Service** (`internal/validation/validation.go`):
   ```go
   type Service struct {
       meta        *metamodel.Metamodel
       ws          lua.WorkspaceInterface  // Optional, for Lua validation
       projectRoot string                   // Optional, for lua_file loading
   }

   // Keep existing signature for backwards compatibility
   func New(meta *metamodel.Metamodel) *Service

   // Add options for Lua support
   func WithWorkspace(ws lua.WorkspaceInterface) Option
   func WithProjectRoot(root string) Option
   ```

5. **Extend `entityViolatesRule()`** (`internal/validation/validation.go`):
   ```go
   func (s *Service) entityViolatesRule(...) bool {
       // ... existing when/then logic ...

       // Check Lua rule if present
       if rule.Lua != "" || rule.LuaFile != "" {
           if s.luaViolates(entity, rule) {
               return true
           }
       }

       // ... existing content rules ...
   }
   ```

**Alternative considered - rejected:**

- Create separate `ReadOnlyWorkspaceInterface` with only read methods
- Rejected: Would require duplicating interface definition and changing `lua.Runtime` to accept either interface
- Wrapper approach is simpler and uses existing interface

**Files to modify:**

1. `internal/metamodel/types.go` - Add `Lua`, `LuaFile` fields to `ValidationRule`
2. `internal/validation/validation.go` - Add functional options for workspace/projectRoot
3. `internal/validation/readonly_workspace.go` (NEW) - Read-only workspace wrapper
4. `internal/validation/lua.go` (NEW) - Lua validation executor
5. `internal/validation/lua_test.go` (NEW) - Tests for Lua validation
6. `internal/workspace/analysis.go` - Pass workspace via options to validation.New()

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

| Input | Source | Validation | On Invalid |
|-------|--------|------------|------------|
| `lua` code | metamodel.yaml | Executed in sandbox | Runtime error → skip rule |
| `lua_file` path | metamodel.yaml | `filepath.IsLocal()`, `.lua` extension required | Error → skip rule |
| Entity data | Graph (trusted) | N/A | N/A |

**Security-Sensitive Operations:**

1. **Lua execution** - Sandboxed via existing `lua.Runtime`:
   - No `io`, `os`, `debug` libraries
   - No `loadfile`, `dofile`, `load`
   - 30s timeout (existing default)

2. **File loading** - Via `script.loadScript()` pattern:
   - Scripts must be in `scripts/` directory
   - Path traversal blocked (`filepath.IsLocal()`)
   - Uses `os.OpenRoot` for traversal-resistant access

3. **Read-only workspace** - Validation Lua can:
   - ✅ Read entities (`get_entity`, `list_entities`)
   - ✅ Read relations (`get_relations`)
   - ✅ Traverse graph (`trace_from`, `trace_to`, `find_path`)
   - ✅ Search (`search`)
   - ❌ Create/update/delete entities (blocked)
   - ❌ Create/delete relations (blocked)
   - ❌ Sync/refresh graph (blocked)
   - ❌ Write files (no output dir, writes to io.Discard)

4. **Output functions** - Validation context uses `io.Discard`:
   - `rela.output()` writes to discard (no stdout noise)
   - `rela.write_file()` fails (no output dir configured)

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

| AC# | Test |
|-----|------|
| 1 | `TestLuaValidation_InlineCode` - rule with `lua: return entity:prop("status") ~= ""`|
| 2 | `TestLuaValidation_ScriptFile` - rule with `lua_file: validate.lua` |
| 3 | `TestLuaValidation_EntityContext` - access id, type, properties |
| 4 | `TestLuaValidation_ReadOnlyWorkspace` - get_entity, list_entities, get_relations work |
| 4b | `TestLuaValidation_MutationsBlocked` - create_entity etc. return errors |
| 4c | `TestLuaValidation_SyncBlocked` - refresh() returns error |
| 5 | `TestLuaValidation_CombinedWithWhenThen` - both filters and Lua |
| 6 | `TestLuaValidation_SyntaxError`, `TestLuaValidation_RuntimeError` |

**Edge Cases:**

- Empty `lua:` string (should be no-op)
- Lua returns `true` → pass
- Lua returns `false` → violation
- Lua returns `nil` → violation
- Lua returns nothing (no return statement) → violation
- Lua returns non-boolean (string, number) → pass (truthy)
- Script file not found (log warning, skip rule)
- Entity with nil properties (entity:prop returns default)
- Cross-entity lookup returns nil (entity doesn't exist)

**Negative Tests:**

- Syntax error in Lua code → skip rule, log warning
- Runtime error (nil access) → skip rule, log warning
- Timeout (infinite loop) → skip rule, log warning
- Invalid `lua_file` path → skip rule, log warning
- Mutation attempt → error returned, rule fails gracefully
- Sync/refresh attempt → error returned

**Integration Test:**

- `TestLuaValidation_Integration` - Full flow via workspace.RunValidations():
  1. Create metamodel with Lua validation rule
  2. Create entities that pass/fail the rule
  3. Run analysis, verify violations

- `TestLuaValidation_CrossEntity` - Validate cross-entity rules:
  1. Rule: "parent must be approved"
  2. Create parent (status=draft) and child (status=in-progress)
  3. Verify child violates rule

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

| Risk | Mitigation |
|------|------------|
| Lua timeout blocks validation | Use existing 30s timeout, log and skip |
| Security bypass in Lua sandbox | Reuse existing hardened `lua.Runtime` |
| Mutation attempt in validation | Read-only wrapper returns clear error |
| Performance with many cross-entity lookups | Acceptable - validation runs infrequently |

**Effort:** m (medium) - Mostly wiring existing components, plus read-only
wrapper

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] User guide / reference docs - Add Lua validation examples to metamodel docs
- [ ] CLI help text (if commands changed) - N/A
- [ ] CLAUDE.md (if new patterns) - N/A (uses existing patterns)
- [ ] README.md (if project-level changes) - N/A
- [ ] API docs (if applicable) - N/A

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:**

| ID | Severity | Finding | Resolution |
|----|----------|---------|------------|
| RR-FYX0 | significant | Breaking change to validation.New() signature | Use functional options pattern for backwards compatibility |
| RR-B0TT | significant | SyncLua() must be blocked in read-only wrapper | Added to blocked methods list, added test case |
| RR-G01Y | minor | Return value semantics unclear | Clarified: `true`=pass, `false`/`nil`/no-return=violation |
| RR-D50C | minor | output()/write_file() in validation context | Use `io.Discard` for stdout, write_file fails gracefully |
