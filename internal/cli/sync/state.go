// Package sync implements the rela CLI sync client: a hash-indexed,
// topologically-ordered push/pull between a local fsstore project and a remote
// pgstore-backed rela-server's /api/sync/ API (FEAT-NJ9FEN, TKT-T4H4YK).
//
// The client keeps a sync-state index (.rela/sync-state.json) mapping each
// record key to the content hash it last agreed on with the server, plus an
// opaque cursor the server mints for incremental manifests. Dirty detection is
// purely local: recompute the canonical hash of each working record and compare
// to the index. Conflict resolution is deliberately dumb — a divergence halts
// that one record with a clear report, and the operator resolves it with
// --force (local-wins on push, remote-wins on pull).
package sync

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/storage"
)

// stateFileName is the sync index, stored in the project's .rela cache dir.
const stateFileName = "sync-state.json"

// State is the persisted sync index. Records maps a record key (see RecordKey)
// to the content hash the client and server last agreed on for that record.
// Cursor is an opaque, server-minted manifest watermark: the client stores and
// echoes it verbatim and never parses it (the server may change its encoding).
type State struct {
	Records map[string]string `json:"records"`
	Cursor  string            `json:"cursor"`
}

// newState returns an empty, ready-to-use State.
func newState() *State {
	return &State{Records: map[string]string{}}
}

// EntityKey is the index/manifest key for an entity: its id.
func EntityKey(id string) string { return id }

// RelationKey is the index/manifest key for a relation: "from/type/to",
// matching the server's manifestKey and record-path encoding. A slash join is
// unambiguous because no key segment may contain a slash (the server's
// validIDSegment rejects them).
func RelationKey(from, relType, to string) string {
	return from + "/" + relType + "/" + to
}

// LoadState reads the sync index from .rela/sync-state.json under cacheDir.
// A missing file is not an error — it yields a fresh, empty index (the
// first-sync case). A present-but-corrupt file IS an error: silently discarding
// it would re-push every local record as a blind create, so the operator must
// see and resolve the corruption.
func LoadState(fs storage.FS, cacheDir string) (*State, error) {
	path := filepath.Join(cacheDir, stateFileName)
	data, err := fs.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return newState(), nil
		}
		return nil, fmt.Errorf("read sync state %s: %w", path, err)
	}
	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parse sync state %s: %w (delete it to re-bootstrap from scratch)", path, err)
	}
	if s.Records == nil {
		s.Records = map[string]string{}
	}
	return &s, nil
}

// Save writes the index back atomically (temp file + rename) so a crash mid-write
// can never leave a truncated index that would be read as "everything is dirty".
func (s *State) Save(fs storage.FS, cacheDir string) error {
	if err := fs.MkdirAll(cacheDir, 0o755); err != nil {
		return fmt.Errorf("create cache dir %s: %w", cacheDir, err)
	}
	// Marshal with sorted keys (encoding/json sorts map keys) for a stable,
	// diff-friendly file.
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal sync state: %w", err)
	}
	final := filepath.Join(cacheDir, stateFileName)
	tmp := final + ".tmp"
	if err := fs.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write sync state temp: %w", err)
	}
	if err := fs.Rename(tmp, final); err != nil {
		return fmt.Errorf("commit sync state: %w", err)
	}
	return nil
}

// Set records the agreed hash for a key. Delete removes a key (used when a
// record is deleted on both ends). Both keep the in-memory index in step with
// the wire so a later Save persists the converged state.
func (s *State) Set(key, hash string) { s.Records[key] = hash }
func (s *State) Delete(key string)    { delete(s.Records, key) }

// Hash returns the indexed hash for a key and whether it is present.
func (s *State) Hash(key string) (string, bool) {
	h, ok := s.Records[key]
	return h, ok
}

// Keys returns the indexed keys sorted, for deterministic iteration in reports
// and tests.
func (s *State) Keys() []string {
	keys := make([]string, 0, len(s.Records))
	for k := range s.Records {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// LocalRecord is one working-copy record (entity or relation) reduced to what
// the sync diff needs: its wire key, its kind, and its current canonical hash.
type LocalRecord struct {
	Key      string
	Kind     Kind
	Hash     string
	Entity   *entity.Entity   // set when Kind == KindEntity
	Relation *entity.Relation // set when Kind == KindRelation
}

// Kind distinguishes entities from relations across the diff and the wire.
type Kind int

const (
	// KindEntity is an entity record (wire kind "entities" / manifest "e").
	KindEntity Kind = iota
	// KindRelation is a relation record (wire kind "relations" / manifest "r").
	KindRelation
)
