package script

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/entity"
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

	_, err := engine.ExecuteAction("../etc/passwd", deps, nil, nil, time.Second)
	if err == nil {
		t.Fatal("expected error for path traversal")
	}
}

func TestExecuteAction_WrongExtension(t *testing.T) {
	engine := NewEngine()
	deps := testWriteDeps("/project")

	_, err := engine.ExecuteAction("script.txt", deps, nil, nil, time.Second)
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

	resp, err := engine.ExecuteAction("test.lua", deps, nil, nil, 5*time.Second)
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

	resp, err := engine.ExecuteAction("ent.lua", deps, ent, nil, 5*time.Second)
	if err != nil {
		t.Fatalf("ExecuteAction failed: %v", err)
	}
	if resp.Message != "T-42" {
		t.Errorf("expected message=T-42 from triggerEntity, got %q", resp.Message)
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

	resp, err := engine.ExecuteAction("params.lua", deps, nil, params, 5*time.Second)
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

	scriptContent := `error("kaboom")`
	scriptPath := filepath.Join(actionsDir, "boom.lua")
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0644); err != nil {
		t.Fatal(err)
	}

	engine := NewEngine()
	deps := testWriteDeps(tmpDir)

	_, err := engine.ExecuteAction("boom.lua", deps, nil, nil, 5*time.Second)
	if err == nil {
		t.Fatal("expected error from script")
	}
	if !strings.Contains(err.Error(), "kaboom") {
		t.Errorf("expected error to mention kaboom, got %v", err)
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
