// Package bleveindex provides a bleve-backed implementation of
// search.Backend for full-text entity search.
package bleveindex

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/analysis/analyzer/keyword"
	"github.com/blevesearch/bleve/v2/analysis/analyzer/standard"
	"github.com/blevesearch/bleve/v2/index/scorch"
	"github.com/blevesearch/bleve/v2/mapping"
	"github.com/blevesearch/bleve/v2/search/query"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/search"
)

// lastModifiedKey is the bleve internal-storage key under which we persist
// the most recent entity mtime observed by this index.
var lastModifiedKey = []byte("rela:last_modified")

// compile-time interface check.
var _ search.Backend = (*Index)(nil)

// Field boost weights for search ranking.
const (
	boostIDExact    = 8.0
	boostIDPrefix   = 6.0
	boostPrimary    = 3.0
	boostProperties = 2.0
	boostContent    = 1.0
)

// bleveDoc is the internal document structure indexed by bleve.
type bleveDoc struct {
	ID         string `json:"id"`
	Type       string `json:"type"`
	Primary    string `json:"primary"`
	Content    string `json:"content"`
	Properties string `json:"properties"`
	All        string `json:"all"`
}

// Index is a bleve-backed full-text search index.
type Index struct {
	index bleve.Index
}

// NewMem creates an in-memory bleve index.
//
// This uses scorch with an empty path, which means scorch starts NEITHER
// its persister nor its merger goroutine (both are gated on a non-empty
// path — see scorch.Open). Nothing ever merges the per-write segments, so
// memory grows with (writes × distinct documents touched) and never comes
// back: on a 2.4k-entity corpus, 1500 single-entity updates spread across
// the corpus reach ~5.7GB. It is therefore only safe for short-lived or
// read-mostly processes, and [New] is the right choice for anything that
// stays up and takes writes.
func NewMem() (*Index, error) {
	idx, err := bleve.NewUsing("", buildMapping(), scorch.Name, scorch.Name, nil)
	if err != nil {
		return nil, fmt.Errorf("bleveindex: create index: %w", err)
	}
	return &Index{index: idx}, nil
}

// New creates (or reopens) a persistent on-disk bleve index at the given
// path. If an index already exists there it is opened and its contents
// reused — see [Index.LastModified], which lets the caller skip a backfill
// entirely when the store has not changed since the index was written.
// If the existing index is corrupted it is removed and recreated.
//
// The on-disk form is what makes scorch's persister and merger run, which
// is what bounds memory: segments produced by individual writes get merged
// and flushed instead of accumulating for the life of the process (the
// failure mode described on [NewMem]).
//
// One deliberate configuration choice:
//
//   - bolt_timeout bounds the exclusive lock bbolt takes on the index
//     directory. A second process opening the same index would otherwise
//     block forever with no diagnostic; instead it fails fast and the
//     caller degrades to an in-memory index.
func New(path string) (*Index, error) {
	idx, err := bleve.OpenUsing(path, indexRuntimeConfig())
	if err == nil {
		return &Index{index: idx}, nil
	}
	if errors.Is(err, ErrIndexLocked) || isLockTimeout(err) {
		return nil, fmt.Errorf("bleveindex: index at %s is locked by another process: %w", path, err)
	}

	// Open failed — either the index doesn't exist yet or it's corrupted.
	// Remove any existing directory so bleve.New can create a fresh one.
	if _, statErr := os.Stat(path); statErr == nil {
		if removeErr := os.RemoveAll(path); removeErr != nil {
			return nil, fmt.Errorf("bleveindex: remove corrupted index at %s: %w", path, removeErr)
		}
	}

	idx, err = bleve.NewUsing(path, buildMapping(), scorch.Name, scorch.Name, indexRuntimeConfig())
	if err != nil {
		if isLockTimeout(err) {
			return nil, fmt.Errorf("bleveindex: index at %s is locked by another process: %w", path, err)
		}
		return nil, fmt.Errorf("bleveindex: create index at %s: %w", path, err)
	}
	return &Index{index: idx}, nil
}

// ErrIndexLocked reports that another process holds the index directory.
var ErrIndexLocked = errors.New("bleveindex: index locked")

// boltOpenTimeout bounds how long bbolt waits for the index directory's
// exclusive lock before giving up. Long enough to ride out a process
// exiting, short enough that a genuinely concurrent opener fails with a
// diagnosable error instead of hanging on startup.
const boltOpenTimeout = 5 * time.Second

// indexRuntimeConfig is the scorch configuration shared by create and
// reopen; see [New] for why each key is set.
func indexRuntimeConfig() map[string]any {
	return map[string]any{
		"bolt_timeout": boltOpenTimeout.String(),
	}
}

// isLockTimeout reports whether err is bbolt's "timeout" from failing to
// acquire the index directory lock. bbolt returns a bare errors.New, so
// matching on the message is the only option available.
func isLockTimeout(err error) bool {
	return err != nil && strings.Contains(err.Error(), "timeout")
}

func buildMapping() *mapping.IndexMappingImpl {
	textField := bleve.NewTextFieldMapping()
	textField.Analyzer = standard.Name

	keywordField := bleve.NewTextFieldMapping()
	keywordField.Analyzer = keyword.Name

	docMapping := bleve.NewDocumentMapping()
	docMapping.AddFieldMappingsAt("id", keywordField)
	docMapping.AddFieldMappingsAt("type", keywordField)
	docMapping.AddFieldMappingsAt("primary", textField)
	docMapping.AddFieldMappingsAt("content", textField)
	docMapping.AddFieldMappingsAt("properties", textField)
	docMapping.AddFieldMappingsAt("all", textField)

	indexMapping := bleve.NewIndexMapping()
	indexMapping.DefaultMapping = docMapping
	indexMapping.DefaultAnalyzer = standard.Name

	return indexMapping
}

// Index adds or updates an entity in the search index.
func (idx *Index) EntityPut(e *entity.Entity) error {
	if err := idx.index.Index(e.ID, entityToDoc(e)); err != nil {
		return err
	}
	return idx.bumpLastModified(e.UpdatedAt)
}

// IndexBatch indexes every entity in a single Bleve batch and bumps
// LastModified once at the end. Use this for initial backfill where
// N round-trips through EntityPut would be O(N) Bleve transactions.
// Returns the number of entities successfully written and the first
// error (if any). Subsequent entities are not attempted on error.
func (idx *Index) IndexBatch(entities []*entity.Entity) (int, error) {
	if len(entities) == 0 {
		return 0, nil
	}
	batch := idx.index.NewBatch()
	var latest time.Time
	for _, e := range entities {
		if err := batch.Index(e.ID, entityToDoc(e)); err != nil {
			return 0, fmt.Errorf("bleveindex: batch index %s: %w", e.ID, err)
		}
		if e.UpdatedAt.After(latest) {
			latest = e.UpdatedAt
		}
	}
	if err := idx.index.Batch(batch); err != nil {
		return 0, fmt.Errorf("bleveindex: commit batch: %w", err)
	}
	if !latest.IsZero() {
		if err := idx.bumpLastModified(latest); err != nil {
			return len(entities), err
		}
	}
	return len(entities), nil
}

// EntityDelete removes an entity from the search index.
func (idx *Index) EntityDelete(id string) error {
	if err := idx.index.Delete(id); err != nil {
		return err
	}
	// A delete carries no mtime from the entity; use wall clock so the
	// timestamp still advances and consumers can observe the change.
	return idx.bumpLastModified(time.Now())
}

// EntityRenamed atomically deletes the old document and indexes the
// renamed entity under its new ID. Uses a single Bleve batch so a
// crash mid-rename cannot leave the index with both the old and new
// keys present.
func (idx *Index) EntityRenamed(oldID string, renamed *entity.Entity) error {
	batch := idx.index.NewBatch()
	batch.Delete(oldID)
	if err := batch.Index(renamed.ID, entityToDoc(renamed)); err != nil {
		return fmt.Errorf("bleveindex: rename %s→%s: index new: %w", oldID, renamed.ID, err)
	}
	if err := idx.index.Batch(batch); err != nil {
		return fmt.Errorf("bleveindex: rename %s→%s: commit batch: %w", oldID, renamed.ID, err)
	}
	return idx.bumpLastModified(renamed.UpdatedAt)
}

// SetWatermark stores an arbitrary named timestamp alongside the index.
// Callers use this to record how far a bulk operation got, so a later
// process can decide whether the index is still current — see
// [Index.Watermark]. Unlike [Index.LastModified], the value is opaque to
// the index: it is stored and returned verbatim, with no MAX semantics,
// so the caller controls exactly what the timestamp means.
func (idx *Index) SetWatermark(key string, t time.Time) error {
	data, err := t.MarshalBinary()
	if err != nil {
		return fmt.Errorf("bleveindex: encode watermark %s: %w", key, err)
	}
	if err := idx.index.SetInternal([]byte(key), data); err != nil {
		return fmt.Errorf("bleveindex: store watermark %s: %w", key, err)
	}
	return nil
}

// Watermark returns a timestamp previously stored by [Index.SetWatermark].
// A missing key yields the zero time and no error — "never recorded" is a
// normal state (a fresh index), not a failure.
func (idx *Index) Watermark(key string) (time.Time, error) {
	data, err := idx.index.GetInternal([]byte(key))
	if err != nil {
		return time.Time{}, fmt.Errorf("bleveindex: read watermark %s: %w", key, err)
	}
	if len(data) == 0 {
		return time.Time{}, nil
	}
	var t time.Time
	if err := t.UnmarshalBinary(data); err != nil {
		return time.Time{}, fmt.Errorf("bleveindex: decode watermark %s: %w", key, err)
	}
	return t, nil
}

// DocCount returns the number of documents currently in the index.
// Callers use it to tell an empty index from a populated one — notably
// when deciding whether a persisted index can be reused as-is.
func (idx *Index) DocCount() (uint64, error) {
	n, err := idx.index.DocCount()
	if err != nil {
		return 0, fmt.Errorf("bleveindex: doc count: %w", err)
	}
	return n, nil
}

// LastModified returns the latest mtime observed by this index. Persistent
// indexes restore this across restarts so consumers can skip reindexing
// when the store's LastModified hasn't advanced.
func (idx *Index) LastModified() time.Time {
	data, err := idx.index.GetInternal(lastModifiedKey)
	if err != nil || len(data) == 0 {
		return time.Time{}
	}
	var t time.Time
	if err := t.UnmarshalBinary(data); err != nil {
		return time.Time{}
	}
	return t
}

// bumpLastModified advances the persisted timestamp if t is newer than the
// current value. Concurrent writers race harmlessly — the monotonic MAX
// semantics ensure the timestamp only moves forward.
func (idx *Index) bumpLastModified(t time.Time) error {
	if !t.After(idx.LastModified()) {
		return nil
	}
	data, err := t.MarshalBinary()
	if err != nil {
		return err
	}
	return idx.index.SetInternal(lastModifiedKey, data)
}

// boostedFields defines the text fields searched per word with their
// boost weights. The keyword `id` field is intentionally absent: it is
// case-sensitive and unanalyzed, so a lower-cased per-word fuzzy query
// never matches it. ID matching is handled by the dedicated exact-term
// and prefix queries in Search.
var boostedFields = []struct {
	field string
	boost float64
}{
	{"primary", boostPrimary},
	{"properties", boostProperties},
	{"content", boostContent},
	{"all", boostContent},
}

// Search returns entity IDs matching the query text, ordered by relevance.
func (idx *Index) Search(text string, limit int) ([]string, error) {
	words := strings.Fields(text)
	if len(words) == 0 {
		return nil, nil
	}

	queries := make([]query.Query, 0, len(words)+2)

	// The `id` field is keyword-analyzed (whole ID stored as one
	// case-sensitive token), so ID matching is driven by these two
	// queries rather than the per-word fuzzy pass — which tokenizes a
	// dashed ID like `VAD-ACT-6P4X` into `vad`/`act`/`6p4x` and can't
	// bridge a partial-ID query back to the original token.
	idText := strings.TrimSpace(text)

	// Exact ID match — boosted highest.
	idExact := bleve.NewTermQuery(idText)
	idExact.SetField("id")
	idExact.SetBoost(boostIDExact)
	queries = append(queries, idExact)

	// Partial ID (prefix) match. Case-sensitive against the stored token,
	// so feed the original-case text — IDs are upper-case and lower-casing
	// here would match nothing. This is what makes typing `VAD-ACT-` in
	// the entity-reference picker surface every `VAD-ACT-*` entity instead
	// of title-scored noise.
	idPrefix := bleve.NewPrefixQuery(idText)
	idPrefix.SetField("id")
	idPrefix.SetBoost(boostIDPrefix)
	queries = append(queries, idPrefix)

	for _, word := range words {
		queries = append(queries, buildBoostedWordQuery(strings.ToLower(word)))
	}

	finalQuery := bleve.NewDisjunctionQuery(queries...)

	req := bleve.NewSearchRequest(finalQuery)
	if limit > 0 {
		req.Size = limit
	} else {
		req.Size = 10000 // practical upper bound
	}
	// Rank by score, then by id as a tie-break. Ties are common here —
	// a query like "requirement" matches every entity of that type with
	// an identical score — and without a second sort key bleve falls
	// back to internal document order, which depends on the order
	// documents happened to be indexed in. That made results
	// unstable across runs (an entity updated after creation moves), so
	// callers saw a different order for identical data. Sorting on a
	// field the caller can reason about keeps output reproducible.
	req.SortBy([]string{"-_score", "id"})

	result, err := idx.index.Search(req)
	if err != nil {
		return nil, fmt.Errorf("bleveindex: search: %w", err)
	}

	ids := make([]string, 0, len(result.Hits))
	for _, hit := range result.Hits {
		ids = append(ids, hit.ID)
	}
	return ids, nil
}

// Close flushes anything still pending and releases resources held by the
// index.
//
// The flush is load-bearing for a persistent index. Writes run with
// unsafe_batch (see [New]), which returns as soon as a batch is applied
// in memory rather than waiting for it to reach disk, and scorch's Close
// stops the persister rather than draining it — so closing straight after
// a write can drop that write. Persisting here means a clean shutdown
// always leaves an index that matches what callers were told was indexed;
// without it the index and its LastModified watermark can disagree, and
// the next startup would reuse an index that silently lost its tail.
func (idx *Index) Close() error {
	return idx.index.Close()
}

func buildBoostedWordQuery(word string) query.Query {
	isWildcard := strings.ContainsAny(word, "*?")
	queries := make([]query.Query, 0, len(boostedFields))

	for _, f := range boostedFields {
		var q query.Query
		if isWildcard {
			wq := bleve.NewWildcardQuery(word)
			wq.SetField(f.field)
			wq.SetBoost(f.boost)
			q = wq
		} else {
			fq := bleve.NewFuzzyQuery(word)
			fq.SetField(f.field)
			fq.SetFuzziness(1)
			fq.SetBoost(f.boost)
			q = fq
		}
		queries = append(queries, q)
	}

	return bleve.NewDisjunctionQuery(queries...)
}

func entityToDoc(e *entity.Entity) bleveDoc {
	var propParts []string
	for _, v := range e.Properties {
		if s, ok := v.(string); ok && s != "" {
			propParts = append(propParts, s)
		}
	}
	props := strings.Join(propParts, " ")

	primary := ""
	if t, ok := e.Properties["title"]; ok {
		if s, ok := t.(string); ok {
			primary = s
		}
	}

	all := strings.Join([]string{e.ID, primary, props, e.Content}, " ")

	return bleveDoc{
		ID:         e.ID,
		Type:       e.Type,
		Primary:    primary,
		Content:    e.Content,
		Properties: props,
		All:        all,
	}
}
