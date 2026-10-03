package lua

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestReadDeps_ReadScriptPaths(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "scripts", "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "scripts", "sub", "a.lua"), []byte("return 1"), 0o600); err != nil {
		t.Fatal(err)
	}
	d := ReadDeps{ProjectRoot: root}

	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{"slash path", "sub/a.lua", false},
		{"backslash separates on every platform", `sub\a.lua`, false},
		{"parent via backslash", `..\scripts\sub\a.lua`, true},
		{"parent via slash", "../scripts/sub/a.lua", true},
		{"absolute", "/sub/a.lua", true},
		{"not lua", "sub/a.txt", true},
		{"empty", "", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			src, err := d.ReadScript(context.Background(), "scripts", tc.path)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("ReadScript(%q) = %q, want an error", tc.path, src)
				}
				return
			}
			if err != nil || src != "return 1" {
				t.Fatalf("ReadScript(%q) = %q, %v", tc.path, src, err)
			}
		})
	}
}
