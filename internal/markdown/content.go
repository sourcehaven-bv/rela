package markdown

import (
	"regexp"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	extast "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/text"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// ExtractHeaders extracts all markdown headers from content using goldmark's AST parser.
// This properly handles headers and ignores lines in code blocks or other non-header contexts.
func ExtractHeaders(content string) []string {
	if content == "" {
		return nil
	}

	source := []byte(content)
	reader := text.NewReader(source)
	p := goldmark.DefaultParser()
	doc := p.Parse(reader)

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
func CheckContentRule(content string, rule *metamodel.ContentRule) bool {
	if rule == nil {
		return true
	}

	headers := ExtractHeaders(content)

	for _, headerCheck := range rule.RequiredHeaders {
		if !MatchHeader(headers, headerCheck) {
			return false
		}
	}

	// Check checklist rules
	if rule.Checklist != nil {
		items := ExtractChecklistItems(content)
		if !CheckChecklistRule(items, rule.Checklist) {
			return false
		}
	}

	return true
}

// ChecklistItem represents a task list item in markdown.
type ChecklistItem struct {
	Checked bool   // Whether the checkbox is checked (- [x])
	Skipped bool   // Whether the item is strikethrough (~~text~~)
	Text    string // The text content of the item
}

// ExtractChecklistItems extracts all markdown checklist items from content using goldmark's AST parser.
// This properly handles task lists and detects strikethrough items.
func ExtractChecklistItems(content string) []ChecklistItem {
	if content == "" {
		return nil
	}

	source := []byte(content)
	reader := text.NewReader(source)

	// Create parser with TaskList and Strikethrough extensions enabled
	md := goldmark.New(
		goldmark.WithExtensions(extension.TaskList, extension.Strikethrough),
	)
	mdParser := md.Parser()
	doc := mdParser.Parse(reader)

	var items []ChecklistItem
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}

		// Look for TaskCheckBox nodes
		if checkbox, ok := n.(*extast.TaskCheckBox); ok {
			// Get the parent list item to extract text
			listItem := findParentListItem(n)
			if listItem == nil {
				return ast.WalkContinue, nil
			}

			// Extract text content and check for strikethrough
			itemText, hasStrikethrough := extractListItemText(listItem, source)

			items = append(items, ChecklistItem{
				Checked: checkbox.IsChecked,
				Skipped: hasStrikethrough,
				Text:    itemText,
			})
		}
		return ast.WalkContinue, nil
	})

	return items
}

// findParentListItem walks up the AST to find the parent ListItem node.
func findParentListItem(n ast.Node) *ast.ListItem {
	for p := n.Parent(); p != nil; p = p.Parent() {
		if li, ok := p.(*ast.ListItem); ok {
			return li
		}
	}
	return nil
}

// extractListItemText extracts the text content of a list item and detects strikethrough.
// Processes all direct children except nested lists to get the item's main text content.
func extractListItemText(li *ast.ListItem, source []byte) (string, bool) {
	var textContent strings.Builder
	hasStrikethrough := false

	// Process all direct children except nested lists
	for c := li.FirstChild(); c != nil; c = c.NextSibling() {
		// Skip nested lists
		if _, ok := c.(*ast.List); ok {
			continue
		}

		_ = ast.Walk(c, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
			if !entering {
				return ast.WalkContinue, nil
			}

			// Skip the checkbox itself
			if _, ok := n.(*extast.TaskCheckBox); ok {
				return ast.WalkContinue, nil
			}

			// Check for strikethrough
			if _, ok := n.(*extast.Strikethrough); ok {
				hasStrikethrough = true
				return ast.WalkContinue, nil
			}

			// Collect text content
			if t, ok := n.(*ast.Text); ok {
				textContent.Write(t.Segment.Value(source))
			}

			return ast.WalkContinue, nil
		})
	}

	return strings.TrimSpace(textContent.String()), hasStrikethrough
}

// CheckChecklistRule validates checklist items against a checklist rule.
func CheckChecklistRule(items []ChecklistItem, rule *metamodel.ChecklistRule) bool {
	if rule == nil {
		return true
	}

	// If no checklist items, nothing to validate
	if len(items) == 0 {
		return true
	}

	if rule.AllChecked {
		for _, item := range items {
			// Item passes if:
			// - It's checked, OR
			// - It's skipped (strikethrough) AND allow-skipped is true
			if item.Checked {
				continue
			}
			if rule.AllowSkipped && item.Skipped {
				continue
			}
			// Found an unchecked item that isn't skipped
			return false
		}
	}

	return true
}
