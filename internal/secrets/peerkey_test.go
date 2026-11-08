package secrets

import (
	"errors"
	"maps"
	"testing"
)

func TestPeerKeyDigests_GlobalOnly(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, `
api_key: sk-abc123
peer_key.acme: sha256:aa
peer_key.beta: sha256:bb
overrides:
  sync.lua:
    peer_key.evil: sha256:cc
`)
	got, err := PeerKeyDigests(dir)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"acme": "sha256:aa", "beta": "sha256:bb"}
	if !maps.Equal(got, want) {
		t.Errorf("PeerKeyDigests = %v, want %v", got, want)
	}
}

func TestPeerKeyDigests_NotFound(t *testing.T) {
	if _, err := PeerKeyDigests(t.TempDir()); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestScriptReadable(t *testing.T) {
	for name, want := range map[string]bool{
		"api_key": true, TokenKey: false, "peer_key.acme": false, "peer_key.": false, "my_peer_key.x": true,
		"peer_credential.source": false,
	} {
		if got := ScriptReadable(name); got != want {
			t.Errorf("ScriptReadable(%q) = %v, want %v", name, got, want)
		}
	}
}

func TestGlobal_IgnoresOverrides(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, `
upstream: relapeer_x
overrides:
  "":
    upstream: wrong
    other: y
`)
	for name, want := range map[string]struct {
		v  string
		ok bool
	}{"upstream": {"relapeer_x", true}, "other": {"", false}} {
		v, ok, err := Global(dir, name)
		if err != nil || v != want.v || ok != want.ok {
			t.Errorf("Global(%q) = %q, %v, %v; want %q, %v", name, v, ok, err, want.v, want.ok)
		}
	}
	if _, _, err := Global(t.TempDir(), "x"); !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}
