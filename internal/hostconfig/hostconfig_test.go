package hostconfig_test

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/hostconfig"
	"github.com/Sourcehaven-BV/rela/internal/secrets"
)

func TestDir(t *testing.T) {
	relaDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(relaDir, hostconfig.AIFile), []byte("model: m\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	d := hostconfig.Dir(relaDir)

	t.Run("reads a host file", func(t *testing.T) {
		data, err := d.File(hostconfig.AIFile)
		if err != nil || string(data) != "model: m\n" {
			t.Fatalf("File = %q, %v", data, err)
		}
	})
	t.Run("absent file is fs.ErrNotExist", func(t *testing.T) {
		if _, err := d.File(hostconfig.MailFile); !errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("err = %v, want fs.ErrNotExist", err)
		}
	})
	t.Run("refuses other names", func(t *testing.T) {
		for _, name := range []string{"secrets.yaml", "../schema.yaml", ""} {
			if _, err := d.File(name); !errors.Is(err, hostconfig.ErrUnknownFile) {
				t.Errorf("File(%q) err = %v, want ErrUnknownFile", name, err)
			}
		}
	})
	t.Run("empty dir has no files", func(t *testing.T) {
		if _, err := hostconfig.Dir("").File(hostconfig.AIFile); !errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("err = %v, want fs.ErrNotExist", err)
		}
	})
	t.Run("secrets come from secrets.yaml", func(t *testing.T) {
		if _, err := d.Secrets(""); !errors.Is(err, secrets.ErrNotFound) {
			t.Fatalf("no file: err = %v, want ErrNotFound", err)
		}
		if err := os.WriteFile(filepath.Join(relaDir, secrets.ConfigFile), []byte("token: abc\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		got, err := d.Secrets("")
		if err != nil || got["token"] != "abc" {
			t.Fatalf("Secrets = %v, %v", got, err)
		}
	})
	if d.Path() != relaDir {
		t.Errorf("Path = %q", d.Path())
	}
}
