package app

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/encryption"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/storage"
)

// These tests exercise the production cryptoAdapter directly.
// Previous coverage all went through hand-rolled test fakes in the
// fsstore package, which hid C1 (tamper collapsed to
// no-matching-key) for an entire slice. Add white-box tests in the
// app package so the adapter's error taxonomy is pinned down.

// buildKeyring writes <dir>/<id>.pub for each recipient, optionally
// writes a local private key, and calls encryption.LoadKeyring —
// exactly the path production uses. Separate temp dir per test keeps
// fixtures isolated.
func buildKeyring(
	t *testing.T,
	recipients map[string]*encryption.PublicKey,
	priv *encryption.Keypair,
) *encryption.Keyring {
	t.Helper()
	dir := t.TempDir()
	for id, pub := range recipients {
		pem, err := encryption.MarshalPublicKeyPEM(pub)
		if err != nil {
			t.Fatalf("marshal pub %s: %v", id, err)
		}
		if err := os.WriteFile(filepath.Join(dir, id+".pub"), pem, 0o644); err != nil {
			t.Fatalf("write pub %s: %v", id, err)
		}
	}
	privPath := ""
	if priv != nil {
		pem, err := encryption.MarshalPrivateKeyPEM(priv)
		if err != nil {
			t.Fatalf("marshal priv: %v", err)
		}
		privPath = filepath.Join(dir, "local.key")
		if err := os.WriteFile(privPath, pem, 0o600); err != nil {
			t.Fatalf("write priv: %v", err)
		}
	}
	kr, err := encryption.LoadKeyring(dir, privPath)
	if err != nil {
		t.Fatalf("LoadKeyring: %v", err)
	}
	return kr
}

// buildGroups loads a Groups via the real YAML loader so we stay on
// the production code path.
func buildGroups(t *testing.T, config map[string][]string) *metamodel.Groups {
	t.Helper()
	var b strings.Builder
	b.WriteString("groups:\n")
	for g, ids := range config {
		b.WriteString("  " + g + ":\n")
		for _, id := range ids {
			b.WriteString("    - " + id + "\n")
		}
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "groups.yaml"), []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	g, err := metamodel.LoadGroups(dir, storage.NewOsFS())
	if err != nil {
		t.Fatalf("LoadGroups: %v", err)
	}
	return g
}

func minimalMeta() *metamodel.Metamodel {
	return &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"ticket": {
				Label:    "Ticket",
				IDPrefix: "TKT",
				Properties: map[string]metamodel.PropertyDef{
					"description": {Type: "string", Encrypted: "engineering"},
				},
			},
		},
	}
}

func newKeypair(t *testing.T) *encryption.Keypair {
	t.Helper()
	k, err := encryption.GenerateKeypair()
	if err != nil {
		t.Fatal(err)
	}
	return k
}

func TestCryptoAdapter_UnwrapAny_LocalIsRecipient(t *testing.T) {
	// alice + bob in engineering; adapter holds alice's private key.
	// A wrap labeled for alice decrypts cleanly.
	alice := newKeypair(t)
	bob := newKeypair(t)
	recipients := map[string]*encryption.PublicKey{
		"alice": alice.PublicKey(),
		"bob":   bob.PublicKey(),
	}
	kr := buildKeyring(t, recipients, alice)
	groups := buildGroups(t, map[string][]string{"engineering": {"alice", "bob"}})
	adapter := buildCrypto(minimalMeta(), groups, kr)
	if adapter == nil {
		t.Fatal("buildCrypto returned nil")
	}

	dk, err := encryption.NewDataKey()
	if err != nil {
		t.Fatal(err)
	}
	aliceWrap, err := encryption.WrapKey(dk, alice.PublicKey())
	if err != nil {
		t.Fatal(err)
	}
	bobWrap, err := encryption.WrapKey(dk, bob.PublicKey())
	if err != nil {
		t.Fatal(err)
	}
	wraps := map[string][]byte{"alice": aliceWrap, "bob": bobWrap}

	got, matched, err := adapter.UnwrapAny(wraps)
	if err != nil {
		t.Fatalf("UnwrapAny: %v", err)
	}
	if matched != "alice" {
		t.Errorf("matched = %q, want alice", matched)
	}
	if !bytes.Equal(got, dk) {
		t.Error("dataKey round-trip mismatch")
	}
}

func TestCryptoAdapter_UnwrapAny_LocalNotInGroup(t *testing.T) {
	// adapter holds eve's private key; wraps are only for alice + bob.
	// eve's identity IS in the recipients file (she has a .pub) but
	// isn't in the engineering group's wrap map.
	alice := newKeypair(t)
	bob := newKeypair(t)
	eve := newKeypair(t)
	recipients := map[string]*encryption.PublicKey{
		"alice": alice.PublicKey(),
		"bob":   bob.PublicKey(),
		"eve":   eve.PublicKey(),
	}
	kr := buildKeyring(t, recipients, eve)
	if kr.LocalIdentity() != "eve" {
		t.Fatalf("LocalIdentity = %q, want eve", kr.LocalIdentity())
	}
	groups := buildGroups(t, map[string][]string{"engineering": {"alice", "bob"}})
	adapter := buildCrypto(minimalMeta(), groups, kr)

	dk, _ := encryption.NewDataKey()
	aliceWrap, _ := encryption.WrapKey(dk, alice.PublicKey())
	bobWrap, _ := encryption.WrapKey(dk, bob.PublicKey())
	wraps := map[string][]byte{"alice": aliceWrap, "bob": bobWrap}

	_, _, err := adapter.UnwrapAny(wraps)
	if !errors.Is(err, encryption.ErrNoMatchingKey) {
		t.Errorf("err = %v, want ErrNoMatchingKey", err)
	}
}

func TestCryptoAdapter_UnwrapAny_TamperedOwnWrapSurfacesDecryptError(t *testing.T) {
	// C1 regression: the earlier adapter probed every wrap and
	// collapsed a genuine ErrDecrypt on alice's wrap into
	// ErrNoMatchingKey. The fix must propagate the decrypt error so
	// fsstore classifies the file as CorruptedFile.
	alice := newKeypair(t)
	bob := newKeypair(t)
	recipients := map[string]*encryption.PublicKey{
		"alice": alice.PublicKey(),
		"bob":   bob.PublicKey(),
	}
	kr := buildKeyring(t, recipients, alice)
	groups := buildGroups(t, map[string][]string{"engineering": {"alice", "bob"}})
	adapter := buildCrypto(minimalMeta(), groups, kr)

	dk, _ := encryption.NewDataKey()
	aliceWrap, _ := encryption.WrapKey(dk, alice.PublicKey())
	bobWrap, _ := encryption.WrapKey(dk, bob.PublicKey())

	// Flip a byte in the middle of alice's wrap.
	aliceWrap[len(aliceWrap)/2] ^= 0x01

	wraps := map[string][]byte{"alice": aliceWrap, "bob": bobWrap}
	_, _, err := adapter.UnwrapAny(wraps)
	if err == nil {
		t.Fatal("expected error on tampered own wrap")
	}
	if errors.Is(err, encryption.ErrNoMatchingKey) {
		t.Errorf("tamper collapsed to no-matching-key (C1 regression): %v", err)
	}
	if !errors.Is(err, encryption.ErrDecrypt) && !errors.Is(err, encryption.ErrBadBlob) {
		t.Errorf("err = %v, want ErrDecrypt or ErrBadBlob", err)
	}
}

func TestCryptoAdapter_UnwrapAny_NoPrivateKey(t *testing.T) {
	alice := newKeypair(t)
	recipients := map[string]*encryption.PublicKey{"alice": alice.PublicKey()}
	kr := buildKeyring(t, recipients, nil) // no private key
	groups := buildGroups(t, map[string][]string{"engineering": {"alice"}})
	adapter := buildCrypto(minimalMeta(), groups, kr)

	dk, _ := encryption.NewDataKey()
	aliceWrap, _ := encryption.WrapKey(dk, alice.PublicKey())
	wraps := map[string][]byte{"alice": aliceWrap}

	_, _, err := adapter.UnwrapAny(wraps)
	if !errors.Is(err, encryption.ErrNoPrivateKey) {
		t.Errorf("err = %v, want ErrNoPrivateKey", err)
	}
}

func TestCryptoAdapter_UnwrapAny_OrphanPrivateKey(t *testing.T) {
	// Private key is a fresh keypair that doesn't correspond to any
	// recipient. LocalIdentity is "" and UnwrapAny returns
	// ErrNoMatchingKey without attempting any wrap.
	alice := newKeypair(t)
	bob := newKeypair(t)
	orphan := newKeypair(t)
	recipients := map[string]*encryption.PublicKey{
		"alice": alice.PublicKey(),
		"bob":   bob.PublicKey(),
	}
	kr := buildKeyring(t, recipients, orphan)
	if kr.LocalIdentity() != "" {
		t.Errorf("LocalIdentity = %q, want empty (orphan key)", kr.LocalIdentity())
	}
	groups := buildGroups(t, map[string][]string{"engineering": {"alice", "bob"}})
	adapter := buildCrypto(minimalMeta(), groups, kr)

	dk, _ := encryption.NewDataKey()
	aliceWrap, _ := encryption.WrapKey(dk, alice.PublicKey())
	wraps := map[string][]byte{"alice": aliceWrap}

	_, _, err := adapter.UnwrapAny(wraps)
	if !errors.Is(err, encryption.ErrNoMatchingKey) {
		t.Errorf("err = %v, want ErrNoMatchingKey", err)
	}
}

func TestBuildCrypto_NilComponents(t *testing.T) {
	// buildCrypto should return nil when any of meta/groups/keyring
	// is missing — the fsstore.Crypto interface spec says nil = fully
	// cleartext-only, and that's how Workspace.New wires it up.
	alice := newKeypair(t)
	recipients := map[string]*encryption.PublicKey{"alice": alice.PublicKey()}
	kr := buildKeyring(t, recipients, alice)
	groups := buildGroups(t, map[string][]string{"engineering": {"alice"}})
	meta := minimalMeta()

	if c := buildCrypto(nil, groups, kr); c != nil {
		t.Error("nil meta should produce nil Crypto")
	}
	if c := buildCrypto(meta, nil, kr); c != nil {
		t.Error("nil groups should produce nil Crypto")
	}
	if c := buildCrypto(meta, groups, nil); c != nil {
		t.Error("nil keyring should produce nil Crypto")
	}
}
