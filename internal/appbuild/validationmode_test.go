package appbuild

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/project"
)

func TestValidationModeFor(t *testing.T) {
	tests := []struct {
		name    string
		config  string // "" means no .rela/config.yaml
		env     string
		want    entitymanager.ValidationMode
		wantErr string
	}{
		{name: "no config", want: entitymanager.ValidationAdvisory},
		{name: "config without key", config: "formatting:\n  line_width: 100\n",
			want: entitymanager.ValidationAdvisory},
		{name: "config strict", config: "validation: strict\n", want: entitymanager.ValidationStrict},
		{name: "env overrides config", config: "validation: strict\n", env: "advisory",
			want: entitymanager.ValidationAdvisory},
		{name: "env strict", env: "strict", want: entitymanager.ValidationStrict},
		{name: "bad config value", config: "validation: blocking\n", wantErr: "config.yaml"},
		{name: "bad env value", env: "blocking", wantErr: "$" + ValidationModeEnv},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(ValidationModeEnv, tc.env)
			root := t.TempDir()
			cache := filepath.Join(root, ".rela")
			if err := os.MkdirAll(cache, 0o755); err != nil {
				t.Fatal(err)
			}
			if tc.config != "" {
				if err := os.WriteFile(filepath.Join(cache, project.ConfigFile), []byte(tc.config), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			got, err := validationModeFor(Config{Paths: &project.Context{Root: root, CacheDir: cache}})
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("err = %v, want it to contain %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Fatalf("mode = %v, want %v", got, tc.want)
			}
		})
	}
}
