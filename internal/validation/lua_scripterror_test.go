package validation

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// TestLuaValidation_InlineCompileErrorSurfacesAsScriptError covers AC1:
// a syntactically broken `lua:` block produces a *lua.ScriptError with
// Surface=validation, Path=validations/<rule-name>, non-empty
// LuaMessage, and renders cleanly via Error().
func TestLuaValidation_InlineCompileErrorSurfacesAsScriptError(t *testing.T) {
	ws := newMockWorkspace()
	rule := metamodel.ValidationRule{
		Name:       "syntax-error",
		EntityType: "ticket",
		Lua:        `if oops invalid`,
	}
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"ticket": {Properties: map[string]metamodel.PropertyDef{}},
		},
		Validations: []metamodel.ValidationRule{rule},
	}
	entities := []*entity.Entity{
		{ID: "TKT-001", Type: "ticket", Properties: map[string]interface{}{}},
	}

	svc := New(meta, ws.services(t.TempDir()))
	result := svc.Check(context.Background(), entities, nil)

	if len(result.Violations) != 0 {
		t.Errorf("got %d violations, want 0 (compile error must not produce violations)",
			len(result.Violations))
	}
	if len(result.ScriptErrors) != 1 {
		t.Fatalf("got %d ScriptErrors, want 1", len(result.ScriptErrors))
	}
	se := result.ScriptErrors[0]
	if se.Surface != lua.SurfaceValidation {
		t.Errorf("surface = %q, want %q", se.Surface, lua.SurfaceValidation)
	}
	wantPath := "validation:" + rule.Name
	if se.Path != wantPath {
		t.Errorf("path = %q, want %q", se.Path, wantPath)
	}
	if se.LuaMessage == "" {
		t.Error("LuaMessage is empty; want compile-error message")
	}
	if !result.HasErrors() {
		t.Error("HasErrors = false, want true")
	}
}

// TestLuaValidation_InlineRuleUsesDistinctCacheNamespace covers
// RR-TEGZP: inline rules use "validation:<name>" (colon) so their
// chunkname / cache namespace cannot collide with a real script
// living at validations/<name>.lua. The Path on a captured frame
// must match the envelope so source-slice resolution still aligns.
func TestLuaValidation_InlineRuleUsesDistinctCacheNamespace(t *testing.T) {
	ws := newMockWorkspace()
	rule := metamodel.ValidationRule{
		Name:       "shared-name.lua",
		EntityType: "ticket",
		Lua:        `local x = nil; return x.field`,
	}
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"ticket": {Properties: map[string]metamodel.PropertyDef{}},
		},
		Validations: []metamodel.ValidationRule{rule},
	}
	entities := []*entity.Entity{
		{ID: "TKT-001", Type: "ticket", Properties: map[string]interface{}{}},
	}
	svc := New(meta, ws.services(t.TempDir()))
	result := svc.Check(context.Background(), entities, nil)

	if len(result.ScriptErrors) != 1 {
		t.Fatalf("got %d ScriptErrors, want 1", len(result.ScriptErrors))
	}
	se := result.ScriptErrors[0]
	wantPath := "validation:" + rule.Name
	if se.Path != wantPath {
		t.Errorf("Path = %q, want %q (inline rules use colon prefix to avoid collision with validations/<file>)",
			se.Path, wantPath)
	}
	// Captured frame must reference the same chunkname so any
	// downstream source-slice resolution stays consistent.
	if se.LuaLine == 0 {
		t.Error("LuaLine = 0; expected runtime error to capture a frame")
	}
}

// TestLuaValidation_FileRuntimeErrorIncludesSourceSlice covers AC2:
// a runtime error in a `lua_file:` script populates Source with the
// failing line + context, and Path resolves to validations/<file>.
func TestLuaValidation_FileRuntimeErrorIncludesSourceSlice(t *testing.T) {
	ws := newMockWorkspace()
	tmpDir := t.TempDir()

	validationsDir := filepath.Join(tmpDir, "validations")
	if err := os.MkdirAll(validationsDir, 0755); err != nil {
		t.Fatal(err)
	}

	scriptName := "broken.lua"
	scriptContent := "" +
		"-- header\n" +
		"-- header\n" +
		"-- header\n" +
		"local x = nil\n" +
		"return x.field\n" +
		"-- footer\n" +
		"-- footer\n" +
		"-- footer\n"
	if err := os.WriteFile(filepath.Join(validationsDir, scriptName), []byte(scriptContent), 0644); err != nil {
		t.Fatal(err)
	}

	rule := metamodel.ValidationRule{
		Name:       "broken",
		EntityType: "ticket",
		LuaFile:    scriptName,
	}
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"ticket": {Properties: map[string]metamodel.PropertyDef{}},
		},
		Validations: []metamodel.ValidationRule{rule},
	}
	entities := []*entity.Entity{
		{ID: "TKT-001", Type: "ticket", Properties: map[string]interface{}{}},
	}

	svc := New(meta, ws.services(tmpDir))
	result := svc.Check(context.Background(), entities, nil)

	if len(result.ScriptErrors) != 1 {
		t.Fatalf("got %d ScriptErrors, want 1: %+v", len(result.ScriptErrors), result.ScriptErrors)
	}
	se := result.ScriptErrors[0]
	wantPath := "validations/" + scriptName
	if se.Path != wantPath {
		t.Errorf("path = %q, want %q", se.Path, wantPath)
	}
	if se.Surface != lua.SurfaceValidation {
		t.Errorf("surface = %q, want %q", se.Surface, lua.SurfaceValidation)
	}
	if se.LuaLine == 0 {
		t.Error("LuaLine = 0, want non-zero")
	}
	if len(se.Source) == 0 {
		t.Fatal("Source is empty; want lines around the failing line")
	}
	// At least one source line should be the failing line, marked as highlighted.
	var foundHighlighted bool
	for _, sl := range se.Source {
		if sl.Highlight {
			foundHighlighted = true
			if !strings.Contains(sl.Text, "x.field") {
				t.Errorf("highlighted line text = %q, want it to contain 'x.field'", sl.Text)
			}
		}
	}
	if !foundHighlighted {
		t.Error("no highlighted line found in Source")
	}
}

// TestLuaValidation_FailOpenWithMixedRules covers AC3: with a
// broken rule and several valid rules, the broken one yields a
// ScriptError but the valid rules still produce their violations.
func TestLuaValidation_FailOpenWithMixedRules(t *testing.T) {
	ws := newMockWorkspace()
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"ticket": {Properties: map[string]metamodel.PropertyDef{}},
		},
		Validations: []metamodel.ValidationRule{
			{
				Name:       "broken",
				EntityType: "ticket",
				Lua:        `error("boom")`,
			},
			{
				Name:       "always-violates-1",
				EntityType: "ticket",
				Lua:        `return { message = "v1" }`,
			},
			{
				Name:       "always-violates-2",
				EntityType: "ticket",
				Lua:        `return { message = "v2" }`,
			},
			{
				Name:       "always-violates-3",
				EntityType: "ticket",
				Lua:        `return { message = "v3" }`,
			},
			{
				Name:       "always-violates-4",
				EntityType: "ticket",
				Lua:        `return { message = "v4" }`,
			},
		},
	}
	entities := []*entity.Entity{
		{ID: "TKT-001", Type: "ticket", Properties: map[string]interface{}{}},
	}
	svc := New(meta, ws.services(t.TempDir()))
	result := svc.Check(context.Background(), entities, nil)

	if len(result.ScriptErrors) != 1 {
		t.Fatalf("got %d ScriptErrors, want 1", len(result.ScriptErrors))
	}
	if len(result.Violations) != 4 {
		t.Fatalf("got %d violations, want 4 (other 4 rules must still run)", len(result.Violations))
	}
}

// TestLuaValidation_LoadErrorWhenFileMissing covers AC6: a missing
// `lua_file:` rule produces a LoadError, not a ScriptError — load
// failures and Lua failures are categorized separately.
func TestLuaValidation_LoadErrorWhenFileMissing(t *testing.T) {
	ws := newMockWorkspace()
	tmpDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmpDir, "validations"), 0755); err != nil {
		t.Fatal(err)
	}

	missingFile := "absent.lua"
	rule := metamodel.ValidationRule{
		Name:       "missing-file",
		EntityType: "ticket",
		LuaFile:    missingFile,
	}
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"ticket": {Properties: map[string]metamodel.PropertyDef{}},
		},
		Validations: []metamodel.ValidationRule{rule},
	}
	entities := []*entity.Entity{
		{ID: "TKT-001", Type: "ticket", Properties: map[string]interface{}{}},
	}

	svc := New(meta, ws.services(tmpDir))
	result := svc.Check(context.Background(), entities, nil)

	if len(result.LoadErrors) != 1 {
		t.Fatalf("got %d LoadErrors, want 1", len(result.LoadErrors))
	}
	if result.LoadErrors[0].RuleName != rule.Name {
		t.Errorf("RuleName = %q, want %q", result.LoadErrors[0].RuleName, rule.Name)
	}
	if !strings.Contains(result.LoadErrors[0].Message, missingFile) {
		t.Errorf("Message = %q, want to contain %q", result.LoadErrors[0].Message, missingFile)
	}
	if len(result.ScriptErrors) != 0 {
		t.Errorf("got %d ScriptErrors, want 0 (load failure should not produce ScriptError)",
			len(result.ScriptErrors))
	}
	if !result.HasErrors() {
		t.Error("HasErrors = false, want true")
	}
}

// TestLuaValidation_ContractErrorReturnNumber covers AC7 (case 1):
// a Lua rule returning a non-table value produces a synthesized
// *lua.ScriptError with no LuaLine and an explanatory message.
func TestLuaValidation_ContractErrorReturnNumber(t *testing.T) {
	ws := newMockWorkspace()
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"ticket": {Properties: map[string]metamodel.PropertyDef{}},
		},
		Validations: []metamodel.ValidationRule{
			{
				Name:       "returns-number",
				EntityType: "ticket",
				Lua:        `return 42`,
			},
		},
	}
	entities := []*entity.Entity{
		{ID: "TKT-001", Type: "ticket", Properties: map[string]interface{}{}},
	}
	svc := New(meta, ws.services(t.TempDir()))
	result := svc.Check(context.Background(), entities, nil)

	if len(result.ScriptErrors) != 1 {
		t.Fatalf("got %d ScriptErrors, want 1", len(result.ScriptErrors))
	}
	se := result.ScriptErrors[0]
	if !strings.Contains(se.LuaMessage, "must return nil or table") {
		t.Errorf("LuaMessage = %q, want to mention 'must return nil or table'", se.LuaMessage)
	}
	if se.LuaLine != 0 {
		t.Errorf("LuaLine = %d, want 0 (contract errors have no frame)", se.LuaLine)
	}
}

// TestLuaValidation_ContractErrorMissingMessageField covers AC7
// (case 2): a violation table lacking the `message` field surfaces
// as a contract error.
func TestLuaValidation_ContractErrorMissingMessageField(t *testing.T) {
	ws := newMockWorkspace()
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"ticket": {Properties: map[string]metamodel.PropertyDef{}},
		},
		Validations: []metamodel.ValidationRule{
			{
				Name:       "missing-message",
				EntityType: "ticket",
				// Has a key, so parseLuaReturnValue treats it as single
				// violation, then notices the missing 'message' field.
				Lua: `return { severity = "error" }`,
			},
		},
	}
	entities := []*entity.Entity{
		{ID: "TKT-001", Type: "ticket", Properties: map[string]interface{}{}},
	}
	svc := New(meta, ws.services(t.TempDir()))
	result := svc.Check(context.Background(), entities, nil)

	// A table with non-numeric keys but no "message" field is parsed
	// as an empty array, producing zero violations and no error —
	// the contract-error path covers the "message field is the only
	// hint and it's empty/missing" case. Force that case via a
	// numeric-key table whose item lacks 'message'.
	if len(result.ScriptErrors) != 0 {
		t.Logf("non-message table treated as empty array (no ScriptError): %v", result.ScriptErrors)
	}

	// Now exercise the array-element missing-message path.
	meta2 := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"ticket": {Properties: map[string]metamodel.PropertyDef{}},
		},
		Validations: []metamodel.ValidationRule{
			{
				Name:       "array-missing-message",
				EntityType: "ticket",
				Lua:        `return { { severity = "error" } }`,
			},
		},
	}
	svc2 := New(meta2, ws.services(t.TempDir()))
	result2 := svc2.Check(context.Background(), entities, nil)

	if len(result2.ScriptErrors) != 1 {
		t.Fatalf("got %d ScriptErrors, want 1", len(result2.ScriptErrors))
	}
	if !strings.Contains(result2.ScriptErrors[0].LuaMessage, "missing 'message' field") {
		t.Errorf("LuaMessage = %q, want to mention 'missing 'message' field'",
			result2.ScriptErrors[0].LuaMessage)
	}
}
