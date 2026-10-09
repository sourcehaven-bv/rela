package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"regexp"
	"slices"
	"sync"

	"github.com/zalando/go-keyring"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/hostconfig"
	"github.com/Sourcehaven-BV/rela/internal/secrets"
	"github.com/Sourcehaven-BV/rela/internal/state"
)

// Host config on the desktop.
//
// A project's secrets live in the OS keychain, and its AI and mail settings in
// the project's own state (inside rela.db for a document). The .rela
// directory is read as a fallback, so a project set up before this, or on a
// Linux machine without a Secret Service, keeps working.
//
// Keychain items are keyed by a random document ID stored in the project, not
// by its path, so a moved or renamed document keeps its secrets. See
// docs/architecture/desktop-documents.md.

const (
	// keychainService is the service name every rela keychain item carries.
	keychainService = "rela-desktop"

	// documentIDKey is the state key holding the project's document ID.
	documentIDKey = "desktop/document-id"

	// hostFileKeyPrefix is the state key prefix for ai.yaml and mail.yaml.
	hostFileKeyPrefix = "desktop/hostconfig/"

	// documentIDBytes is the length of a document ID before hex encoding.
	documentIDBytes = 16

	// maxSecretBytes is the longest secret value accepted. Windows Credential
	// Manager stores at most 2560 bytes and macOS limits the whole `security`
	// command to 4096, so this leaves room for the base64 go-keyring adds.
	maxSecretBytes = 2000
)

// secretNamePattern is what a secret name may look like: the keys scripts
// read with rela.secret(), and part of a keychain account name.
var secretNamePattern = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,64}$`)

// keychain is the part of the OS credential store the desktop uses.
type keychain interface {
	Get(service, user string) (string, error)
	Set(service, user, password string) error
	Delete(service, user string) error
}

// osKeychain is the real credential store, through go-keyring.
type osKeychain struct{}

func (osKeychain) Get(service, user string) (string, error) { return keyring.Get(service, user) }
func (osKeychain) Set(service, user, password string) error {
	return keyring.Set(service, user, password)
}
func (osKeychain) Delete(service, user string) error { return keyring.Delete(service, user) }

// keychainSecrets reads and writes each project's secrets in the keychain.
//
// One item holds one secret, under the account "<document id>/<name>",
// because the platforms limit an item's size. A further item under the bare
// document ID is the document's index: the secret names, because go-keyring
// cannot list items, and the places allowed to read them.
//
// The places exist because the document ID comes from the document. A
// document someone else made can carry the ID of one of the user's own
// documents, copied from a document the user shared. Its scripts must not
// read that document's secrets just by opening: they are released only to a
// place (a project's .rela directory) the user set a secret in or approved.
//
// Entries are cached per document after the first read: a script runs on
// every automation, and each keychain read starts a process on macOS. Only
// this type writes the items, so the cache is invalidated where they change.
type keychainSecrets struct {
	kc    keychain
	mu    sync.Mutex
	cache map[string]*keychainEntry // document id → entry
}

// keychainEntry is one document's index item plus its secret values.
type keychainEntry struct {
	Names  []string `json:"names"`
	Places []string `json:"places"`
	values map[string]string
}

// maxPlaces bounds the places one index keeps, so the item stays under the
// platforms' size limits. The oldest place is dropped first.
const maxPlaces = 32

// placeKey identifies a place in the index without storing its path.
func placeKey(place string) string {
	sum := sha256.Sum256([]byte(place))
	return hex.EncodeToString(sum[:8])
}

// newKeychainSecrets returns a store over kc.
//
// Nil: kc is rejected.
func newKeychainSecrets(kc keychain) (*keychainSecrets, error) {
	if kc == nil {
		return nil, errors.New("keychain secrets: nil keychain")
	}
	return &keychainSecrets{kc: kc, cache: map[string]*keychainEntry{}}, nil
}

// all returns every secret of a document; an empty map when it has none.
// It does not check the place: callers that release secrets to scripts use
// [keychainSecrets.forPlace].
func (k *keychainSecrets) all(docID string) (map[string]string, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	e, err := k.entry(docID)
	if err != nil {
		return nil, err
	}
	return maps.Clone(e.values), nil
}

// forPlace returns a document's secrets when place may read them. A
// document with no secrets has nothing to withhold, so trusted is then true.
func (k *keychainSecrets) forPlace(docID, place string) (values map[string]string, trusted bool, err error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	e, err := k.entry(docID)
	if err != nil {
		return nil, false, err
	}
	if len(e.values) > 0 && !slices.Contains(e.Places, placeKey(place)) {
		return nil, false, nil
	}
	return maps.Clone(e.values), true, nil
}

// trust lets place read a document's secrets.
func (k *keychainSecrets) trust(docID, place string) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	e, err := k.entry(docID)
	if err != nil {
		return err
	}
	delete(k.cache, docID)
	return k.writeIndex(docID, e.Names, withPlace(e.Places, place))
}

// errNotTrusted refuses a change at a place that may not read the secrets
// already stored for the document.
var errNotTrusted = errors.New("allow this document to use its existing secrets first")

// set stores one secret, adds it to the index and lets place read it: the
// user setting a secret for a project trusts that project. At a place that
// may not read the secrets already stored, it is refused, because adding one
// would approve them all without the user being asked.
func (k *keychainSecrets) set(docID, place, name, value string) error {
	if !secretNamePattern.MatchString(name) {
		return fmt.Errorf("a secret name uses only letters, digits, '.', '_' and '-' (at most 64): %q", name)
	}
	return k.put(docID, place, name, value)
}

// put is set without the name check. The token store uses it for its
// `token/` items, whose names a secret name cannot take.
func (k *keychainSecrets) put(docID, place, name, value string) error {
	if value == "" {
		return errors.New("a secret needs a value")
	}
	if len(value) > maxSecretBytes {
		return fmt.Errorf("a secret is at most %d bytes", maxSecretBytes)
	}
	k.mu.Lock()
	defer k.mu.Unlock()
	e, err := k.entry(docID)
	if err != nil {
		return err
	}
	if len(e.values) > 0 && !slices.Contains(e.Places, placeKey(place)) {
		return errNotTrusted
	}
	delete(k.cache, docID)
	if err := k.kc.Set(keychainService, docID+"/"+name, value); err != nil {
		return fmt.Errorf("store secret %s in the keychain: %w", name, err)
	}
	names := e.Names
	isNew := !slices.Contains(names, name)
	if isNew {
		names = append(slices.Clone(names), name)
	}
	if err := k.writeIndex(docID, names, withPlace(e.Places, place)); err != nil {
		if isNew {
			// Unlisted, the item could be neither shown nor removed.
			_ = k.kc.Delete(keychainService, docID+"/"+name)
		}
		return err
	}
	return nil
}

// remove deletes one secret and drops it from the index. Like set, it is
// refused at a place that may not read the secrets: a copy carrying the ID
// must not delete the original's credentials.
func (k *keychainSecrets) remove(docID, place, name string) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	e, err := k.entry(docID)
	if err != nil {
		return err
	}
	if len(e.values) > 0 && !slices.Contains(e.Places, placeKey(place)) {
		return errNotTrusted
	}
	delete(k.cache, docID)
	if err := k.kc.Delete(keychainService, docID+"/"+name); err != nil && !errors.Is(err, keyring.ErrNotFound) {
		return fmt.Errorf("remove secret %s from the keychain: %w", name, err)
	}
	names := slices.DeleteFunc(slices.Clone(e.Names), func(n string) bool { return n == name })
	return k.writeIndex(docID, names, e.Places)
}

// withPlace returns places with place added last, keeping at most maxPlaces.
func withPlace(places []string, place string) []string {
	key := placeKey(place)
	out := slices.DeleteFunc(slices.Clone(places), func(p string) bool { return p == key })
	out = append(out, key)
	if len(out) > maxPlaces {
		out = out[len(out)-maxPlaces:]
	}
	return out
}

// entry reads a document's index and secret values, through the cache.
// Callers hold mu and must not change the result.
func (k *keychainSecrets) entry(docID string) (*keychainEntry, error) {
	if e, ok := k.cache[docID]; ok {
		return e, nil
	}
	e := &keychainEntry{}
	raw, err := k.kc.Get(keychainService, docID)
	switch {
	case errors.Is(err, keyring.ErrNotFound):
	case err != nil:
		return nil, fmt.Errorf("read the secret list from the keychain: %w", err)
	default:
		if err := json.Unmarshal([]byte(raw), e); err != nil {
			return nil, fmt.Errorf("the keychain's secret list for this document is damaged: %w", err)
		}
	}
	e.values = make(map[string]string, len(e.Names))
	for _, name := range e.Names {
		v, err := k.kc.Get(keychainService, docID+"/"+name)
		if errors.Is(err, keyring.ErrNotFound) {
			continue // removed by hand; the index is only a list
		}
		if err != nil {
			return nil, fmt.Errorf("read secret %s from the keychain: %w", name, err)
		}
		e.values[name] = v
	}
	k.cache[docID] = e
	return e, nil
}

// writeIndex replaces a document's index item, removing it when it lists no
// secrets. Callers hold mu.
func (k *keychainSecrets) writeIndex(docID string, names, places []string) error {
	if len(names) == 0 {
		if err := k.kc.Delete(keychainService, docID); err != nil && !errors.Is(err, keyring.ErrNotFound) {
			return fmt.Errorf("update the secret list in the keychain: %w", err)
		}
		return nil
	}
	names = slices.Sorted(slices.Values(names))
	raw, err := json.Marshal(keychainEntry{Names: names, Places: places})
	if err != nil {
		return err
	}
	if err := k.kc.Set(keychainService, docID, string(raw)); err != nil {
		return fmt.Errorf("update the secret list in the keychain: %w", err)
	}
	return nil
}

// documentIDPattern is the shape documentID mints. A stored ID of any other
// shape is replaced: the ID becomes part of keychain account names, so a
// '/' in it could reach another document's items.
var documentIDPattern = regexp.MustCompile(`^[0-9a-f]{32}$`)

// documentID returns the project's document ID, creating it on first use.
func documentID(ctx context.Context, kv state.KV) (string, error) {
	raw, err := kv.Get(ctx, documentIDKey)
	if err == nil && documentIDPattern.Match(raw) {
		return string(raw), nil
	}
	if err != nil && !os.IsNotExist(err) && !errors.Is(err, fs.ErrNotExist) {
		return "", fmt.Errorf("read document id: %w", err)
	}
	buf := make([]byte, documentIDBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	id := hex.EncodeToString(buf)
	if err := kv.Put(ctx, documentIDKey, []byte(id)); err != nil {
		return "", fmt.Errorf("store document id: %w", err)
	}
	return id, nil
}

// desktopHost is a project's host config on the desktop. It implements
// lua.HostConfig and mail.Source.
type desktopHost struct {
	// ctx is the context state reads use. HostConfig.File takes none, because
	// the .rela implementation reads files; storing the opening context keeps
	// its values without inventing a context.Background per read.
	ctx     context.Context //nolint:containedctx // HostConfig.File has no ctx parameter
	dir     hostconfig.Dir
	kv      state.KV
	secrets *keychainSecrets
	docID   string
}

// newDesktopHost returns the host config for the project whose .rela
// directory is relaDir and whose state is kv.
//
// Nil: kv and kc are rejected.
func newDesktopHost(ctx context.Context, relaDir string, kv state.KV, kc *keychainSecrets) (*desktopHost, error) {
	if kv == nil || kc == nil {
		return nil, errors.New("desktop host config: nil state or keychain")
	}
	id, err := documentID(ctx, kv)
	if err != nil {
		return nil, err
	}
	return &desktopHost{
		ctx: context.WithoutCancel(ctx), dir: hostconfig.Dir(relaDir), kv: kv, secrets: kc, docID: id,
	}, nil
}

// Secrets returns the file's secrets for scriptPath with the keychain's on
// top. The keychain holds only global secrets: per-script overrides stay a
// secrets.yaml feature. Keychain secrets are left out until this place may
// read them (see keychainSecrets).
func (h *desktopHost) Secrets(scriptPath string) (map[string]string, error) {
	out, err := h.dir.Secrets(scriptPath)
	if err != nil && !errors.Is(err, secrets.ErrNotFound) {
		return nil, err
	}
	fromKeychain, _, kerr := h.secrets.forPlace(h.docID, h.Path())
	if kerr != nil {
		return nil, kerr
	}
	if out == nil && len(fromKeychain) == 0 {
		return nil, secrets.ErrNotFound
	}
	if out == nil {
		out = map[string]string{}
	}
	maps.Copy(out, withoutTokenItems(fromKeychain))
	return out, nil
}

// File returns ai.yaml or mail.yaml from the project's state, else from the
// .rela directory.
func (h *desktopHost) File(name string) ([]byte, error) {
	if err := hostconfig.CheckName(name); err != nil {
		return nil, err
	}
	data, err := h.kv.Get(h.ctx, hostFileKeyPrefix+name)
	if err == nil {
		return data, nil
	}
	if !os.IsNotExist(err) && !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	return h.dir.File(name)
}

// Path is the project's .rela directory.
func (h *desktopHost) Path() string { return h.dir.Path() }

// projectOptions are the appbuild options a desktop project opens with.
//
// No ACL: the desktop user owns the machine and the project files, so an
// acl.yaml could not restrict them anyway, and its roles are written for a
// server's users. It still applies when the project is served. Secrets come
// from the keychain when there is one.
func projectOptions(relaDir string, kc *keychainSecrets) []appbuild.Option {
	// The app stays open, so background automation actions use the queue.
	opts := []appbuild.Option{appbuild.WithACL(acl.NopACL{}), appbuild.WithBackgroundAutomationJobs()}
	if kc != nil {
		opts = append(opts,
			appbuild.WithHostConfig(func(kv state.KV) (appbuild.HostConfig, error) {
				return newDesktopHost(context.Background(), relaDir, kv, kc)
			}),
			// Connector tokens go in the keychain too, beside the secrets
			// (TKT-01KZSO).
			appbuild.WithTokenStore(newKeychainTokens))
	}
	return opts
}
