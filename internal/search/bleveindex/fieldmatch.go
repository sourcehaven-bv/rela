package bleveindex

import (
	"path/filepath"
	"strings"
	"sync"

	"github.com/blevesearch/bleve/v2/analysis"
	"github.com/blevesearch/bleve/v2/analysis/analyzer/standard"
	"github.com/blevesearch/bleve/v2/registry"
	blevesearch "github.com/blevesearch/bleve/v2/search"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/search"
)

// compile-time check: Index provides match provenance.
var _ search.FieldMatcher = (*Index)(nil)

// searchFuzziness mirrors the FuzzyQuery fuzziness used by Index.Search
// (buildBoostedWordQuery). Keep the two in sync: a MatchedFields pass that is
// LESS permissive than the query would drop a hit the query legitimately
// surfaced from a visible field (a false drop). More permissive is safe.
const searchFuzziness = 1

// MatchedFields reports which logical fields of e the query text matched, using
// the same standard analyzer and fuzzy/wildcard/prefix rules as Index.Search,
// so the provenance is faithful to what the index would match. It answers per
// field (id / content / prop:<name>) rather than over the concatenated blob,
// which is what lets the ACL seam distinguish a visible-field hit from a
// hidden-field hit within the same entity.
//
// Superset-safety: the result is UNIONED with a plain case-insensitive
// substring pass ([search.MatchTextFields]) so it can never under-report
// relative to the simplest possible match — the fuzzy/analyzer path only ever
// ADDS fields. This upholds the FieldMatcher "never omit a matched field"
// contract (no false drops), at the cost of occasionally reporting an extra
// field (which only ever KEEPS a hit — safe).
func (idx *Index) MatchedFields(e *entity.Entity, text string) map[string]struct{} {
	if text == "" {
		return nil
	}

	// Base: exact substring per field — the guaranteed floor.
	out := search.MatchTextFields(e, text)
	if out == nil {
		out = make(map[string]struct{})
	}

	analyzer := idx.textAnalyzer()
	if analyzer == nil {
		return normalizeEmpty(out) // analyzer unavailable: substring floor only
	}

	queryTokens := analyzeTerms(analyzer, text)
	rawTerms := strings.Fields(strings.ToLower(text))

	// content
	if fieldTextMatches(analyzer, e.Content, queryTokens, rawTerms) {
		out[search.FieldContent] = struct{}{}
	}
	// id: keyword-analyzed in the index (case-sensitive prefix/exact), so match
	// the raw id against the raw query, mirroring the idExact/idPrefix queries.
	if idMatches(e.ID, strings.TrimSpace(text)) {
		out[search.FieldID] = struct{}{}
	}
	// properties: per-property, so a hidden prop's match is distinguishable.
	for name, v := range e.Properties {
		s, ok := v.(string)
		if !ok || s == "" {
			continue
		}
		if fieldTextMatches(analyzer, s, queryTokens, rawTerms) {
			out[search.PropFieldPrefix+name] = struct{}{}
		}
	}

	return normalizeEmpty(out)
}

// cachedStandardAnalyzer resolves the standard analyzer once for the process.
// The standard analyzer is stateless and safe to share across goroutines, and
// MatchedFields runs once per surviving hit — building a fresh registry cache
// per call (the previous shape) was needless hot-path allocation. Nil when the
// registry cannot provide it; callers fall back to the substring floor.
var cachedStandardAnalyzer = sync.OnceValue(func() analysis.Analyzer {
	a, err := registry.NewCache().AnalyzerNamed(standard.Name)
	if err != nil {
		return nil
	}
	return a
})

// textAnalyzer returns the standard analyzer used by the text fields in the
// index mapping (see buildMapping).
func (idx *Index) textAnalyzer() analysis.Analyzer {
	return cachedStandardAnalyzer()
}

// analyzeTerms tokenizes text with the analyzer, lower-cased token terms.
func analyzeTerms(a analysis.Analyzer, text string) []string {
	stream := a.Analyze([]byte(text))
	terms := make([]string, 0, len(stream))
	for _, tok := range stream {
		terms = append(terms, string(tok.Term))
	}
	return terms
}

// fieldTextMatches reports whether the field value matches the query under the
// same rules Index.Search applies to text fields: a fuzzy (edit-distance
// searchFuzziness) or wildcard token match. rawTerms carries the un-analyzed
// query words for wildcard handling (wildcards aren't analyzer tokens).
func fieldTextMatches(a analysis.Analyzer, fieldValue string, queryTokens, rawTerms []string) bool {
	fieldTokens := analyzeTerms(a, fieldValue)
	if len(fieldTokens) == 0 {
		return false
	}
	fieldSet := make(map[string]struct{}, len(fieldTokens))
	for _, ft := range fieldTokens {
		fieldSet[ft] = struct{}{}
	}

	for _, qt := range queryTokens {
		if _, exact := fieldSet[qt]; exact {
			return true
		}
		for ft := range fieldSet {
			if withinEditDistance(qt, ft, searchFuzziness) {
				return true
			}
		}
	}
	// Wildcards operate on the raw (pre-analysis) term against field tokens.
	for _, rt := range rawTerms {
		if !strings.ContainsAny(rt, "*?") {
			continue
		}
		for ft := range fieldSet {
			if wildcardMatch(rt, ft) {
				return true
			}
		}
	}
	return false
}

// idMatches mirrors the keyword-analyzed id field: case-sensitive exact or
// prefix match of the raw query against the raw id.
func idMatches(id, rawQuery string) bool {
	if rawQuery == "" {
		return false
	}
	return id == rawQuery || strings.HasPrefix(id, rawQuery)
}

// withinEditDistance reports whether a and b are within max edits, using
// bleve's own Levenshtein so the fuzzy match agrees with the index FuzzyQuery.
//
// LevenshteinDistanceMax returns (distance, exceededMax): the bool is true when
// the computation short-circuited because the distance is GREATER than max, so
// "within" means NOT exceeded and distance ≤ max.
func withinEditDistance(a, b string, maxDist int) bool {
	dist, exceeded := blevesearch.LevenshteinDistanceMax(a, b, maxDist)
	return !exceeded && dist <= maxDist
}

// wildcardMatch reports whether a `*`/`?` pattern matches a token, mirroring
// the index WildcardQuery. filepath.Match's glob semantics (`*` any run, `?`
// one char) match bleve's wildcard operators for the single-token case.
func wildcardMatch(pattern, token string) bool {
	ok, err := filepath.Match(pattern, token)
	return err == nil && ok
}

func normalizeEmpty(m map[string]struct{}) map[string]struct{} {
	if len(m) == 0 {
		return nil
	}
	return m
}
