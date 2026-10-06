package main

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/zalando/go-keyring"

	"github.com/Sourcehaven-BV/rela/internal/ai"
	"github.com/Sourcehaven-BV/rela/internal/hostconfig"
	"github.com/Sourcehaven-BV/rela/internal/secrets"
)

// fakeKeychain is an in-memory keychain that counts reads.
type fakeKeychain struct {
	mu    sync.Mutex
	items map[string]string
	gets  int
	// failSet makes Set fail for this account.
	failSet string
}

func newFakeKeychain() *fakeKeychain { return &fakeKeychain{items: map[string]string{}} }

func (f *fakeKeychain) Get(service, user string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.gets++
	v, ok := f.items[service+"|"+user]
	if !ok {
		return "", keyring.ErrNotFound
	}
	return v, nil
}

func (f *fakeKeychain) Set(service, user, password string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if user == f.failSet {
		return errors.New("keychain refused")
	}
	f.items[service+"|"+user] = password
	return nil
}

func (f *fakeKeychain) Delete(service, user string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.items[service+"|"+user]; !ok {
		return keyring.ErrNotFound
	}
	delete(f.items, service+"|"+user)
	return nil
}

// memKV is an in-memory state.KV.
type memKV struct {
	mu sync.Mutex
	m  map[string][]byte
}

func newMemKV() *memKV { return &memKV{m: map[string][]byte{}} }

func (k *memKV) Get(_ context.Context, key string) ([]byte, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	v, ok := k.m[key]
	if !ok {
		return nil, &fs.PathError{Op: "get", Path: key, Err: fs.ErrNotExist}
	}
	return v, nil
}

func (k *memKV) Put(_ context.Context, key string, data []byte) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.m[key] = data
	return nil
}

func (k *memKV) Delete(_ context.Context, key string) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	delete(k.m, key)
	return nil
}

func TestKeychainSecrets(t *testing.T) {
	kc := newFakeKeychain()
	store, newErr := newKeychainSecrets(kc)
	require.NoError(t, newErr)

	t.Run("set, read and remove", func(t *testing.T) {
		require.NoError(t, store.set("doc1", "here", "token", "abc"))
		require.NoError(t, store.set("doc1", "here", "api.key", "def"))
		require.NoError(t, store.set("doc2", "here", "token", "other"))

		got, err := store.all("doc1")
		require.NoError(t, err)
		assert.Equal(t, map[string]string{"token": "abc", "api.key": "def"}, got)
		assert.Equal(t, `{"names":["api.key","token"],"places":["`+placeKey("here")+`"]}`,
			kc.items[keychainService+"|doc1"], "index lists names, sorted, and the place that set them")

		require.NoError(t, store.remove("doc1", "here", "token"))
		got, err = store.all("doc1")
		require.NoError(t, err)
		assert.Equal(t, map[string]string{"api.key": "def"}, got)

		require.NoError(t, store.remove("doc1", "here", "api.key"))
		_, indexed := kc.items[keychainService+"|doc1"]
		assert.False(t, indexed, "an empty index is removed")

		other, err := store.all("doc2")
		require.NoError(t, err)
		assert.Equal(t, map[string]string{"token": "other"}, other, "documents do not share secrets")
	})

	t.Run("reads are cached until a change", func(t *testing.T) {
		require.NoError(t, store.set("doc3", "here", "a", "1"))
		_, err := store.all("doc3")
		require.NoError(t, err)
		before := kc.gets
		_, err = store.all("doc3")
		require.NoError(t, err)
		assert.Equal(t, before, kc.gets, "second read is served from the cache")

		require.NoError(t, store.set("doc3", "here", "a", "2"))
		got, err := store.all("doc3")
		require.NoError(t, err)
		assert.Equal(t, "2", got["a"])
	})

	t.Run("a cached map cannot be changed by a caller", func(t *testing.T) {
		got, err := store.all("doc3")
		require.NoError(t, err)
		got["a"] = "tampered"
		again, err := store.all("doc3")
		require.NoError(t, err)
		assert.Equal(t, "2", again["a"])
	})

	t.Run("rejects bad names and values", func(t *testing.T) {
		for _, name := range []string{"", "a/b", "has space", strings.Repeat("x", 65)} {
			require.Error(t, store.set("doc1", "here", name, "v"), "name %q", name)
		}
		require.Error(t, store.set("doc1", "here", "empty", ""))
		require.Error(t, store.set("doc1", "here", "big", strings.Repeat("x", maxSecretBytes+1)))
	})

	t.Run("a damaged index is reported", func(t *testing.T) {
		kc.items[keychainService+"|doc9"] = "not json"
		_, err := store.all("doc9")
		assert.Error(t, err)
	})

	_, newErr = newKeychainSecrets(nil)
	require.Error(t, newErr)
}

func TestDocumentID(t *testing.T) {
	kv := newMemKV()
	first, err := documentID(context.Background(), kv)
	require.NoError(t, err)
	assert.Len(t, first, 2*documentIDBytes)
	again, err := documentID(context.Background(), kv)
	require.NoError(t, err)
	assert.Equal(t, first, again, "the id is created once")
	other, err := documentID(context.Background(), newMemKV())
	require.NoError(t, err)
	assert.NotEqual(t, first, other)
}

func TestDesktopHost(t *testing.T) {
	ctx := context.Background()
	relaDir := t.TempDir()
	kv := newMemKV()
	store, setupErr := newKeychainSecrets(newFakeKeychain())
	require.NoError(t, setupErr)
	host, setupErr := newDesktopHost(ctx, relaDir, kv, store)
	require.NoError(t, setupErr)

	t.Run("no secrets anywhere is ErrNotFound", func(t *testing.T) {
		_, err := host.Secrets("")
		assert.ErrorIs(t, err, secrets.ErrNotFound)
	})

	t.Run("keychain only", func(t *testing.T) {
		require.NoError(t, store.set(host.docID, host.Path(), "token", "kc"))
		got, err := host.Secrets("")
		require.NoError(t, err)
		assert.Equal(t, map[string]string{"token": "kc"}, got)
	})

	t.Run("file and keychain merge, keychain wins", func(t *testing.T) {
		require.NoError(t, os.WriteFile(filepath.Join(relaDir, secrets.ConfigFile),
			[]byte("token: file\nonly_file: f\n"), 0o600))
		got, err := host.Secrets("")
		require.NoError(t, err)
		assert.Equal(t, map[string]string{"token": "kc", "only_file": "f"}, got)
	})

	t.Run("files come from state, then the directory", func(t *testing.T) {
		_, err := host.File(hostconfig.AIFile)
		require.ErrorIs(t, err, fs.ErrNotExist, "absent everywhere")

		require.NoError(t, os.WriteFile(filepath.Join(relaDir, hostconfig.AIFile), []byte("from: dir\n"), 0o600))
		data, err := host.File(hostconfig.AIFile)
		require.NoError(t, err)
		assert.Equal(t, "from: dir\n", string(data))

		require.NoError(t, kv.Put(ctx, hostFileKeyPrefix+hostconfig.AIFile, []byte("from: state\n")))
		data, err = host.File(hostconfig.AIFile)
		require.NoError(t, err)
		assert.Equal(t, "from: state\n", string(data))

		_, err = host.File("secrets.yaml")
		assert.ErrorIs(t, err, hostconfig.ErrUnknownFile)
	})

	t.Run("another place reads the secrets only once trusted", func(t *testing.T) {
		moved, err := newDesktopHost(ctx, t.TempDir(), kv, store)
		require.NoError(t, err)
		_, err = moved.Secrets("")
		require.ErrorIs(t, err, secrets.ErrNotFound, "a copy carrying the id must not read the keychain")
		require.ErrorIs(t, store.set(moved.docID, moved.Path(), "new", "v"), errNotTrusted,
			"setting a secret must not approve the existing ones")
		require.ErrorIs(t, store.remove(moved.docID, moved.Path(), "token"), errNotTrusted,
			"a copy must not delete the original's secrets")

		require.NoError(t, store.trust(moved.docID, moved.Path()))
		got, err := moved.Secrets("")
		require.NoError(t, err)
		assert.Equal(t, "kc", got["token"])
	})

	_, setupErr = newDesktopHost(ctx, relaDir, nil, store)
	require.Error(t, setupErr)
	_, setupErr = newDesktopHost(ctx, relaDir, kv, nil)
	require.Error(t, setupErr)
	assert.Equal(t, relaDir, host.Path())
}

func TestDocumentID_ReplacesMalformed(t *testing.T) {
	ctx := context.Background()
	for _, bad := range []string{"other/../doc", "ABCDEF0123456789ABCDEF0123456789", "short"} {
		kv := newMemKV()
		require.NoError(t, kv.Put(ctx, documentIDKey, []byte(bad)))
		id, err := documentID(ctx, kv)
		require.NoError(t, err)
		assert.NotEqual(t, bad, id)
		assert.Regexp(t, documentIDPattern, id)
	}
}

func TestWithPlace_Bounded(t *testing.T) {
	var places []string
	for i := range maxPlaces + 5 {
		places = withPlace(places, strconv.Itoa(i))
	}
	assert.Len(t, places, maxPlaces)
	assert.Equal(t, placeKey(strconv.Itoa(maxPlaces+4)), places[len(places)-1], "the newest place is kept")
	assert.Len(t, withPlace(places, strconv.Itoa(maxPlaces+4)), maxPlaces, "a known place is not added twice")
}

func TestCheckHostFile(t *testing.T) {
	require.NoError(t, checkHostFile(hostconfig.AIFile, []byte("base_url: http://localhost:1\nmodel: m\n")))
	require.Error(t, checkHostFile(hostconfig.AIFile, []byte("model: m\n")))
	require.NoError(t, checkHostFile(hostconfig.MailFile, []byte("transport: memory\nfrom: rela@example.com\n")))
	require.Error(t, checkHostFile(hostconfig.MailFile, []byte("transport: carrier-pigeon\n")))
}

// fakeWindow stands in for the Wails window a bound call came from.
type fakeWindow struct{ name string }

func (w fakeWindow) Name() string { return w.name }

// fromWindow is a bound call's context from the window with this name.
func fromWindow(name string) context.Context {
	return context.WithValue(context.Background(), application.WindowKey, fakeWindow{name})
}

func TestProjectSettings_RefusesOtherWindows(t *testing.T) {
	_, err := newProjectSettings(nil, nil)
	require.Error(t, err)
	s, err := newProjectSettings(&Desktop{registry: newProjectRegistry()}, nil)
	require.NoError(t, err)
	s.windows.add("settings-1", "gone")

	for _, ctx := range []context.Context{context.Background(), fromWindow("main"), fromWindow("")} {
		assert.Equal(t, errNotSettingsWindow.Error(), s.Load(ctx).Error)
		assert.Equal(t, errNotSettingsWindow.Error(), s.SetSecret(ctx, "a", "b"))
		assert.Equal(t, errNotSettingsWindow.Error(), s.DeleteSecret(ctx, "a"))
		assert.Equal(t, errNotSettingsWindow.Error(), s.TrustSecrets(ctx))
		assert.Equal(t, errNotSettingsWindow.Error(), s.SaveFile(ctx, hostconfig.AIFile, ""))
	}
	assert.NotEmpty(t, s.Load(fromWindow("settings-1")).Error, "a closed project is not edited")
	s.windows.remove("settings-1")
	assert.Equal(t, errNotSettingsWindow.Error(), s.Load(fromWindow("settings-1")).Error)
}

func TestKeychainSecrets_SetRollsBackWhenIndexFails(t *testing.T) {
	kc := newFakeKeychain()
	kc.failSet = "doc3"
	store, err := newKeychainSecrets(kc)
	require.NoError(t, err)
	require.Error(t, store.set("doc3", "here", "token", "v"))
	assert.NotContains(t, kc.items, keychainService+"|doc3/token", "an unlisted item would be unreachable")
}

func TestCheckHostFile_RefusesKeyInAIConfig(t *testing.T) {
	err := checkHostFile(hostconfig.AIFile, []byte("base_url: http://localhost:1/v1\nmodel: m\napi_key: sk-x\n"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), ai.SecretKey)
}
