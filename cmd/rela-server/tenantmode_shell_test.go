package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/dataentry"
	"github.com/Sourcehaven-BV/rela/internal/script"
)

// TestShellBoot reads the shell's app name and language from the shared
// data-entry.yaml, and leaves the shell as built without one (BUG-21N3J4N).
func TestShellBoot(t *testing.T) {
	cases := []struct {
		name      string
		dataEntry string
		want      dataentry.ShellBoot
	}{
		{"named", "app:\n  name: Sourcehaven\n  language: en\n", dataentry.ShellBoot{AppName: "Sourcehaven", Language: "en"}},
		{"no file", "", dataentry.ShellBoot{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			schema := "entities:\n  note:\n    id_type: sequential\n    id_prefix: N\n    label: Note\n    properties:\n      title: {type: string}\n"
			if err := os.WriteFile(filepath.Join(dir, "schema.yaml"), []byte(schema), 0o600); err != nil {
				t.Fatal(err)
			}
			if tc.dataEntry != "" {
				if err := os.WriteFile(filepath.Join(dir, "data-entry.yaml"), []byte(tc.dataEntry), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			base, err := appbuild.DiscoverSharedBase(dir, script.NewEngine())
			if err != nil {
				t.Fatal(err)
			}
			if got := shellBoot(base); got != tc.want {
				t.Errorf("shellBoot = %+v, want %+v", got, tc.want)
			}
		})
	}
}
