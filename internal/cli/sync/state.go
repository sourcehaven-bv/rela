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

// Baseline is what the replica last agreed on with the primary for one record.
// It holds TWO independent tokens because, since the replica reads and writes
// through the authorized /api/v1 API (TKT-8P1TM7, "sync is a fancy browser"),
// the server's conflict token and the client's own change-detector are no
// longer the same value:
//
//   - Server is the OPAQUE token the primary returned (its ETag). The replica
//     never parses it: it is echoed back as If-Match on the next write and
//     compared only for equality to detect "the primary moved." The primary's
//     /api/v1 ETag (computeEntityETag) is truncated + relation-folded, a
//     different value space from canonical.HashEntity — so the replica cannot
//     recompute it locally and must treat it as opaque.
//   - Local is the replica's OWN canonical hash (canonical.HashEntity /
//     HashRelation) of the working record at last sync. It is recomputed
//     locally to answer "have I edited this since?" and is NEVER sent to the
//     primary. It is the change-detector, not the conflict token.
//
// Dirty  = canonical(working) != Local.        (I edited it.)
// Moved  = server_now_ETag    != Server.       (the primary edited it.)
// Conflict = Dirty AND Moved.
type Baseline struct {
	Server string `json:"server"` // opaque primary ETag (If-Match + equality)
	Local  string `json:"local"`  // canonical hash of the working record (mine)
	// Type is the record's entity/relation type. It is recorded so a PUSH of a
	// LOCAL DELETION can still resolve the /api/v1 route plural — the working
	// record is gone by then, so its type can no longer be read from the store.
	// For a relation it is the relation type (the FROM plural is resolved from
	// the FROM endpoint's own baseline). Not secret (config is public).
	Type string `json:"type,omitempty"`
}

// State is the persisted sync index. Records maps a record key (see RecordKey)
// to the Baseline the client and server last agreed on for that record.
// Cursor is an opaque, server-minted manifest watermark: the client stores and
// echoes it verbatim and never parses it (the server may change its encoding).
type State struct {
	Records map[string]Baseline `json:"records"`
	Cursor  string              `json:"cursor"`
}

// newState returns an empty, ready-to-use State.
func newState() *State {
	return &State{Records: map[string]Baseline{}}
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
	s, err := unmarshalState(data)
	if err != nil {
		return nil, fmt.Errorf("parse sync state %s: %w (delete it to re-bootstrap from scratch)", path, err)
	}
	return s, nil
}

// unmarshalState decodes the index, tolerating the LEGACY single-string format
// (`"records": {"k": "<hash>"}`) written before TKT-8P1TM7 split the baseline
// into {server, local}. In the legacy world the one hash was the shared
// canonical hash. On migration we set Local to it (it IS the canonical hash of
// the last-agreed record, so change-detection stays correct) but leave Server
// EMPTY: the old value was never a /api/v1 ETag, so the replica must re-fetch to
// learn the primary's real conflict token. An empty Server just means the next
// pull re-baselines it — lossless, no spurious conflict.
func unmarshalState(data []byte) (*State, error) {
	// Try the current shape first.
	var s State
	if err := json.Unmarshal(data, &s); err == nil {
		if s.Records == nil {
			s.Records = map[string]Baseline{}
		}
		return &s, nil
	}
	// Fall back to the legacy single-string shape and migrate.
	var legacy struct {
		Records map[string]string `json:"records"`
		Cursor  string            `json:"cursor"`
	}
	if err := json.Unmarshal(data, &legacy); err != nil {
		return nil, err
	}
	s = State{Records: make(map[string]Baseline, len(legacy.Records)), Cursor: legacy.Cursor}
	for k, h := range legacy.Records {
		s.Records[k] = Baseline{Local: h} // Server left empty → re-fetch to re-baseline
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

// Set records the agreed baseline for a key: the primary's opaque ETag
// (server) and the replica's own canonical hash of the working record (local).
// Delete removes a key (used when a record is deleted on both ends). Both keep
// the in-memory index in step with the wire so a later Save persists the
// converged state.
func (s *State) Set(key, server, local, typ string) {
	s.Records[key] = Baseline{Server: server, Local: local, Type: typ}
}
func (s *State) Delete(key string) { delete(s.Records, key) }

// Baseline returns the indexed baseline for a key and whether it is present.
func (s *State) Baseline(key string) (Baseline, bool) {
	b, ok := s.Records[key]
	return b, ok
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
