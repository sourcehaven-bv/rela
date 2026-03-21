// Package search provides full-text search using Bleve.
// It is decoupled from the domain model - callers provide Document structs.
package search

import (
	"fmt"
	"strings"

	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/analysis/analyzer/keyword"
	"github.com/blevesearch/bleve/v2/analysis/analyzer/standard"
	"github.com/blevesearch/bleve/v2/mapping"
	"github.com/blevesearch/bleve/v2/search/query"
)

// Document represents a searchable document.
// Callers are responsible for constructing this from their domain objects.
type Document struct {
	ID          string // unique identifier
	Type        string // document type (for filtering)
	Primary     string // primary display field (title/name/label)
	Description string // description text
	Content     string // body content
	Properties  string // all property values joined
}

// Index wraps a Bleve index for document search.
type Index struct {
	index bleve.Index
}

// bleveDoc is the internal document structure indexed by Bleve.
type bleveDoc struct {
	ID          string `json:"id"`
	Type        string `json:"type"`
	Primary     string `json:"primary"`
	Description string `json:"description"`
	Content     string `json:"content"`
	Properties  string `json:"properties"`
	All         string `json:"all"` // everything combined for simple queries
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
	docMapping.AddFieldMappingsAt("primary", textFieldMapping)
	docMapping.AddFieldMappingsAt("description", textFieldMapping)
	docMapping.AddFieldMappingsAt("content", textFieldMapping)
	docMapping.AddFieldMappingsAt("properties", textFieldMapping)
	docMapping.AddFieldMappingsAt("all", textFieldMapping)

	indexMapping := bleve.NewIndexMapping()
	indexMapping.DefaultMapping = docMapping
	indexMapping.DefaultAnalyzer = standard.Name

	return indexMapping
}

// Index adds or updates a document in the index.
func (idx *Index) Index(doc Document) error {
	return idx.index.Index(doc.ID, toBleveDoc(doc))
}

// IndexBatch indexes multiple documents (for initial load).
func (idx *Index) IndexBatch(docs []Document) error {
	batch := idx.index.NewBatch()
	for _, doc := range docs {
		if err := batch.Index(doc.ID, toBleveDoc(doc)); err != nil {
			return fmt.Errorf("failed to batch index %s: %w", doc.ID, err)
		}
	}
	return idx.index.Batch(batch)
}

// Remove removes a document from the index.
func (idx *Index) Remove(id string) error {
	return idx.index.Delete(id)
}

// Field boost weights for search ranking.
const (
	boostID         = 5.0 // ID field gets highest boost (exact match)
	boostPrimary    = 3.0 // Primary field (title/name/label) gets high boost
	boostProperties = 2.0 // Other properties get medium boost
	boostContent    = 1.0 // Body content gets base boost
)

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

// Search performs a search query and returns scored results.
// words are OR'd together with fuzzy matching.
// phrases must all match exactly (AND logic).
// Results are ranked with field boosting: primary (3x) > properties (2x) > content (1x).
func (idx *Index) Search(words, phrases []string, limit int) ([]Result, error) {
	if len(words) == 0 && len(phrases) == 0 {
		return nil, nil
	}

	queries := make([]query.Query, 0, len(words))

	// Add boosted queries for each word across fields
	for _, word := range words {
		word = strings.ToLower(word)
		queries = append(queries, buildBoostedWordQuery(word))
	}

	// Add phrase queries (must all match - AND logic)
	phraseQueries := make([]query.Query, 0, len(phrases))
	for _, phrase := range phrases {
		phraseQueries = append(phraseQueries, buildBoostedPhraseQuery(phrase))
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

// buildBoostedWordQuery creates a disjunction query across fields with boosting.
// ID field is boosted 5x, primary 3x, properties 2x, content 1x.
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

// phraseFields defines the fields to search for phrase queries (excludes ID and all).
var phraseFields = []struct {
	field string
	boost float64
}{
	{"primary", boostPrimary},
	{"properties", boostProperties},
	{"content", boostContent},
}

// buildBoostedPhraseQuery creates a phrase query across fields with boosting.
func buildBoostedPhraseQuery(phrase string) query.Query {
	queries := make([]query.Query, 0, len(phraseFields))
	for _, f := range phraseFields {
		pq := bleve.NewMatchPhraseQuery(phrase)
		pq.SetField(f.field)
		pq.SetBoost(f.boost)
		queries = append(queries, pq)
	}
	return bleve.NewDisjunctionQuery(queries...)
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

// toBleveDoc converts a Document to the internal Bleve document format.
func toBleveDoc(doc Document) bleveDoc {
	all := strings.Join([]string{
		doc.ID,
		doc.Primary,
		doc.Description,
		doc.Content,
		doc.Properties,
	}, " ")

	return bleveDoc{
		ID:          doc.ID,
		Type:        doc.Type,
		Primary:     doc.Primary,
		Description: doc.Description,
		Content:     doc.Content,
		Properties:  doc.Properties,
		All:         all,
	}
}
