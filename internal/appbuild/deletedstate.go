package appbuild

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"slices"
	"strings"
	"sync"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/state"
)

// deletedStates keeps the last state of a deleted face, which an
// `on.deleted` job needs, readable only for as long as the job does
// (TKT-RJS3KKJ).
//
// The job payload holds the state sealed with AES-256-GCM under a key of its
// own. The key lives in state.KV, never in the job table. Erasing the key
// makes the ciphertext in the job row unreadable, wherever that row is: a
// pending job, a finished row the postgres queue has not swept yet, or a dead
// job. The key is erased when the job finishes, and every key of an entity is
// erased when the entity is purged.
//
// The keys of one entity sit under one directory named by the hash of its id,
// with an index of their handles, because state.KV cannot list keys. The
// index is updated read-modify-write under a process-local lock, so two
// processes can lose an entry, and a purge would then miss that key. So a
// purge also writes a new random generation for the entity. seal reads the
// generation before it stores the key and binds it into the sealed state;
// open refuses a state whose generation is no longer current. The
// generation is one Put, so it holds across processes and needs no clock.
// The index only lets a purge delete the keys at once.
type deletedStates struct {
	kv state.KV
	mu sync.Mutex
}

// errDeletedStateErased reports a job whose key was erased: the entity was
// purged, or an earlier delivery of the job finished.
var errDeletedStateErased = errors.New("the deleted state was erased")

// errDeletedStateCorrupt reports a sealed state that can never be opened:
// it is malformed or does not match its key. A retry would meet it again.
var errDeletedStateCorrupt = errors.New("the deleted state cannot be opened")

const (
	deletedStatePrefix = "automation-jobs/deleted/"
	deletedStateIndex  = "index"
	deletedStateGen    = "generation"
	generationBytes    = 16
	handleBytes        = 16
	dataKeyBytes       = 32
)

// sealDeletedState encrypts last under a fresh key, stores the key, and
// returns the handle that names it and the sealed text for the payload.
func (d *deletedStates) seal(ctx context.Context, last *entity.Entity) (handle, sealed string, err error) {
	plain, err := json.Marshal(last)
	if err != nil {
		return "", "", fmt.Errorf("automation job: encode deleted state: %w", err)
	}
	h := make([]byte, handleBytes)
	key := make([]byte, dataKeyBytes)
	_, _ = rand.Read(h)   // never fails (crypto/rand)
	_, _ = rand.Read(key) // never fails (crypto/rand)
	handle = hex.EncodeToString(h)
	aead, err := newAEAD(key)
	if err != nil {
		return "", "", err
	}
	// The generation is read before the key is stored, so a purge at any
	// point after this read makes the state unreadable (see open).
	gen, err := d.generation(ctx, last.ID)
	if err != nil {
		return "", "", err
	}
	out := slices.Clone(gen)
	nonce := make([]byte, aead.NonceSize())
	_, _ = rand.Read(nonce) // never fails (crypto/rand)
	out = append(out, nonce...)
	out = aead.Seal(out, nonce, plain, sealAAD(last.ID, last.Face, handle, gen))

	if err := d.kv.Put(ctx, deletedStateKey(last.ID, handle), key); err != nil {
		return "", "", fmt.Errorf("automation job: store deleted-state key: %w", err)
	}
	if err := d.updateIndex(ctx, last.ID, func(hs []string) []string { return append(hs, handle) }); err != nil {
		_ = d.kv.Delete(ctx, deletedStateKey(last.ID, handle))
		return "", "", err
	}
	return handle, base64.StdEncoding.EncodeToString(out), nil
}

// open decrypts the state that seal returned for (id, face, handle). It
// returns errDeletedStateErased when the key is gone.
func (d *deletedStates) open(
	ctx context.Context, id string, face entity.Face, handle, sealed string,
) (*entity.Entity, error) {
	if !validHandle(handle) {
		return nil, errors.New("automation job: malformed deleted-state handle")
	}
	raw, err := base64.StdEncoding.DecodeString(sealed)
	if err != nil {
		return nil, errDeletedStateCorrupt
	}
	key, err := d.kv.Get(ctx, deletedStateKey(id, handle))
	if err != nil {
		if os.IsNotExist(err) || errors.Is(err, fs.ErrNotExist) {
			return nil, errDeletedStateErased
		}
		return nil, fmt.Errorf("automation job: read deleted-state key: %w", err)
	}
	aead, err := newAEAD(key)
	if err != nil {
		return nil, err
	}
	if len(raw) < generationBytes+aead.NonceSize() {
		return nil, errDeletedStateCorrupt
	}
	sealedGen, rest := raw[:generationBytes], raw[generationBytes:]
	gen, err := d.generation(ctx, id)
	if err != nil {
		return nil, err
	}
	if !slices.Equal(sealedGen, gen) {
		return nil, errDeletedStateErased // purged since it was sealed
	}
	plain, err := aead.Open(nil, rest[:aead.NonceSize()], rest[aead.NonceSize():],
		sealAAD(id, face, handle, sealedGen))
	if err != nil {
		return nil, errDeletedStateCorrupt
	}
	var last entity.Entity
	dec := json.NewDecoder(strings.NewReader(string(plain)))
	dec.UseNumber()
	if err := dec.Decode(&last); err != nil {
		return nil, errDeletedStateCorrupt
	}
	for k, v := range last.Properties {
		last.Properties[k] = jsonNumbers(v)
	}
	return &last, nil
}

// erase removes the key for handle, which makes its sealed state unreadable.
func (d *deletedStates) erase(ctx context.Context, id, handle string) error {
	if !validHandle(handle) {
		return nil
	}
	if err := d.kv.Delete(ctx, deletedStateKey(id, handle)); err != nil {
		return fmt.Errorf("automation job: erase deleted-state key: %w", err)
	}
	return d.updateIndex(ctx, id, func(hs []string) []string {
		return slices.DeleteFunc(hs, func(h string) bool { return h == handle })
	})
}

// purge erases the keys the index lists for id. With newGeneration it first
// writes a new generation, which makes every state sealed for id so far
// unreadable, listed or not. The generation is a record that stays, so a
// caller passes newGeneration only when the schema has `on.deleted`
// automations: without one, no queued `on.deleted` job can run anyway.
func (d *deletedStates) purge(ctx context.Context, id string, newGeneration bool) error {
	if newGeneration {
		gen := make([]byte, generationBytes)
		_, _ = rand.Read(gen) // never fails (crypto/rand)
		if err := d.kv.Put(ctx, deletedStateDir(id)+deletedStateGen, gen); err != nil {
			return fmt.Errorf("automation job: record purge: %w", err)
		}
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	handles, err := d.readIndex(ctx, id)
	if err != nil {
		return err
	}
	var errs []error
	for _, h := range handles {
		if err := d.kv.Delete(ctx, deletedStateKey(id, h)); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("automation job: erase deleted-state keys: %w", errors.Join(errs...))
	}
	return d.kv.Delete(ctx, deletedStateDir(id)+deletedStateIndex)
}

// generation returns id's current generation: zeros until its first purge.
// A record of another length reads as itself, so it matches no sealed state
// and fails closed.
func (d *deletedStates) generation(ctx context.Context, id string) ([]byte, error) {
	b, err := d.kv.Get(ctx, deletedStateDir(id)+deletedStateGen)
	if err != nil {
		if os.IsNotExist(err) || errors.Is(err, fs.ErrNotExist) {
			return make([]byte, generationBytes), nil
		}
		return nil, fmt.Errorf("automation job: read purge generation: %w", err)
	}
	return b, nil
}

func (d *deletedStates) updateIndex(ctx context.Context, id string, change func([]string) []string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	handles, err := d.readIndex(ctx, id)
	if err != nil {
		return err
	}
	handles = change(handles)
	key := deletedStateDir(id) + deletedStateIndex
	if len(handles) == 0 {
		return d.kv.Delete(ctx, key)
	}
	if err := d.kv.Put(ctx, key, []byte(strings.Join(handles, "\n"))); err != nil {
		return fmt.Errorf("automation job: write deleted-state index: %w", err)
	}
	return nil
}

// readIndex returns the valid handles in id's index. Callers hold d.mu.
func (d *deletedStates) readIndex(ctx context.Context, id string) ([]string, error) {
	b, err := d.kv.Get(ctx, deletedStateDir(id)+deletedStateIndex)
	if err != nil {
		if os.IsNotExist(err) || errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("automation job: read deleted-state index: %w", err)
	}
	var out []string
	for h := range strings.SplitSeq(string(b), "\n") {
		if validHandle(h) && !slices.Contains(out, h) {
			out = append(out, h)
		}
	}
	return out, nil
}

// PurgeDeletedJobStates makes the state of every pending `on.deleted` job
// for the entity id unreadable, so those jobs end without running. `rela
// history-purge` calls it; the soft-delete purge reaches the same erase
// through [entitymanager.PurgeNotifier].
func PurgeDeletedJobStates(ctx context.Context, kv state.KV, meta *metamodel.Metamodel, id string) error {
	if kv == nil || meta == nil {
		return errors.New("appbuild: purge deleted job states: nil state store or metamodel")
	}
	return (&deletedStates{kv: kv}).purge(ctx, id, hasDeletedTriggers(meta))
}

// deletedStateDir is the KV directory, with a trailing slash, of id's keys.
// The id is hashed: it is user-chosen text and need not be a valid key.
func deletedStateDir(id string) string {
	sum := sha256.Sum256([]byte(id))
	return deletedStatePrefix + hex.EncodeToString(sum[:]) + "/"
}

func deletedStateKey(id, handle string) string { return deletedStateDir(id) + handle }

// validHandle reports whether h is a handle seal made: it reaches a KV key
// from a payload, so nothing else may.
func validHandle(h string) bool {
	if len(h) != 2*handleBytes {
		return false
	}
	_, err := hex.DecodeString(h)
	return err == nil
}

// sealAAD binds the ciphertext to the face, handle and generation it was
// sealed with, so a payload cannot pair one job's state with another's key,
// or claim the current generation to pass a purge.
func sealAAD(id string, face entity.Face, handle string, gen []byte) []byte {
	return append([]byte(entity.FormatStateRef(id, face)+"\x00"+handle+"\x00"), gen...)
}

func newAEAD(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("automation job: deleted-state key: %w", err)
	}
	return cipher.NewGCM(block)
}
