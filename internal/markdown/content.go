package markdown

import (
	"regexp"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/model"
)

// ExtractHeaders extracts all markdown headers from content using goldmark's AST parser.
// This properly handles headers and ignores lines in code blocks or other non-header contexts.
func ExtractHeaders(content string) []string {
	if content == "" {
		return nil
	}

	source := []byte(content)
	reader := text.NewReader(source)
	parser := goldmark.DefaultParser()
	doc := parser.Parse(reader)

	var headers []string
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		if heading, ok := n.(*ast.Heading); ok {
			// Reconstruct header text with proper # prefix
			var textContent strings.Builder
			for c := heading.FirstChild(); c != nil; c = c.NextSibling() {
				if t, ok := c.(*ast.Text); ok {
					textContent.Write(t.Segment.Value(source))
				}
			}
			header := strings.Repeat("#", heading.Level) + " " + textContent.String()
			headers = append(headers, header)
		}
		return ast.WalkContinue, nil
	})

	return headers
}

// MatchHeader checks if any header matches the given header check.
func MatchHeader(headers []string, check metamodel.HeaderCheck) bool {
	matchStr := check.GetMatchString()
	if matchStr == "" {
		return true
	}

	if check.IsPattern() {
		re, err := regexp.Compile(matchStr)
		if err != nil {
			return false
		}
		for _, h := range headers {
			if re.MatchString(h) {
				return true
			}
		}
		return false
	}

	// Exact match
	for _, h := range headers {
		if h == matchStr {
			return true
		}
	}
	return false
}

// CheckContentRule validates markdown content against content rules.
func CheckContentRule(entity *model.Entity, rule *metamodel.ContentRule) bool {
	if rule == nil {
		return true
	}

	headers := ExtractHeaders(entity.Content)

	for _, headerCheck := range rule.RequiredHeaders {
		if !MatchHeader(headers, headerCheck) {
			return false
		}
	}

	return true
}
