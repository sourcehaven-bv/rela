package tokenstore_test

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/secrets"
	"github.com/Sourcehaven-BV/rela/internal/state"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/tokenstore"
	"github.com/Sourcehaven-BV/rela/internal/tokenstore/tokenstoretest"
)

func newKV(tb testing.TB) state.KV {
	tb.Helper()
	mem := storage.NewMemFS()
	if err := mem.MkdirAll("/root", 0o755); err != nil {
		tb.Fatal(err)
	}
	rfs, err := storage.NewRootedFS(mem, "/root")
	if err != nil {
		tb.Fatal(err)
	}
	return state.NewFSKV(rfs)
}

func testKey(b byte) []byte { return bytes.Repeat([]byte{b}, tokenstore.KeySize) }

func newSealed(tb testing.TB, kv state.KV, key []byte, scope string) *tokenstore.Sealed {
	tb.Helper()
	s, err := tokenstore.NewSealed(kv, key, scope)
	if err != nil {
		tb.Fatal(err)
	}
	return s
}

func TestSealed_Conformance(t *testing.T) {
	tokenstoretest.RunAll(t, func(tb testing.TB) tokenstore.Store {
		tb.Helper()
		return newSealed(tb, newKV(tb), testKey(1), "proj-a")
	})
}

func TestNewSealed_RejectsBadInput(t *testing.T) {
	if _, err := tokenstore.NewSealed(nil, testKey(1), "s"); err == nil {
		t.Error("nil kv accepted")
	}
	if _, err := tokenstore.NewSealed(newKV(t), testKey(1)[:16], "s"); err == nil {
		t.Error("16-byte key accepted")
	}
	if _, err := tokenstore.NewSealed(newKV(t), testKey(1), ""); err == nil {
		t.Error("empty scope accepted")
	}
	if _, err := tokenstore.NewSealed(newKV(t), testKey(1), "a|b"); err == nil {
		t.Error("scope with the AAD separator accepted")
	}
}

// TKT-01KZSO R11: a record sealed under another key reports ErrWrongKey; a
// record moved to another name or scope does not open.
func TestSealed_BindsKeyNameAndScope(t *testing.T) {
	ctx := context.Background()
	kv := newKV(t)
	a := newSealed(t, kv, testKey(1), "proj-a")
	n, _ := tokenstore.ParseName("basecamp")
	if err := a.Put(ctx, n, tokenstore.Token{Refresh: "r", Access: "a"}); err != nil {
		t.Fatal(err)
	}

	other := newSealed(t, kv, testKey(2), "proj-a")
	if _, err := other.Get(ctx, n); !errors.Is(err, tokenstore.ErrWrongKey) {
		t.Errorf("other key: %v, want ErrWrongKey", err)
	}

	otherScope := newSealed(t, kv, testKey(1), "proj-b")
	if _, err := otherScope.Get(ctx, n); !errors.Is(err, tokenstore.ErrCorrupt) {
		t.Errorf("other scope: %v, want ErrCorrupt", err)
	}

	raw, err := kv.Get(ctx, "tokens/basecamp")
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(raw, []byte(`"r"`)) || bytes.Contains(raw, []byte("refresh")) {
		t.Fatal("the stored record holds plaintext")
	}
	if err := kv.Put(ctx, "tokens/crm", raw); err != nil {
		t.Fatal(err)
	}
	crm, _ := tokenstore.ParseName("crm")
	if _, err := a.Get(ctx, crm); !errors.Is(err, tokenstore.ErrCorrupt) {
		t.Errorf("copied to another name: %v, want ErrCorrupt", err)
	}

	tampered := bytes.Clone(raw)
	tampered[len(tampered)-1] ^= 1
	if err := kv.Put(ctx, "tokens/basecamp", tampered); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Get(ctx, n); !errors.Is(err, tokenstore.ErrCorrupt) {
		t.Errorf("tampered: %v, want ErrCorrupt", err)
	}

	if err := kv.Put(ctx, "tokens/basecamp", []byte{9, 9}); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Get(ctx, n); !errors.Is(err, tokenstore.ErrCorrupt) {
		t.Errorf("short record: %v, want ErrCorrupt", err)
	}
}

func TestParseName_AgreesWithPrincipal(t *testing.T) {
	for _, s := range []string{
		"basecamp", "a", "0x", "crm_2-eu", strings.Repeat("z", 63), strings.Repeat("z", 64),
		"", "-a", "_a", "A", "a b", "a/b", "a.b", "a:b", "con", "lpt1", "console", "ü",
	} {
		_, err := tokenstore.ParseName(s)
		if got, want := err == nil, principal.ValidIntegrationName(s); got != want {
			t.Errorf("ParseName(%q) ok=%v, principal.ValidIntegrationName=%v", s, got, want)
		}
	}
}

func TestKeySecret_AgreesWithSecrets(t *testing.T) {
	if tokenstore.KeySecret != secrets.TokenKey {
		t.Fatalf("tokenstore.KeySecret %q != secrets.TokenKey %q", tokenstore.KeySecret, secrets.TokenKey)
	}
}

func TestLoadKey(t *testing.T) {
	good := "AQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQE=" // 32 bytes of 0x01
	env := func(v string) func(string) string {
		return func(k string) string {
			if k == tokenstore.KeyEnv {
				return v
			}
			return ""
		}
	}
	if _, err := tokenstore.LoadKey(nil, env("")); !errors.Is(err, tokenstore.ErrNoKey) {
		t.Errorf("no key: %v", err)
	}
	k, err := tokenstore.LoadKey(map[string]string{"token_key": good}, env("garbage"))
	if err != nil || !bytes.Equal(k, testKey(1)) {
		t.Errorf("secrets.yaml must win: %v", err)
	}
	if _, err = tokenstore.LoadKey(nil, env(good)); err != nil {
		t.Errorf("env key: %v", err)
	}
	_, err = tokenstore.LoadKey(nil, env("c2hvcnQ="))
	if err == nil || !strings.Contains(err.Error(), tokenstore.KeyEnv) || !strings.Contains(err.Error(), "32") {
		t.Errorf("short env key: %v", err)
	}
	if _, err := tokenstore.LoadKey(map[string]string{"token_key": "!!"}, nil); err == nil {
		t.Error("non-base64 key accepted")
	}
}
