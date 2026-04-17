// Package bleveindex provides a bleve-backed implementation of
// fsstore.SearchIndex for full-text entity search.
package bleveindex

import (
	"fmt"
	"os"
	"strings"

	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/analysis/analyzer/keyword"
	"github.com/blevesearch/bleve/v2/analysis/analyzer/standard"
	"github.com/blevesearch/bleve/v2/mapping"
	"github.com/blevesearch/bleve/v2/search/query"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store/fsstore"
)

// compile-time interface check.
var _ fsstore.SearchIndex = (*Index)(nil)

// Field boost weights for search ranking.
const (
	boostID         = 5.0
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
func NewMem() (*Index, error) {
	idx, err := bleve.NewMemOnly(buildMapping())
	if err != nil {
		return nil, fmt.Errorf("bleveindex: create index: %w", err)
	}
	return &Index{index: idx}, nil
}

// New creates a persistent on-disk bleve index at the given path.
// If an index already exists at that path, it is opened instead.
// If the existing index is corrupted, it is removed and recreated.
// The caller (fsstore) repopulates the index via populateSearchIndex.
func New(path string) (*Index, error) {
	idx, err := bleve.Open(path)
	if err == nil {
		return &Index{index: idx}, nil
	}

	// Open failed — either the index doesn't exist yet or it's corrupted.
	// Remove any existing directory so bleve.New can create a fresh one.
	if _, statErr := os.Stat(path); statErr == nil {
		if removeErr := os.RemoveAll(path); removeErr != nil {
			return nil, fmt.Errorf("bleveindex: remove corrupted index at %s: %w", path, removeErr)
		}
	}

	idx, err = bleve.New(path, buildMapping())
	if err != nil {
		return nil, fmt.Errorf("bleveindex: create index at %s: %w", path, err)
	}
	return &Index{index: idx}, nil
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
func (idx *Index) Index(e *entity.Entity) error {
	return idx.index.Index(e.ID, entityToDoc(e))
}

// Remove deletes an entity from the search index.
func (idx *Index) Remove(id string) error {
	return idx.index.Delete(id)
}

// boostedFields defines the fields to search with their boost weights.
var boostedFields = []struct {
	field string
	boost float64
}{
	{"id", boostID},
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

	queries := make([]query.Query, 0, len(words)+1)

	// Exact ID match (keyword field) — boosted highest.
	idQuery := bleve.NewTermQuery(text)
	idQuery.SetField("id")
	idQuery.SetBoost(boostID)
	queries = append(queries, idQuery)

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

// Persistent returns true — bleve indexes are stored on disk.
func (idx *Index) Persistent() bool { return true }

// Close releases resources held by the index.
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
