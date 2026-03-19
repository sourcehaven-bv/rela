// Package search provides full-text search using Bleve.
package search

import (
	"fmt"
	"sort"
	"strings"

	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/analysis/analyzer/keyword"
	"github.com/blevesearch/bleve/v2/analysis/analyzer/standard"
	"github.com/blevesearch/bleve/v2/mapping"
	"github.com/blevesearch/bleve/v2/search/query"

	"github.com/Sourcehaven-BV/rela/internal/model"
)

// Index wraps a Bleve index for entity search.
type Index struct {
	index bleve.Index
}

// entityDoc is the document structure indexed by Bleve.
type entityDoc struct {
	ID          string `json:"id"`
	Type        string `json:"type"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Content     string `json:"content"`
	Properties  string `json:"properties"` // all property values joined
	All         string `json:"all"`        // everything combined for simple queries
}

// Result represents a search result with score.
type Result struct {
	ID    string
	Score float64
}

// NewIndex creates an in-memory Bleve index.
func NewIndex() (*Index, error) {
	idxMapping := buildMapping()
	index, err := bleve.NewMemOnly(idxMapping)
	if err != nil {
		return nil, fmt.Errorf("failed to create search index: %w", err)
	}
	return &Index{index: index}, nil
}

// buildMapping creates the index mapping with field-specific analyzers.
func buildMapping() *mapping.IndexMappingImpl {
	// Text field with standard analyzer (tokenization, lowercase, etc.)
	textFieldMapping := bleve.NewTextFieldMapping()
	textFieldMapping.Analyzer = standard.Name

	// Keyword field for exact matching (ID, type)
	keywordFieldMapping := bleve.NewTextFieldMapping()
	keywordFieldMapping.Analyzer = keyword.Name

	// Document mapping
	docMapping := bleve.NewDocumentMapping()
	docMapping.AddFieldMappingsAt("id", keywordFieldMapping)
	docMapping.AddFieldMappingsAt("type", keywordFieldMapping)
	docMapping.AddFieldMappingsAt("title", textFieldMapping)
	docMapping.AddFieldMappingsAt("description", textFieldMapping)
	docMapping.AddFieldMappingsAt("content", textFieldMapping)
	docMapping.AddFieldMappingsAt("properties", textFieldMapping)
	docMapping.AddFieldMappingsAt("all", textFieldMapping)

	indexMapping := bleve.NewIndexMapping()
	indexMapping.DefaultMapping = docMapping
	indexMapping.DefaultAnalyzer = standard.Name

	return indexMapping
}

// IndexEntity adds or updates an entity in the index.
func (idx *Index) IndexEntity(e *model.Entity) error {
	doc := entityToDoc(e)
	return idx.index.Index(e.ID, doc)
}

// IndexAll indexes multiple entities (for initial load).
func (idx *Index) IndexAll(entities []*model.Entity) error {
	batch := idx.index.NewBatch()
	for _, e := range entities {
		doc := entityToDoc(e)
		if err := batch.Index(e.ID, doc); err != nil {
			return fmt.Errorf("failed to batch index %s: %w", e.ID, err)
		}
	}
	return idx.index.Batch(batch)
}

// RemoveEntity removes an entity from the index.
func (idx *Index) RemoveEntity(id string) error {
	return idx.index.Delete(id)
}

// Search performs a search query and returns scored results.
// words are OR'd together with fuzzy matching.
// phrases must all match exactly (AND logic).
func (idx *Index) Search(words, phrases []string, limit int) ([]Result, error) {
	if len(words) == 0 && len(phrases) == 0 {
		return nil, nil
	}

	var queries []query.Query

	// Add fuzzy queries for each word (OR logic within words)
	for _, word := range words {
		word = strings.ToLower(word)

		// Check for wildcard patterns
		if strings.ContainsAny(word, "*?") {
			wq := bleve.NewWildcardQuery(word)
			wq.SetField("all")
			queries = append(queries, wq)
		} else {
			// Fuzzy match with edit distance 1
			fq := bleve.NewFuzzyQuery(word)
			fq.SetFuzziness(1)
			fq.SetField("all")
			queries = append(queries, fq)
		}
	}

	// Add phrase queries (must all match - AND logic)
	phraseQueries := make([]query.Query, 0, len(phrases))
	for _, phrase := range phrases {
		pq := bleve.NewMatchPhraseQuery(phrase)
		pq.SetField("all")
		phraseQueries = append(phraseQueries, pq)
	}

	var finalQuery query.Query

	switch {
	case len(queries) > 0 && len(phraseQueries) > 0:
		// Words (OR) AND phrases (all must match)
		wordQuery := bleve.NewDisjunctionQuery(queries...)
		phraseQuery := bleve.NewConjunctionQuery(phraseQueries...)
		finalQuery = bleve.NewConjunctionQuery(wordQuery, phraseQuery)
	case len(queries) > 0:
		// Just words (OR)
		finalQuery = bleve.NewDisjunctionQuery(queries...)
	default:
		// Just phrases (AND)
		finalQuery = bleve.NewConjunctionQuery(phraseQueries...)
	}

	searchRequest := bleve.NewSearchRequest(finalQuery)
	searchRequest.Size = limit

	searchResult, err := idx.index.Search(searchRequest)
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	results := make([]Result, 0, len(searchResult.Hits))
	for _, hit := range searchResult.Hits {
		results = append(results, Result{
			ID:    hit.ID,
			Score: hit.Score,
		})
	}

	return results, nil
}

// SearchSimple performs a simple text search (convenience method).
func (idx *Index) SearchSimple(queryStr string, limit int) ([]Result, error) {
	words := strings.Fields(queryStr)
	return idx.Search(words, nil, limit)
}

// Close closes the index.
func (idx *Index) Close() error {
	return idx.index.Close()
}

// entityToDoc converts an entity to an indexable document.
func entityToDoc(e *model.Entity) entityDoc {
	// Sort property keys for deterministic output.
	keys := make([]string, 0, len(e.Properties))
	for k := range e.Properties {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var propParts []string
	for _, k := range keys {
		v := e.Properties[k]
		switch val := v.(type) {
		case string:
			propParts = append(propParts, val)
		case []string:
			propParts = append(propParts, val...)
		case []interface{}:
			for _, item := range val {
				if s, ok := item.(string); ok {
					propParts = append(propParts, s)
				}
			}
		default:
			propParts = append(propParts, fmt.Sprintf("%v", v))
		}
	}

	properties := strings.Join(propParts, " ")
	all := strings.Join([]string{
		e.ID,
		e.Title(),
		e.Description(),
		e.Content,
		properties,
	}, " ")

	return entityDoc{
		ID:          e.ID,
		Type:        e.Type,
		Title:       e.Title(),
		Description: e.Description(),
		Content:     e.Content,
		Properties:  properties,
		All:         all,
	}
}
