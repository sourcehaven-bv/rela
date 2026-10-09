package main

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/tokenstore"
	"github.com/Sourcehaven-BV/rela/internal/tokenstore/tokenstoretest"
)

func newTestKeychainTokens(tb testing.TB, kc *keychainSecrets, kv *memKV, relaDir string) (*desktopHost, tokenstore.Store) {
	tb.Helper()
	host, err := newDesktopHost(context.Background(), relaDir, kv, kc)
	require.NoError(tb, err)
	st, err := newKeychainTokens(host)
	require.NoError(tb, err)
	return host, st
}

func TestKeychainTokens_Conformance(t *testing.T) {
	tokenstoretest.RunAll(t, func(tb testing.TB) tokenstore.Store {
		tb.Helper()
		kc, err := newKeychainSecrets(newFakeKeychain())
		require.NoError(tb, err)
		_, st := newTestKeychainTokens(tb, kc, newMemKV(), tb.TempDir())
		return st
	})
}

// R3: a document that only carries this document's ID cannot read,
// replace or delete its tokens, and the token items never reach a script.
func TestKeychainTokens_PlaceCheck(t *testing.T) {
	ctx := context.Background()
	kc, err := newKeychainSecrets(newFakeKeychain())
	require.NoError(t, err)
	kv := newMemKV()
	host, st := newTestKeychainTokens(t, kc, kv, t.TempDir())
	tok := tokenstore.Token{Refresh: "R", Access: "A", ExpiresAt: time.Now().Add(time.Hour).UTC()}
	require.NoError(t, st.Put(ctx, "crm", tok))

	got, err := host.Secrets("")
	require.NoError(t, err)
	for k := range got {
		assert.False(t, strings.HasPrefix(k, tokenItemPrefix), "script secrets include token item %s", k)
	}

	_, copied := newTestKeychainTokens(t, kc, kv, t.TempDir())
	_, err = copied.Get(ctx, "crm")
	require.ErrorContains(t, err, "not trusted")
	require.ErrorIs(t, copied.Put(ctx, "crm", tokenstore.Token{Refresh: "evil"}), errNotTrusted)
	require.ErrorIs(t, copied.Delete(ctx, "crm"), errNotTrusted)

	back, err := st.Get(ctx, "crm")
	require.NoError(t, err)
	assert.Equal(t, "R", back.Refresh)
	assert.Equal(t, "A", back.Access)
}

func TestKeychainTokens_DamagedState(t *testing.T) {
	ctx := context.Background()
	kc, err := newKeychainSecrets(newFakeKeychain())
	require.NoError(t, err)
	host, st := newTestKeychainTokens(t, kc, newMemKV(), t.TempDir())
	require.NoError(t, st.Put(ctx, "crm", tokenstore.Token{Refresh: "R"}))
	require.NoError(t, kc.put(host.docID, host.Path(), stateItem("crm"), "{nope"))
	_, err = st.Get(ctx, "crm")
	require.ErrorIs(t, err, tokenstore.ErrCorrupt)

	_, err = newKeychainTokens(nil)
	require.Error(t, err)
}
