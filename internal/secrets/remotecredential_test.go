package secrets

import (
	"errors"
	"testing"
)

func TestScriptReadable(t *testing.T) {
	for name, want := range map[string]bool{
		"api_key":                    true,
		TokenKey:                     false,
		"remote_credential.upstream": false,
		"remote_credential.":         false,
		"my_remote_credential.x":     true,
		"remote_credential":          true,
	} {
		if got := ScriptReadable(name); got != want {
			t.Errorf("ScriptReadable(%q) = %v, want %v", name, got, want)
		}
	}
}

// TestIsRemoteCredential pins that every name the reconcile client accepts as
// a remote credential is one no script can read.
func TestIsRemoteCredential(t *testing.T) {
	for name, want := range map[string]bool{
		"remote_credential.up": true, "remote_credential.": false, "remote_credential": false,
		"api_key": false, TokenKey: false, "x.remote_credential.up": false,
	} {
		if got := IsRemoteCredential(name); got != want {
			t.Errorf("IsRemoteCredential(%q) = %v, want %v", name, got, want)
		}
		if IsRemoteCredential(name) && ScriptReadable(name) {
			t.Errorf("%q is a remote credential that a script may read", name)
		}
	}
}

func TestGlobal_IgnoresOverrides(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, `
remote_credential.upstream: tok-x
overrides:
  "":
    remote_credential.upstream: wrong
    other: y
  sync.lua:
    other: z
`)
	for name, want := range map[string]struct {
		v  string
		ok bool
	}{"remote_credential.upstream": {"tok-x", true}, "other": {"", false}} {
		v, ok, err := Global(dir, name)
		if err != nil || v != want.v || ok != want.ok {
			t.Errorf("Global(%q) = %q, %v, %v; want %q, %v", name, v, ok, err, want.v, want.ok)
		}
	}
	// Load with no script reads the same global section: the pull reads its
	// credential through it, and reconcile diff through Global.
	all, err := Load(dir, "")
	if err != nil || all["remote_credential.upstream"] != "tok-x" || all["other"] != "" {
		t.Errorf("Load(\"\") = %v, %v; want only the global section", all, err)
	}
	if _, _, err := Global(t.TempDir(), "x"); !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}
