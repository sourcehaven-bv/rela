package script

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/lua"
)

func TestParseActionResponse_Nil(t *testing.T) {
	resp, err := parseActionResponse(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp == nil {
		t.Fatal("expected empty response, got nil")
	}
	if resp.Redirect != "" || resp.Message != "" {
		t.Errorf("expected empty fields, got %+v", resp)
	}
}

func TestParseActionResponse_Valid(t *testing.T) {
	tests := []struct {
		name string
		in   map[string]interface{}
		want ActionResponse
	}{
		{
			name: "redirect only",
			in:   map[string]interface{}{"redirect": "/foo"},
			want: ActionResponse{Redirect: "/foo"},
		},
		{
			name: "message only",
			in:   map[string]interface{}{"message": "hi"},
			want: ActionResponse{Message: "hi"},
		},
		{
			name: "all fields",
			in: map[string]interface{}{
				"redirect":     "/x",
				"message":      "done",
				"message_type": "success",
			},
			want: ActionResponse{Redirect: "/x", Message: "done", MessageType: "success"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseActionResponse(tt.in)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if *got != tt.want {
				t.Errorf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestParseActionResponse_InvalidRedirect(t *testing.T) {
	tests := []struct {
		name     string
		redirect string
	}{
		{"no leading slash", "foo"},
		{"protocol relative", "//evil.com"},
		{"absolute url", "https://evil.com"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseActionResponse(map[string]interface{}{"redirect": tt.redirect})
			if err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestParseActionResponse_InvalidMessageType(t *testing.T) {
	_, err := parseActionResponse(map[string]interface{}{"message_type": "bogus"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestParseActionResponse_NotATable(t *testing.T) {
	_, err := parseActionResponse("oops")
	if err == nil {
		t.Fatal("expected error for non-table return")
	}
}

func TestValidateRedirect(t *testing.T) {
	tests := []struct {
		in      string
		wantErr bool
	}{
		{"", false},
		{"/foo", false},
		{"/foo/bar", false},
		{"foo", true},
		{"//evil.com", true},
		{"https://evil.com", true},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			err := validateRedirect(tt.in)
			if tt.wantErr && err == nil {
				t.Errorf("expected error for %q", tt.in)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error for %q: %v", tt.in, err)
			}
		})
	}
}

func TestExecuteAction_PathTraversal(t *testing.T) {
	engine := NewEngine()
	deps := testWriteDeps("/project")

	_, err := engine.ExecuteAction(context.Background(), "../etc/passwd", deps, nil, nil, time.Second, "")
	if err == nil {
		t.Fatal("expected error for path traversal")
	}
}

func TestExecuteAction_WrongExtension(t *testing.T) {
	engine := NewEngine()
	deps := testWriteDeps("/project")

	_, err := engine.ExecuteAction(context.Background(), "script.txt", deps, nil, nil, time.Second, "")
	if err == nil {
		t.Fatal("expected error for wrong extension")
	}
}

func TestExecuteAction_RealFile(t *testing.T) {
	// Set up a real project directory with an action script
	tmpDir := t.TempDir()
	actionsDir := filepath.Join(tmpDir, "actions")
	if err := os.MkdirAll(actionsDir, 0755); err != nil {
		t.Fatal(err)
	}

	scriptContent := `
		return {
			redirect = "/test",
			message = "executed",
			message_type = "success",
		}
	`
	scriptPath := filepath.Join(actionsDir, "test.lua")
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0644); err != nil {
		t.Fatal(err)
	}

	engine := NewEngine()
	deps := testWriteDeps(tmpDir)

	resp, err := engine.ExecuteAction(context.Background(), "test.lua", deps, nil, nil, 5*time.Second, "")
	if err != nil {
		t.Fatalf("ExecuteAction failed: %v", err)
	}
	if resp.Redirect != "/test" || resp.Message != "executed" || resp.MessageType != "success" {
		t.Errorf("unexpected response: %+v", resp)
	}
}

func TestExecuteAction_WithTriggerEntity(t *testing.T) {
	tmpDir := t.TempDir()
	actionsDir := filepath.Join(tmpDir, "actions")
	if err := os.MkdirAll(actionsDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Script reads the `entity` global injected by ExecuteAction when
	// triggerEntity is non-nil.
	scriptContent := `return {message = entity.id}`
	scriptPath := filepath.Join(actionsDir, "ent.lua")
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0644); err != nil {
		t.Fatal(err)
	}

	engine := NewEngine()
	deps := testWriteDeps(tmpDir)
	ent := &entity.Entity{ID: "T-42", Type: "ticket"}

	resp, err := engine.ExecuteAction(context.Background(), "ent.lua", deps, ent, nil, 5*time.Second, "")
	if err != nil {
		t.Fatalf("ExecuteAction failed: %v", err)
	}
	if resp.Message != ent.ID {
		t.Errorf("expected message=%s from triggerEntity, got %q", ent.ID, resp.Message)
	}
}

func TestExecuteAction_WithParams(t *testing.T) {
	tmpDir := t.TempDir()
	actionsDir := filepath.Join(tmpDir, "actions")
	if err := os.MkdirAll(actionsDir, 0755); err != nil {
		t.Fatal(err)
	}

	scriptContent := `return {message = rela.params.greeting}`
	scriptPath := filepath.Join(actionsDir, "params.lua")
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0644); err != nil {
		t.Fatal(err)
	}

	engine := NewEngine()
	deps := testWriteDeps(tmpDir)
	params := map[string]string{"greeting": "hello"}

	resp, err := engine.ExecuteAction(context.Background(), "params.lua", deps, nil, params, 5*time.Second, "")
	if err != nil {
		t.Fatalf("ExecuteAction failed: %v", err)
	}
	if resp.Message != "hello" {
		t.Errorf("expected message=hello, got %q", resp.Message)
	}
}

func TestExecuteAction_ScriptError(t *testing.T) {
	tmpDir := t.TempDir()
	actionsDir := filepath.Join(tmpDir, "actions")
	if err := os.MkdirAll(actionsDir, 0755); err != nil {
		t.Fatal(err)
	}

	scriptContent := `print("before")
error("kaboom")`
	scriptPath := filepath.Join(actionsDir, "boom.lua")
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0644); err != nil {
		t.Fatal(err)
	}

	engine := NewEngine()
	deps := testWriteDeps(tmpDir)
	ent := &entity.Entity{ID: "T-99", Type: "ticket"}

	_, err := engine.ExecuteAction(context.Background(), "boom.lua", deps, ent,
		map[string]string{"greeting": "hi", "password": "leak"}, 5*time.Second, "corr-test")
	if err == nil {
		t.Fatal("expected error from script")
	}

	var se *lua.ScriptError
	if !errors.As(err, &se) {
		t.Fatalf("expected *lua.ScriptError, got %T: %v", err, err)
	}

	if se.Surface != lua.SurfaceAction {
		t.Errorf("Surface=%q, want %q", se.Surface, lua.SurfaceAction)
	}
	if se.Path != "actions/boom.lua" {
		t.Errorf("Path=%q, want actions/boom.lua", se.Path)
	}
	if se.EntityID != ent.ID {
		t.Errorf("EntityID=%q, want %q", se.EntityID, ent.ID)
	}
	if !strings.Contains(se.LuaMessage, "kaboom") {
		t.Errorf("LuaMessage=%q, want contains kaboom", se.LuaMessage)
	}
	if se.LuaLine != 2 {
		t.Errorf("LuaLine=%d, want 2", se.LuaLine)
	}
	if se.Args["greeting"] != "hi" {
		t.Errorf("Args[greeting]=%v, want hi", se.Args["greeting"])
	}
	if se.Args["password"] != "<redacted>" {
		t.Errorf("Args[password]=%v, want redacted", se.Args["password"])
	}
	if !strings.Contains(se.CapturedOutput, "before") {
		t.Errorf("CapturedOutput=%q, want contains 'before'", se.CapturedOutput)
	}
	if len(se.Source) == 0 {
		t.Error("Source is empty; expected slice around line 2")
	}
	if se.CorrelationID != "corr-test" {
		t.Errorf("CorrelationID=%q, want corr-test", se.CorrelationID)
	}
}

func TestCheckActionScriptExists_Missing(t *testing.T) {
	tmpDir := t.TempDir()
	actionsDir := filepath.Join(tmpDir, "actions")
	if err := os.MkdirAll(actionsDir, 0755); err != nil {
		t.Fatal(err)
	}

	err := CheckActionScriptExists(tmpDir, "missing.lua")
	if err == nil {
		t.Fatal("expected error for missing script")
	}
}

func TestCheckActionScriptExists_Present(t *testing.T) {
	tmpDir := t.TempDir()
	actionsDir := filepath.Join(tmpDir, "actions")
	if err := os.MkdirAll(actionsDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(actionsDir, "ok.lua"), []byte("return {}"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := CheckActionScriptExists(tmpDir, "ok.lua"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}
