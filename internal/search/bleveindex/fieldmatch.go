package bleveindex

import (
	"regexp"
	"slices"
	"strings"
	"sync"

	"github.com/blevesearch/bleve/v2/analysis"
	"github.com/blevesearch/bleve/v2/analysis/analyzer/standard"
	"github.com/blevesearch/bleve/v2/registry"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/search"
)

// compile-time check: Index provides match provenance.
var _ search.FieldMatcher = (*Index)(nil)

// searchFuzziness mirrors the FuzzyQuery fuzziness buildBoostedWordQuery
// sets. Keep the two in sync (see [Index.MatchedFields]).
const searchFuzziness = 1

// MatchedFields reports which logical fields of e the query text matched
// (id, content, prop:<name>), by the rules [Index.SearchAdmitted] uses:
//
//   - id: the trimmed text equals the id or is a prefix of it (the idExact
//     and idPrefix queries), or a word matches a token of the id as the
//     "all" field analyzes it.
//   - every other field: a lowercased query word, unanalyzed, matches one of
//     the field's standard-analyzer tokens. A word with '*' or '?' matches
//     as a whole-token wildcard; any other word within one rune edit.
//
// It must not report a field the search did not match on. A field scope
// keeps a hit when a visible field matched, so an extra visible field would
// return a hit whose real match was a hidden property. An earlier version
// unioned [search.MatchTextFields] and the query's analyzer tokens into the
// result. Both over-report: "xxfoobar" holds the substring "foobar" but is
// two edits from it, and the word "foo,bar" analyzes to "foo" and "bar" while
// bleve matches the raw word.
func (idx *Index) MatchedFields(e *entity.Entity, text string) map[string]struct{} {
	words := strings.Fields(strings.ToLower(text))
	if len(words) == 0 {
		return nil
	}
	analyzer := idx.textAnalyzer()
	if analyzer == nil {
		// coverage-ignore-start: defensive: textAnalyzer only returns nil if the bleve registry lacks the standard
		// analyzer, which never happens with bleve linked
		return nil // nothing can be attributed, so the field scope drops the hit
		// coverage-ignore-end
	}
	m := newWordMatcher(words)
	out := make(map[string]struct{})
	if idMatches(e.ID, strings.TrimSpace(text)) || m.matches(analyzeTerms(analyzer, e.ID)) {
		out[search.FieldID] = struct{}{}
	}
	if m.matches(analyzeTerms(analyzer, e.Content)) {
		out[search.FieldContent] = struct{}{}
	}
	for name, v := range e.Properties {
		if s, ok := v.(string); ok && s != "" && m.matches(analyzeTerms(analyzer, s)) {
			out[search.PropFieldPrefix+name] = struct{}{}
		}
	}
	return normalizeEmpty(out)
}

// cachedStandardAnalyzer resolves the standard analyzer once for the process.
// The standard analyzer is stateless and safe to share across goroutines, and
// MatchedFields runs once per surviving hit — building a fresh registry cache
// per call (the previous shape) was needless hot-path allocation. Nil when the
// registry cannot provide it; MatchedFields then reports no field.
var cachedStandardAnalyzer = sync.OnceValue(func() analysis.Analyzer {
	a, err := registry.NewCache().AnalyzerNamed(standard.Name)
	if err != nil {
		return nil // coverage-ignore: defensive: the standard analyzer is always registered when bleve is linked, so
		// AnalyzerNamed(standard.Name) never errors
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

// wordMatcher holds the per-word queries buildBoostedWordQuery issues for
// one search: a fuzzy word, or a wildcard compiled the way bleve compiles a
// WildcardQuery (whole-token regexp).
type wordMatcher struct {
	fuzzy     [][]rune
	wildcards []*regexp.Regexp
}

// wildcardReplacer is bleve's wildcardRegexpReplacer (search/query).
var wildcardReplacer = strings.NewReplacer(
	"+", `\+`, "(", `\(`, ")", `\)`, "^", `\^`, "$", `\$`, ".", `\.`,
	"{", `\{`, "}", `\}`, "[", `\[`, "]", `\]`, "|", `\|`, `\`, `\\`,
	"*", ".*", "?", ".",
)

func newWordMatcher(words []string) wordMatcher {
	var m wordMatcher
	for _, w := range words {
		if !strings.ContainsAny(w, "*?") {
			m.fuzzy = append(m.fuzzy, []rune(w))
			continue
		}
		re, err := regexp.Compile("^(?:" + wildcardReplacer.Replace(w) + ")$")
		if err != nil {
			continue // coverage-ignore: the replacer escapes every regexp metacharacter
		}
		m.wildcards = append(m.wildcards, re)
	}
	return m
}

// matches reports whether some word matches some of tokens.
func (m wordMatcher) matches(tokens []string) bool {
	for _, tok := range tokens {
		rt := []rune(tok)
		for _, w := range m.fuzzy {
			if withinOneEdit(w, rt) {
				return true
			}
		}
		for _, re := range m.wildcards {
			if re.MatchString(tok) {
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

// withinOneEdit reports whether a and b are at most searchFuzziness (one)
// rune edits apart. The index's fuzzy automaton counts runes, so a byte
// distance would disagree on non-ASCII text.
func withinOneEdit(a, b []rune) bool {
	if len(a) > len(b) {
		a, b = b, a
	}
	if len(b)-len(a) > searchFuzziness {
		return false
	}
	i := 0
	for i < len(a) && a[i] == b[i] {
		i++
	}
	if len(a) == len(b) {
		return i == len(a) || slices.Equal(a[i+1:], b[i+1:])
	}
	return slices.Equal(a[i:], b[i+1:])
}

func normalizeEmpty(m map[string]struct{}) map[string]struct{} {
	if len(m) == 0 {
		return nil
	}
	return m
}
