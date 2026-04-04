package validation

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/graph"
	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/model"
)

// mockWorkspace implements lua.WorkspaceInterface for testing.
type mockWorkspace struct {
	graph *graph.Graph
	meta  *metamodel.Metamodel
}

func newMockWorkspace() *mockWorkspace {
	g := graph.New()
	g.AddNode(&model.Entity{
		ID:   "TKT-001",
		Type: "ticket",
		Properties: map[string]interface{}{
			"title":  "Valid ticket",
			"status": "ready",
		},
	})
	g.AddNode(&model.Entity{
		ID:   "TKT-002",
		Type: "ticket",
		Properties: map[string]interface{}{
			"title":  "Invalid ticket",
			"status": "draft",
		},
	})
	g.AddNode(&model.Entity{
		ID:   "PARENT-001",
		Type: "ticket",
		Properties: map[string]interface{}{
			"title":  "Parent ticket",
			"status": "approved",
		},
	})
	g.AddEdge(&model.Relation{
		From: "TKT-001",
		Type: "child-of",
		To:   "PARENT-001",
	})

	return &mockWorkspace{
		graph: g,
		meta: &metamodel.Metamodel{
			Entities: map[string]metamodel.EntityDef{
				"ticket": {
					Properties: map[string]metamodel.PropertyDef{
						"title":  {Type: "string"},
						"status": {Type: "string"},
					},
				},
			},
		},
	}
}

// Entity queries
func (m *mockWorkspace) GetEntity(id string) (*model.Entity, bool) {
	return m.graph.GetNode(id)
}

func (m *mockWorkspace) EntitiesByType(entityType string) []*model.Entity {
	return m.graph.NodesByType(entityType)
}

// Entity mutations
func (m *mockWorkspace) CreateEntityLua(_, _ string, _ map[string]interface{}, _ string) (*model.Entity, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockWorkspace) UpdateEntityLua(_, _ *model.Entity) error {
	return fmt.Errorf("not implemented")
}

func (m *mockWorkspace) DeleteEntityLua(_, _ string, _ bool) error {
	return fmt.Errorf("not implemented")
}

// Relation queries
func (m *mockWorkspace) AllRelations() []*model.Relation {
	return m.graph.AllEdges()
}

// Relation mutations
func (m *mockWorkspace) CreateRelationLua(_, _, _ string) (*model.Relation, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockWorkspace) DeleteRelation(_, _, _ string) error {
	return fmt.Errorf("not implemented")
}

// Graph operations
func (m *mockWorkspace) TraceFrom(id string, maxDepth int) *model.TraceResult {
	return m.graph.TraceFrom(id, maxDepth)
}

func (m *mockWorkspace) TraceTo(id string, maxDepth int) *model.TraceResult {
	return m.graph.TraceTo(id, maxDepth)
}

func (m *mockWorkspace) FindPath(from, to string) []model.PathStep {
	return m.graph.FindPath(from, to)
}

// Search
func (m *mockWorkspace) SearchSimple(query string, limit int) ([]*model.Entity, error) {
	var results []*model.Entity
	query = strings.ToLower(query)
	for _, e := range m.graph.AllNodes() {
		title := strings.ToLower(e.GetString("title"))
		if strings.Contains(title, query) {
			results = append(results, e)
			if len(results) >= limit {
				break
			}
		}
	}
	return results, nil
}

// Sync
func (m *mockWorkspace) SyncLua() error {
	return fmt.Errorf("not implemented")
}

// Verify mockWorkspace implements lua.WorkspaceInterface
var _ lua.WorkspaceInterface = (*mockWorkspace)(nil)

func TestLuaValidation_SingleViolation(t *testing.T) {
	ws := newMockWorkspace()
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"ticket": {
				Properties: map[string]metamodel.PropertyDef{
					"status": {Type: "string"},
				},
			},
		},
		Validations: []metamodel.ValidationRule{
			{
				Name:        "status-not-empty",
				Description: "Status must not be empty",
				EntityType:  "ticket",
				Lua: `
					local status = entity.properties.status
					if status == nil or status == "" then
						return { message = "Status is required" }
					end
					return nil
				`,
				Severity: "error",
			},
		},
	}

	entities := []*model.Entity{
		{
			ID:         "TKT-001",
			Type:       "ticket",
			Properties: map[string]interface{}{"status": "ready"},
		},
		{
			ID:         "TKT-002",
			Type:       "ticket",
			Properties: map[string]interface{}{"status": ""},
		},
	}

	svc := New(meta, WithWorkspace(ws), WithProjectRoot(t.TempDir()))
	violations := svc.Check(entities, nil)

	if len(violations) != 1 {
		t.Fatalf("got %d violations, want 1", len(violations))
	}
	if violations[0].EntityID != "TKT-002" {
		t.Errorf("violation entity = %s, want TKT-002", violations[0].EntityID)
	}
	if violations[0].Description != "Status is required" {
		t.Errorf("violation description = %q, want %q", violations[0].Description, "Status is required")
	}
}

func TestLuaValidation_MultipleViolations(t *testing.T) {
	ws := newMockWorkspace()
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"ticket": {
				Properties: map[string]metamodel.PropertyDef{
					"status": {Type: "string"},
					"owner":  {Type: "string"},
				},
			},
		},
		Validations: []metamodel.ValidationRule{
			{
				Name:        "required-fields",
				Description: "Required fields check",
				EntityType:  "ticket",
				Lua: `
					local issues = {}
					if entity.properties.status == nil or entity.properties.status == "" then
						table.insert(issues, { message = "Status is required", severity = "error" })
					end
					if entity.properties.owner == nil or entity.properties.owner == "" then
						table.insert(issues, { message = "Owner is required", severity = "warning" })
					end
					if #issues > 0 then
						return issues
					end
					return nil
				`,
				Severity: "error",
			},
		},
	}

	entities := []*model.Entity{
		{
			ID:         "TKT-001",
			Type:       "ticket",
			Properties: map[string]interface{}{}, // missing both status and owner
		},
	}

	svc := New(meta, WithWorkspace(ws), WithProjectRoot(t.TempDir()))
	violations := svc.Check(entities, nil)

	if len(violations) != 2 {
		t.Fatalf("got %d violations, want 2", len(violations))
	}

	// Check we got both violations with correct severities
	foundStatus, foundOwner := false, false
	for _, v := range violations {
		if v.Description == "Status is required" && v.Severity == "error" {
			foundStatus = true
		}
		if v.Description == "Owner is required" && v.Severity == "warning" {
			foundOwner = true
		}
	}
	if !foundStatus {
		t.Error("missing 'Status is required' violation with severity error")
	}
	if !foundOwner {
		t.Error("missing 'Owner is required' violation with severity warning")
	}
}

func TestLuaValidation_SeverityOverride(t *testing.T) {
	ws := newMockWorkspace()
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"ticket": {Properties: map[string]metamodel.PropertyDef{}},
		},
		Validations: []metamodel.ValidationRule{
			{
				Name:       "test-rule",
				EntityType: "ticket",
				Lua:        `return { message = "Custom warning", severity = "warning" }`,
				Severity:   "error", // default is error, but Lua overrides to warning
			},
		},
	}

	entities := []*model.Entity{
		{ID: "TKT-001", Type: "ticket", Properties: map[string]interface{}{}},
	}

	svc := New(meta, WithWorkspace(ws), WithProjectRoot(t.TempDir()))
	violations := svc.Check(entities, nil)

	if len(violations) != 1 {
		t.Fatalf("got %d violations, want 1", len(violations))
	}
	if violations[0].Severity != "warning" {
		t.Errorf("severity = %s, want warning (Lua should override rule default)", violations[0].Severity)
	}
}

func TestLuaValidation_SeverityDefault(t *testing.T) {
	ws := newMockWorkspace()
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"ticket": {Properties: map[string]metamodel.PropertyDef{}},
		},
		Validations: []metamodel.ValidationRule{
			{
				Name:       "test-rule",
				EntityType: "ticket",
				Lua:        `return { message = "Uses default severity" }`, // no severity specified
				Severity:   "warning",
			},
		},
	}

	entities := []*model.Entity{
		{ID: "TKT-001", Type: "ticket", Properties: map[string]interface{}{}},
	}

	svc := New(meta, WithWorkspace(ws), WithProjectRoot(t.TempDir()))
	violations := svc.Check(entities, nil)

	if len(violations) != 1 {
		t.Fatalf("got %d violations, want 1", len(violations))
	}
	if violations[0].Severity != "warning" {
		t.Errorf("severity = %s, want warning (should use rule default)", violations[0].Severity)
	}
}

func TestLuaValidation_ReturnValues(t *testing.T) {
	ws := newMockWorkspace()

	tests := []struct {
		name     string
		lua      string
		wantPass bool
	}{
		{
			name:     "return nil passes",
			lua:      `return nil`,
			wantPass: true,
		},
		{
			name:     "no return passes",
			lua:      `local x = 1`,
			wantPass: true,
		},
		{
			name:     "return table with message violates",
			lua:      `return { message = "error" }`,
			wantPass: false,
		},
		{
			name:     "return array of tables violates",
			lua:      `return { { message = "error 1" }, { message = "error 2" } }`,
			wantPass: false,
		},
		{
			name:     "return empty table passes",
			lua:      `return {}`,
			wantPass: true, // no message field, so treated as empty array
		},
		{
			name:     "return non-table is error (fail open)",
			lua:      `return "string"`,
			wantPass: true, // fail open
		},
		{
			name:     "return number is error (fail open)",
			lua:      `return 42`,
			wantPass: true, // fail open
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			meta := &metamodel.Metamodel{
				Entities: map[string]metamodel.EntityDef{
					"ticket": {Properties: map[string]metamodel.PropertyDef{}},
				},
				Validations: []metamodel.ValidationRule{
					{
						Name:       "test-rule",
						EntityType: "ticket",
						Lua:        tt.lua,
					},
				},
			}

			entities := []*model.Entity{
				{ID: "TKT-001", Type: "ticket", Properties: map[string]interface{}{}},
			}

			svc := New(meta, WithWorkspace(ws), WithProjectRoot(t.TempDir()))
			violations := svc.Check(entities, nil)

			gotPass := len(violations) == 0
			if gotPass != tt.wantPass {
				t.Errorf("got pass=%v, want pass=%v", gotPass, tt.wantPass)
			}
		})
	}
}

func TestLuaValidation_EntityContext(t *testing.T) {
	ws := newMockWorkspace()
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"ticket": {
				Properties: map[string]metamodel.PropertyDef{
					"title":  {Type: "string"},
					"status": {Type: "string"},
				},
			},
		},
		Validations: []metamodel.ValidationRule{
			{
				Name:       "check-entity-context",
				EntityType: "ticket",
				Lua: `
					-- Check entity.id and entity.type are available
					if entity.id ~= "TKT-001" then
						return { message = "entity.id mismatch" }
					end
					if entity.type ~= "ticket" then
						return { message = "entity.type mismatch" }
					end
					-- Check prop() method works
					if entity:prop("title") ~= "Test Ticket" then
						return { message = "title mismatch" }
					end
					if entity:prop("status") ~= "open" then
						return { message = "status mismatch" }
					end
					-- Check prop() with default
					if entity:prop("missing", "default") ~= "default" then
						return { message = "default value mismatch" }
					end
					return nil
				`,
			},
		},
	}

	entities := []*model.Entity{
		{
			ID:   "TKT-001",
			Type: "ticket",
			Properties: map[string]interface{}{
				"title":  "Test Ticket",
				"status": "open",
			},
		},
	}

	svc := New(meta, WithWorkspace(ws), WithProjectRoot(t.TempDir()))
	violations := svc.Check(entities, nil)

	if len(violations) != 0 {
		t.Errorf("got %d violations, want 0 (entity context should work): %v", len(violations), violations)
	}
}

func TestLuaValidation_ReadOnlyWorkspace(t *testing.T) {
	ws := newMockWorkspace()
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"ticket": {Properties: map[string]metamodel.PropertyDef{}},
		},
		Validations: []metamodel.ValidationRule{
			{
				Name:       "cross-entity-lookup",
				EntityType: "ticket",
				Lua: `
					-- Can look up other entities
					local other = rela.get_entity("PARENT-001")
					if not other then
						return { message = "get_entity failed" }
					end
					if other:prop("status") ~= "approved" then
						return { message = "status mismatch" }
					end
					return nil
				`,
			},
		},
	}

	entities := []*model.Entity{
		{ID: "TKT-001", Type: "ticket", Properties: map[string]interface{}{}},
	}

	svc := New(meta, WithWorkspace(ws), WithProjectRoot(t.TempDir()))
	violations := svc.Check(entities, nil)

	if len(violations) != 0 {
		t.Errorf("got %d violations, want 0 (cross-entity lookup should work)", len(violations))
	}
}

func TestLuaValidation_MutationsBlocked(t *testing.T) {
	ws := newMockWorkspace()
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"ticket": {Properties: map[string]metamodel.PropertyDef{}},
		},
		Validations: []metamodel.ValidationRule{
			{
				Name:       "try-create",
				EntityType: "ticket",
				Lua: `
					local ok, err = pcall(function()
						rela.create_entity("ticket", {title = "New"})
					end)
					-- Should fail, if it succeeded that's a problem
					if ok then
						return { message = "mutation should have been blocked" }
					end
					return nil
				`,
			},
		},
	}

	entities := []*model.Entity{
		{ID: "TKT-001", Type: "ticket", Properties: map[string]interface{}{}},
	}

	svc := New(meta, WithWorkspace(ws), WithProjectRoot(t.TempDir()))
	violations := svc.Check(entities, nil)

	if len(violations) != 0 {
		t.Errorf("got %d violations, want 0 (mutations should be blocked)", len(violations))
	}
}

func TestLuaValidation_SyncBlocked(t *testing.T) {
	ws := newMockWorkspace()
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"ticket": {Properties: map[string]metamodel.PropertyDef{}},
		},
		Validations: []metamodel.ValidationRule{
			{
				Name:       "try-refresh",
				EntityType: "ticket",
				Lua: `
					local ok, err = pcall(function()
						rela.refresh()
					end)
					-- Should fail
					if ok then
						return { message = "refresh should have been blocked" }
					end
					return nil
				`,
			},
		},
	}

	entities := []*model.Entity{
		{ID: "TKT-001", Type: "ticket", Properties: map[string]interface{}{}},
	}

	svc := New(meta, WithWorkspace(ws), WithProjectRoot(t.TempDir()))
	violations := svc.Check(entities, nil)

	if len(violations) != 0 {
		t.Errorf("got %d violations, want 0 (refresh should be blocked)", len(violations))
	}
}

func TestLuaValidation_CombinedWithWhenThen(t *testing.T) {
	ws := newMockWorkspace()
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"ticket": {
				Properties: map[string]metamodel.PropertyDef{
					"status":   {Type: "string"},
					"priority": {Type: "string"},
				},
			},
		},
		Validations: []metamodel.ValidationRule{
			{
				Name:       "ready-needs-priority-and-lua-check",
				EntityType: "ticket",
				When:       []string{"status=ready"},
				Then:       []string{"priority!="},
				Lua: `
					if entity:prop("priority") == "invalid" then
						return { message = "Priority cannot be 'invalid'" }
					end
					return nil
				`,
			},
		},
	}

	entities := []*model.Entity{
		// Passes both when/then and Lua
		{ID: "TKT-001", Type: "ticket", Properties: map[string]interface{}{"status": "ready", "priority": "high"}},
		// Fails when/then (no priority)
		{ID: "TKT-002", Type: "ticket", Properties: map[string]interface{}{"status": "ready"}},
		// Passes when/then but fails Lua (priority is "invalid")
		{ID: "TKT-003", Type: "ticket", Properties: map[string]interface{}{"status": "ready", "priority": "invalid"}},
		// Doesn't match when (status!=ready), so no check
		{ID: "TKT-004", Type: "ticket", Properties: map[string]interface{}{"status": "draft"}},
	}

	svc := New(meta, WithWorkspace(ws), WithProjectRoot(t.TempDir()))
	violations := svc.Check(entities, nil)

	// Should have 2 violations: TKT-002 (then fails) and TKT-003 (lua fails)
	if len(violations) != 2 {
		t.Fatalf("got %d violations, want 2", len(violations))
	}

	ids := make(map[string]bool)
	for _, v := range violations {
		ids[v.EntityID] = true
	}
	if !ids["TKT-002"] {
		t.Error("expected violation for TKT-002 (then fails)")
	}
	if !ids["TKT-003"] {
		t.Error("expected violation for TKT-003 (lua fails)")
	}
}

func TestLuaValidation_SyntaxError(t *testing.T) {
	ws := newMockWorkspace()
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"ticket": {Properties: map[string]metamodel.PropertyDef{}},
		},
		Validations: []metamodel.ValidationRule{
			{
				Name:       "syntax-error",
				EntityType: "ticket",
				Lua:        `this is not valid lua!!!`,
			},
		},
	}

	entities := []*model.Entity{
		{ID: "TKT-001", Type: "ticket", Properties: map[string]interface{}{}},
	}

	svc := New(meta, WithWorkspace(ws), WithProjectRoot(t.TempDir()))
	violations := svc.Check(entities, nil)

	// Syntax error should fail open (no violation)
	if len(violations) != 0 {
		t.Errorf("got %d violations, want 0 (syntax error should skip rule)", len(violations))
	}
}

func TestLuaValidation_RuntimeError(t *testing.T) {
	ws := newMockWorkspace()
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"ticket": {Properties: map[string]metamodel.PropertyDef{}},
		},
		Validations: []metamodel.ValidationRule{
			{
				Name:       "runtime-error",
				EntityType: "ticket",
				Lua:        `return nil_variable.property`,
			},
		},
	}

	entities := []*model.Entity{
		{ID: "TKT-001", Type: "ticket", Properties: map[string]interface{}{}},
	}

	svc := New(meta, WithWorkspace(ws), WithProjectRoot(t.TempDir()))
	violations := svc.Check(entities, nil)

	// Runtime error should fail open (no violation)
	if len(violations) != 0 {
		t.Errorf("got %d violations, want 0 (runtime error should skip rule)", len(violations))
	}
}

func TestLuaValidation_ScriptFile(t *testing.T) {
	ws := newMockWorkspace()

	// Create temp directory with validations/ subdirectory
	tmpDir := t.TempDir()
	validationsDir := filepath.Join(tmpDir, "validations")
	if err := os.MkdirAll(validationsDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Write a test script that returns violations
	scriptContent := `
		if entity:prop("status") ~= "valid" then
			return { message = "Status must be 'valid'" }
		end
		return nil
	`
	if err := os.WriteFile(filepath.Join(validationsDir, "validate-status.lua"), []byte(scriptContent), 0644); err != nil {
		t.Fatal(err)
	}

	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"ticket": {
				Properties: map[string]metamodel.PropertyDef{
					"status": {Type: "string"},
				},
			},
		},
		Validations: []metamodel.ValidationRule{
			{
				Name:       "status-valid",
				EntityType: "ticket",
				LuaFile:    "validate-status.lua",
			},
		},
	}

	entities := []*model.Entity{
		{ID: "TKT-001", Type: "ticket", Properties: map[string]interface{}{"status": "valid"}},
		{ID: "TKT-002", Type: "ticket", Properties: map[string]interface{}{"status": "invalid"}},
	}

	svc := New(meta, WithWorkspace(ws), WithProjectRoot(tmpDir))
	violations := svc.Check(entities, nil)

	if len(violations) != 1 {
		t.Fatalf("got %d violations, want 1", len(violations))
	}
	if violations[0].EntityID != "TKT-002" {
		t.Errorf("violation entity = %s, want TKT-002", violations[0].EntityID)
	}
	if violations[0].Description != "Status must be 'valid'" {
		t.Errorf("violation description = %q, want %q", violations[0].Description, "Status must be 'valid'")
	}
}

func TestLuaValidation_ScriptFileNotFound(t *testing.T) {
	ws := newMockWorkspace()
	tmpDir := t.TempDir()

	// Create validations directory but no script file
	validationsDir := filepath.Join(tmpDir, "validations")
	if err := os.MkdirAll(validationsDir, 0755); err != nil {
		t.Fatal(err)
	}

	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"ticket": {Properties: map[string]metamodel.PropertyDef{}},
		},
		Validations: []metamodel.ValidationRule{
			{
				Name:       "missing-script",
				EntityType: "ticket",
				LuaFile:    "nonexistent.lua",
			},
		},
	}

	entities := []*model.Entity{
		{ID: "TKT-001", Type: "ticket", Properties: map[string]interface{}{}},
	}

	svc := New(meta, WithWorkspace(ws), WithProjectRoot(tmpDir))
	violations := svc.Check(entities, nil)

	// Missing script should fail open (no violation)
	if len(violations) != 0 {
		t.Errorf("got %d violations, want 0 (missing script should skip rule)", len(violations))
	}
}

func TestLuaValidation_NoWorkspace(t *testing.T) {
	// When no workspace is configured, Lua rules are skipped
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"ticket": {Properties: map[string]metamodel.PropertyDef{}},
		},
		Validations: []metamodel.ValidationRule{
			{
				Name:       "lua-rule",
				EntityType: "ticket",
				Lua:        `return { message = "would fail" }`, // Would fail if executed
			},
		},
	}

	entities := []*model.Entity{
		{ID: "TKT-001", Type: "ticket", Properties: map[string]interface{}{}},
	}

	// No WithWorkspace option
	svc := New(meta)
	violations := svc.Check(entities, nil)

	// Should have no violations since Lua rule is skipped without workspace
	if len(violations) != 0 {
		t.Errorf("got %d violations, want 0 (Lua rules should be skipped without workspace)", len(violations))
	}
}

func TestLuaValidation_CrossEntityValidation(t *testing.T) {
	ws := newMockWorkspace()
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"ticket": {
				Properties: map[string]metamodel.PropertyDef{
					"status": {Type: "string"},
				},
			},
		},
		Validations: []metamodel.ValidationRule{
			{
				Name:        "parent-must-be-approved",
				Description: "If ticket has parent, parent must be approved",
				EntityType:  "ticket",
				Lua: `
					-- Get relations where this entity is "from" with type "child-of"
					local rels = rela.get_relations({from = entity.id, type = "child-of"})
					if #rels == 0 then
						return nil -- no parent, OK
					end

					local parent = rela.get_entity(rels[1].to)
					if not parent then
						return nil -- parent doesn't exist, OK (shouldn't happen)
					end

					if parent:prop("status") ~= "approved" then
						return { message = "Parent ticket must be approved" }
					end
					return nil
				`,
			},
		},
	}

	// TKT-001 has a parent (PARENT-001) which is approved - should pass
	// TKT-002 has no parent - should pass
	entities := []*model.Entity{
		{ID: "TKT-001", Type: "ticket", Properties: map[string]interface{}{"status": "ready"}},
		{ID: "TKT-002", Type: "ticket", Properties: map[string]interface{}{"status": "draft"}},
	}

	svc := New(meta, WithWorkspace(ws), WithProjectRoot(t.TempDir()))
	violations := svc.Check(entities, nil)

	if len(violations) != 0 {
		t.Errorf("got %d violations, want 0", len(violations))
	}
}

func TestReadOnlyWorkspace(t *testing.T) {
	ws := newMockWorkspace()
	roWs := newReadOnlyWorkspace(ws)

	t.Run("read operations work", func(t *testing.T) {
		// GetEntity
		e, ok := roWs.GetEntity("TKT-001")
		if !ok || e.ID != "TKT-001" {
			t.Error("GetEntity failed")
		}

		// EntitiesByType
		entities := roWs.EntitiesByType("ticket")
		if len(entities) == 0 {
			t.Error("EntitiesByType failed")
		}

		// AllRelations
		rels := roWs.AllRelations()
		if len(rels) == 0 {
			t.Error("AllRelations failed")
		}
	})

	t.Run("mutations return error", func(t *testing.T) {
		_, err := roWs.CreateEntityLua("ticket", "", nil, "")
		if !errors.Is(err, ErrReadOnly) {
			t.Errorf("CreateEntityLua returned %v, want ErrReadOnly", err)
		}

		err = roWs.UpdateEntityLua(nil, nil)
		if !errors.Is(err, ErrReadOnly) {
			t.Errorf("UpdateEntityLua returned %v, want ErrReadOnly", err)
		}

		err = roWs.DeleteEntityLua("", "", false)
		if !errors.Is(err, ErrReadOnly) {
			t.Errorf("DeleteEntityLua returned %v, want ErrReadOnly", err)
		}

		_, err = roWs.CreateRelationLua("", "", "")
		if !errors.Is(err, ErrReadOnly) {
			t.Errorf("CreateRelationLua returned %v, want ErrReadOnly", err)
		}

		err = roWs.DeleteRelation("", "", "")
		if !errors.Is(err, ErrReadOnly) {
			t.Errorf("DeleteRelation returned %v, want ErrReadOnly", err)
		}

		err = roWs.SyncLua()
		if !errors.Is(err, ErrReadOnly) {
			t.Errorf("SyncLua returned %v, want ErrReadOnly", err)
		}
	})
}

func TestLuaValidation_Timeout(t *testing.T) {
	ws := newMockWorkspace()
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"ticket": {Properties: map[string]metamodel.PropertyDef{}},
		},
		Validations: []metamodel.ValidationRule{
			{
				Name:       "infinite-loop",
				EntityType: "ticket",
				// This would run forever without timeout
				Lua: `while true do end`,
			},
		},
	}

	entities := []*model.Entity{
		{ID: "TKT-001", Type: "ticket", Properties: map[string]interface{}{}},
	}

	// Should complete within reasonable time (timeout kicks in)
	svc := New(meta, WithWorkspace(ws), WithProjectRoot(t.TempDir()))
	violations := svc.Check(entities, nil)

	// Timeout should fail open (no violation, rule skipped due to error)
	if len(violations) != 0 {
		t.Errorf("got %d violations, want 0 (timeout should skip rule)", len(violations))
	}
}

func TestLuaValidation_PathTraversal(t *testing.T) {
	ws := newMockWorkspace()
	tmpDir := t.TempDir()

	// Create validations directory with a valid script
	validationsDir := filepath.Join(tmpDir, "validations")
	if err := os.MkdirAll(validationsDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(validationsDir, "valid.lua"), []byte(`return nil`), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a secret file outside validations/
	secretPath := filepath.Join(tmpDir, "secret.lua")
	if err := os.WriteFile(secretPath, []byte(`return { message = "should not run" }`), 0644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name     string
		luaFile  string
		wantSkip bool // true if rule should be skipped (fail open)
	}{
		{
			name:     "valid script",
			luaFile:  "valid.lua",
			wantSkip: false,
		},
		{
			name:     "path traversal with ..",
			luaFile:  "../secret.lua",
			wantSkip: true, // Should be blocked
		},
		{
			name:     "absolute path",
			luaFile:  "/etc/passwd.lua",
			wantSkip: true, // Should be blocked
		},
		{
			name:     "non-.lua extension",
			luaFile:  "malicious.txt",
			wantSkip: true, // Should be blocked
		},
		{
			name:     "embedded traversal",
			luaFile:  "subdir/../../../secret.lua",
			wantSkip: true, // Should be blocked
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			meta := &metamodel.Metamodel{
				Entities: map[string]metamodel.EntityDef{
					"ticket": {Properties: map[string]metamodel.PropertyDef{}},
				},
				Validations: []metamodel.ValidationRule{
					{
						Name:       "test-rule",
						EntityType: "ticket",
						LuaFile:    tt.luaFile,
					},
				},
			}

			entities := []*model.Entity{
				{ID: "TKT-001", Type: "ticket", Properties: map[string]interface{}{}},
			}

			svc := New(meta, WithWorkspace(ws), WithProjectRoot(tmpDir))
			violations := svc.Check(entities, nil)

			// If rule should be skipped, expect 0 violations (fail open)
			// If valid script, expect 0 violations (script returns nil)
			if len(violations) != 0 {
				t.Errorf("got %d violations, want 0", len(violations))
			}
		})
	}
}
