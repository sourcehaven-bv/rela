package encryption

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func writePubKey(t *testing.T, dir, name string) *Keypair {
	t.Helper()
	k := mustGenerate(t)
	pemBytes, err := MarshalPublicKeyPEM(k.PublicKey())
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, name+".pub")
	if err := os.WriteFile(path, pemBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	return k
}

func writePrivKey(t *testing.T, path string, k *Keypair) {
	t.Helper()
	pemBytes, err := MarshalPrivateKeyPEM(k)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, pemBytes, 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestLoadKeyring_Empty(t *testing.T) {
	dir := t.TempDir()
	kr, err := LoadKeyring(dir, "")
	if err != nil {
		t.Fatal(err)
	}
	if got := kr.Identities(); len(got) != 0 {
		t.Fatalf("Identities = %v, want empty", got)
	}
	if kr.HasPrivateKey() {
		t.Fatal("HasPrivateKey should be false")
	}
}

func TestLoadKeyring_NonexistentDir(t *testing.T) {
	kr, err := LoadKeyring(filepath.Join(t.TempDir(), "missing"), "")
	if err != nil {
		t.Fatal(err)
	}
	if len(kr.Identities()) != 0 {
		t.Fatal("missing dir should yield empty recipients")
	}
}

func TestLoadKeyring_EmptyKeysDirArgument(t *testing.T) {
	kr, err := LoadKeyring("", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(kr.Identities()) != 0 {
		t.Fatal("empty keysDir arg should skip load")
	}
}

func TestLoadKeyring_SingleRecipient(t *testing.T) {
	dir := t.TempDir()
	writePubKey(t, dir, "alice")
	kr, err := LoadKeyring(dir, "")
	if err != nil {
		t.Fatal(err)
	}
	got := kr.Identities()
	if !reflect.DeepEqual(got, []string{"alice"}) {
		t.Fatalf("Identities = %v, want [alice]", got)
	}
	if _, ok := kr.Recipient("alice"); !ok {
		t.Fatal("alice missing")
	}
	if _, ok := kr.Recipient("bob"); ok {
		t.Fatal("bob should not exist")
	}
}

func TestLoadKeyring_ManyRecipients_Sorted(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"charlie", "alice", "bob"} {
		writePubKey(t, dir, name)
	}
	kr, err := LoadKeyring(dir, "")
	if err != nil {
		t.Fatal(err)
	}
	got := kr.Identities()
	want := []string{"alice", "bob", "charlie"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Identities = %v, want %v", got, want)
	}
}

func TestLoadKeyring_IgnoresSubdirsAndNonPub(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "readme.md"), []byte("ignore"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".DS_Store"), []byte("ignore"), 0o644); err != nil {
		t.Fatal(err)
	}
	writePubKey(t, dir, "alice")
	kr, err := LoadKeyring(dir, "")
	if err != nil {
		t.Fatal(err)
	}
	if got := kr.Identities(); !reflect.DeepEqual(got, []string{"alice"}) {
		t.Fatalf("Identities = %v, want [alice]", got)
	}
}

func TestLoadKeyring_BadPubFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "bad.pub"), []byte("not a PEM"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadKeyring(dir, "")
	if !errors.Is(err, ErrBadPEM) {
		t.Fatalf("err = %v, want ErrBadPEM", err)
	}
}

func TestLoadKeyring_PrivateKey(t *testing.T) {
	dir := t.TempDir()
	alice := writePubKey(t, dir, "alice")
	privPath := filepath.Join(dir, "key")
	writePrivKey(t, privPath, alice)

	kr, err := LoadKeyring(dir, privPath)
	if err != nil {
		t.Fatal(err)
	}
	if !kr.HasPrivateKey() {
		t.Fatal("HasPrivateKey should be true")
	}

	// End-to-end: wrap for alice, unwrap via keyring.
	dk, _ := NewDataKey()
	wrapped, err := WrapKey(dk, alice.PublicKey())
	if err != nil {
		t.Fatal(err)
	}
	got, err := kr.Unwrap(wrapped)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, dk) {
		t.Fatal("unwrap via keyring mismatch")
	}
}

func TestLoadKeyring_PrivateKeyMissing(t *testing.T) {
	dir := t.TempDir()
	_, err := LoadKeyring(dir, filepath.Join(dir, "does-not-exist"))
	if err == nil {
		t.Fatal("expected error for explicit missing private key")
	}
}

func TestLoadKeyring_PrivateKeyBadPEM(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "key")
	if err := os.WriteFile(path, []byte("garbage"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := LoadKeyring(dir, path)
	if !errors.Is(err, ErrBadPEM) {
		t.Fatalf("err = %v, want ErrBadPEM", err)
	}
}

func TestKeyring_Unwrap_NoPrivateKey(t *testing.T) {
	dir := t.TempDir()
	kr, err := LoadKeyring(dir, "")
	if err != nil {
		t.Fatal(err)
	}
	_, err = kr.Unwrap(make([]byte, wrappedBlobSize))
	if !errors.Is(err, ErrNoPrivateKey) {
		t.Fatalf("err = %v, want ErrNoPrivateKey", err)
	}
}

func TestLoadKeyring_ReadDirPermissionError(t *testing.T) {
	// Pass a path that exists but is a file, not a directory, to
	// trigger a non-ErrNotExist read error.
	dir := t.TempDir()
	file := filepath.Join(dir, "not-a-dir")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadKeyring(file, "")
	if err == nil {
		t.Fatal("expected error reading a file as a dir")
	}
}
