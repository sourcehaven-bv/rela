---
id: PLAN-pwbl
status: done
title: 'Planning: Add metamodel cleanup/trim command'
type: planning-checklist
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

**IN SCOPE:**
- New `rela analyze schema` CLI subcommand
- New MCP tool `analyze_schema` 
- Report entity types with zero instances
- Report relation types with zero instances
- Report custom types (enums) that are not referenced by any property
- Report types with few instances (configurable `--threshold` flag, default 0)
- Show where types are referenced (data-entry.yaml, views.yaml, validations, automations)
- `--cleanup` flag to remove unused types from metamodel.yaml
- Auto-update data-entry.yaml when removing types (remove forms/lists/views referencing them)
- Auto-update views.yaml when removing types (remove views referencing them)
- `--dry-run` flag to preview changes
- JSON output support (consistent with other analyze commands)

**OUT OF SCOPE:**
- Interactive selection of types to remove (use --cleanup with explicit type filtering instead)
- Undo/rollback functionality (users should use git for this)
- Backup file creation (git provides this)

**Acceptance Criteria:**

1. **AC1: Show unused entity types** - `rela analyze schema` shows entity types with 0 instances
   - Test: Create metamodel with 3 entity types, add instances for 2, verify 1 shows as unused

2. **AC2: Show unused relation types** - Shows relation types with 0 instances
   - Test: Create metamodel with 3 relation types, create relations for 2, verify 1 shows as unused

3. **AC3: Show unused custom types** - Shows custom types (enums) not referenced by any property
   - Test: Create metamodel with 3 custom types, use 2 in properties, verify 1 shows as unused

4. **AC4: Threshold filtering** - `--threshold N` shows types with ≤N instances
   - Test: With types having 0, 2, 5 instances, `--threshold 2` shows first two

5. **AC5: Reference tracking** - Shows where types are referenced in data-entry.yaml and views.yaml
   - Test: Create unused type that's referenced in a form, output shows the form reference

6. **AC6: Cleanup removes from metamodel** - `--cleanup` removes unused types from metamodel.yaml
   - Test: Run cleanup on unused type, verify metamodel.yaml no longer contains it

7. **AC7: Cleanup removes unused enums** - `--cleanup` removes unused custom types from metamodel.yaml
   - Test: Run cleanup on unused enum, verify metamodel.yaml types section no longer contains it

8. **AC8: Cleanup updates data-entry.yaml** - Forms/lists referencing removed types are removed
   - Test: Remove entity type, verify referencing form is removed from data-entry.yaml

9. **AC9: Cleanup updates views.yaml** - Views referencing removed types are removed
   - Test: Remove entity type, verify referencing view is removed from views.yaml

10. **AC10: Dry-run mode** - `--dry-run` shows what would change without modifying files
    - Test: Run with --dry-run, verify no files modified but output shows planned changes

11. **AC11: JSON output** - `-o json` produces structured JSON output
    - Test: Run with -o json, verify valid JSON with expected structure

12. **AC12: MCP tool** - `analyze_schema` MCP tool provides same functionality
    - Test: Call via MCP, verify same results as CLI

## Research

- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Existing Solutions:**

- **Similar CLI commands**: `internal/cli/analyze.go` has `analyze orphans`, `analyze cardinality`, etc. - same subcommand pattern
- **YAML AST manipulation**: `internal/migration/yaml_util.go` has `DeleteMapKey()`, `GetMapValue()`, etc.
- **Graph queries**: `graph.NodesByType()`, `graph.RelationsOfType()` for counting instances
- **Metamodel access**: `meta.EntityTypes()`, `meta.RelationTypes()`, `meta.Types` for type enumeration
- **Data entry config**: `internal/dataentryconfig/config.go` has `Config` struct with Forms, Lists, Views
- **Views config**: `internal/views/types.go` has `File` struct with ViewDef
- **MCP tools**: `internal/mcp/tools_schema.go` has `list_entity_types` pattern to follow

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

### Phase 1: Analysis Logic (new package `internal/schema`)

Create `internal/schema/analyze.go`:
```go
type SchemaAnalysis struct {
    UnusedEntityTypes   []TypeUsage
    UnusedRelationTypes []TypeUsage
    UnusedCustomTypes   []CustomTypeUsage  // NEW: unused enums
    LowUsageEntityTypes []TypeUsage
    LowUsageRelationTypes []TypeUsage
}

type TypeUsage struct {
    Name       string
    Count      int      // instance count
    References []Reference // where it's used
}

type CustomTypeUsage struct {
    Name       string
    References []Reference // properties that use this type
}

type Reference struct {
    File    string // "data-entry.yaml", "views.yaml", "metamodel.yaml"
    Section string // e.g., "forms.ticket", "views.ticket-context", "entities.ticket.properties.status"
    Kind    string // "form", "list", "view", "validation", "automation", "property"
}

func Analyze(meta *metamodel.Metamodel, graph *graph.Graph, 
             dataEntry *dataentryconfig.Config, views *views.File,
             threshold int) *SchemaAnalysis
```

### Phase 2: Cleanup Logic (in `internal/schema`)

Create `internal/schema/cleanup.go`:
```go
type CleanupPlan struct {
    MetamodelChanges   []Change
    DataEntryChanges   []Change  
    ViewsChanges       []Change
}

type Change struct {
    File   string
    Action string // "remove_entity_type", "remove_relation_type", "remove_custom_type", "remove_form", etc.
    Target string // what's being removed
}

func PlanCleanup(analysis *SchemaAnalysis, entityTypes, relationTypes, customTypes []string) *CleanupPlan
func ExecuteCleanup(plan *CleanupPlan, projectCtx *project.Context) error
```

YAML modification via AST (using migration/yaml_util patterns):
- Load file with `yaml.Unmarshal` into `*yaml.Node`
- Navigate to entities/relations/types maps
- Use `DeleteMapKey()` to remove types
- Write back preserving comments

### Phase 3: CLI Command

Add to `internal/cli/analyze.go`:
```go
var analyzeSchemaCmd = &cobra.Command{
    Use:   "schema",
    Short: "Analyze metamodel schema usage",
    Long:  "Show unused or underused entity and relation types",
    RunE: func(cmd *cobra.Command, args []string) error {
        // 1. Load data-entry.yaml if exists
        // 2. Load views.yaml if exists
        // 3. Run schema.Analyze()
        // 4. If --cleanup: run schema.PlanCleanup() then schema.ExecuteCleanup()
        // 5. Output results (table or JSON)
    },
}
```

Flags:
- `--threshold int` (default 0) - show types with ≤N instances
- `--cleanup` - remove unused types
- `--dry-run` - preview changes without writing
- `-o json` - JSON output (inherited from analyze parent)

### Phase 4: MCP Tool

Add to `internal/mcp/tools_schema.go`:
```go
func (s *Server) handleAnalyzeSchema(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error)
```

**Files to modify:**

1. `internal/schema/analyze.go` (NEW) - analysis logic
2. `internal/schema/cleanup.go` (NEW) - cleanup logic  
3. `internal/schema/analyze_test.go` (NEW) - unit tests
4. `internal/schema/cleanup_test.go` (NEW) - unit tests
5. `internal/cli/analyze.go` - add `analyzeSchemaCmd` subcommand
6. `internal/cli/analyze_test.go` - add CLI tests
7. `internal/mcp/tools.go` - register new tool
8. `internal/mcp/tools_schema.go` - add handler (or new file `tools_analyze.go`)

**Alternatives considered:**

1. **Add to existing workspace package** - Rejected: schema analysis is distinct from workspace operations
2. **Put cleanup in migration package** - Rejected: migrations are for schema evolution, not cleanup
3. **Interactive TUI for selection** - Rejected: CLI flags provide sufficient control, keeps scope small

**Dependencies:**

- `internal/metamodel` - Metamodel, EntityDef, RelationDef, CustomType
- `internal/graph` - Graph queries
- `internal/dataentryconfig` - Config loading
- `internal/views` - Views loading
- `internal/migration` - yaml_util.go helpers
- `internal/project` - Context for file paths
- `gopkg.in/yaml.v3` - YAML AST manipulation

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

| Input | Source | Validation |
|-------|--------|------------|
| `--threshold` | CLI flag | Integer ≥ 0, Cobra handles type |
| `--cleanup` | CLI flag | Boolean, no validation needed |
| `--dry-run` | CLI flag | Boolean, no validation needed |
| Entity/relation names | From metamodel.yaml | Already validated by metamodel loader |
| File paths | From project.Context | Already validated by project discovery |

**Security-Sensitive Operations:**

| Operation | Protection |
|-----------|------------|
| File writes (metamodel.yaml, data-entry.yaml, views.yaml) | Only writes to known project files, never arbitrary paths |
| File deletion | Not performed - only modifies content |

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

| AC | Unit Test | Integration Test |
|----|-----------|------------------|
| AC1 | `TestAnalyze_UnusedEntityTypes` | CLI: `rela analyze schema` with fixture |
| AC2 | `TestAnalyze_UnusedRelationTypes` | CLI: same fixture |
| AC3 | `TestAnalyze_UnusedCustomTypes` | CLI: same fixture |
| AC4 | `TestAnalyze_ThresholdFiltering` | CLI: `--threshold 2` |
| AC5 | `TestAnalyze_ReferencesInDataEntry` | Fixture with data-entry.yaml |
| AC6 | `TestCleanup_RemovesEntityFromMetamodel` | CLI: `--cleanup` + verify file |
| AC7 | `TestCleanup_RemovesEnumFromMetamodel` | CLI: `--cleanup` + verify file |
| AC8 | `TestCleanup_UpdatesDataEntry` | CLI: cleanup + verify data-entry.yaml |
| AC9 | `TestCleanup_UpdatesViews` | CLI: cleanup + verify views.yaml |
| AC10 | `TestCleanup_DryRun` | CLI: `--dry-run` + verify no changes |
| AC11 | `TestAnalyze_JSONOutput` | CLI: `-o json` + parse output |
| AC12 | `TestMCP_AnalyzeSchema` | MCP tool call |

**Edge Cases:**

1. Empty metamodel (no entities/relations defined) - should report "no types defined"
2. All types in use - should report "all types in use"
3. No data-entry.yaml exists - should skip data-entry reference checking
4. No views.yaml exists - should skip views reference checking
5. Type referenced in validation rules - should show in references
6. Type referenced in automations - should show in references
7. Cleanup would break cardinality constraints - should warn but allow
8. Unicode in type names - should handle correctly
9. Custom type used by another custom type (nested enums) - should detect reference

**Negative Tests:**

1. `--threshold -1` - should error with "threshold must be non-negative"
2. `--cleanup` without unused types - should succeed with "nothing to clean"
3. Malformed data-entry.yaml - should warn and continue analysis
4. Malformed views.yaml - should warn and continue analysis

**Integration Test Approach:**

Use test fixtures in `testdata/schema/` with:
- `fixture-basic/` - basic metamodel with unused types
- `fixture-with-dataentry/` - includes data-entry.yaml
- `fixture-with-views/` - includes views.yaml
- `fixture-complex/` - has validations and automations referencing types

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

| Risk | Impact | Likelihood | Mitigation |
|------|--------|------------|------------|
| YAML formatting lost on rewrite | Medium | Low | Use yaml.Node AST manipulation, not struct marshal |
| Cleanup breaks validation rules | Medium | Medium | Detect references in validations, warn before cleanup |
| Cleanup breaks automations | Medium | Medium | Detect references in automations, warn before cleanup |
| Test coverage insufficient | Low | Medium | Write comprehensive unit and integration tests |

**Effort:** M (medium) - Already estimated on ticket

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** Plan approved by user, enum cleanup added to scope.
