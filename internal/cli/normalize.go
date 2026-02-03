package cli

import (
	"bytes"
	"strings"

	"github.com/spf13/cobra"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"

	"github.com/Sourcehaven-BV/rela/internal/model"
)

const (
	targetHeaderLevel = 2
	maxHeaderLevel    = 6
)

var (
	normalizeDryRun bool
)

var normalizeCmd = &cobra.Command{
	Use:   "normalize [type]",
	Short: "Normalize markdown headers in entity files",
	Long: `Normalizes markdown headers in entity files to start at level 2 (##).

This command adjusts header levels so the minimum header level in each entity
is ##, preserving the relative hierarchy. For example, if an entity has:
  # Overview
  ## Details
  ### Subsection

It will be normalized to:
  ## Overview
  ### Details
  #### Subsection

Setext-style headers (underlined with === or ---) are converted to ATX style (##).

If headers already start at ## or deeper, no changes are made.

Examples:
  rela normalize                # Normalize all entities
  rela normalize requirements   # Normalize only requirements
  rela normalize req            # Alias works too
  rela normalize --dry-run      # Preview changes without writing`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var entities []*model.Entity

		if len(args) > 0 {
			typeName := args[0]
			resolvedType, _, err := resolveEntityType(typeName)
			if err != nil {
				return err
			}
			entities = g.NodesByType(resolvedType)
		} else {
			entities = g.AllNodes()
		}

		if len(entities) == 0 {
			out.WriteMessage("No entities found")
			return nil
		}

		modified := 0
		for _, entity := range entities {
			normalized := normalizeHeaders(entity.Content)
			if normalized == entity.Content {
				continue
			}

			if normalizeDryRun {
				out.WriteMessage("Would normalize: %s", entity.ID)
				modified++
				continue
			}

			entity.Content = normalized

			if err := repo.WriteEntity(entity, meta); err != nil {
				out.WriteWarning("Failed to write %s: %v", entity.ID, err)
				continue
			}

			g.AddNode(entity)
			modified++

			if verbose {
				out.WriteMessage("Normalized: %s", entity.ID)
			}
		}

		if !normalizeDryRun && modified > 0 {
			if err := saveCache(); err != nil {
				out.WriteWarning("Failed to save cache: %v", err)
			}
		}

		if normalizeDryRun {
			out.WriteMessage("Dry run: %d entities would be modified", modified)
		} else {
			out.WriteSuccess("Normalized %d entities", modified)
		}

		return nil
	},
}

// headerInfo stores information about a heading found in the document.
type headerInfo struct {
	level     int
	lineStart int    // byte position of line start
	isSetext  bool   // true if setext-style (underlined)
	fullEnd   int    // byte position after the entire header (including underline + newline)
	text      string // header text content
}

// normalizeHeaders adjusts markdown headers so the minimum level is 2 (##).
// It uses goldmark to parse the markdown AST, ensuring proper handling of
// code blocks and other markdown constructs. Setext headers are converted to ATX.
func normalizeHeaders(content string) string {
	if content == "" {
		return content
	}

	source := []byte(content)
	headers := collectHeaders(source)

	if len(headers) == 0 {
		return content
	}

	minLevel := findMinLevel(headers)
	if minLevel >= targetHeaderLevel {
		return content
	}

	shift := targetHeaderLevel - minLevel
	return applyHeaderShift(source, headers, shift)
}

// collectHeaders parses the markdown and returns information about all headers.
func collectHeaders(source []byte) []headerInfo {
	md := goldmark.New()
	reader := text.NewReader(source)
	doc := md.Parser().Parse(reader)

	var headers []headerInfo
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		if heading, ok := n.(*ast.Heading); ok {
			if h := parseHeading(heading, source); h != nil {
				headers = append(headers, *h)
			}
		}
		return ast.WalkContinue, nil
	})
	return headers
}

// parseHeading extracts header information from an AST heading node.
func parseHeading(heading *ast.Heading, source []byte) *headerInfo {
	lines := heading.Lines()
	if lines.Len() == 0 {
		return nil
	}

	seg := lines.At(0)
	headerText := string(source[seg.Start:seg.Stop])

	lineStart := seg.Start
	for lineStart > 0 && source[lineStart-1] != '\n' {
		lineStart--
	}

	isATX := lineStart < len(source) && source[lineStart] == '#'

	h := &headerInfo{
		level:     heading.Level,
		lineStart: lineStart,
		text:      headerText,
		isSetext:  !isATX,
	}

	if isATX {
		h.fullEnd = findLineEnd(source, seg.Stop)
	} else {
		h.fullEnd = findSetextEnd(source, seg.Stop)
	}

	return h
}

// findLineEnd returns the position at the end of the current line.
func findLineEnd(source []byte, pos int) int {
	for pos < len(source) && source[pos] != '\n' {
		pos++
	}
	return pos
}

// findSetextEnd returns the position after the setext underline.
func findSetextEnd(source []byte, pos int) int {
	// Skip to end of header text line
	for pos < len(source) && source[pos] != '\n' {
		pos++
	}
	if pos < len(source) {
		pos++ // skip newline
	}
	// Skip underline characters
	for pos < len(source) && (source[pos] == '=' || source[pos] == '-') {
		pos++
	}
	// Skip trailing whitespace on underline
	for pos < len(source) && source[pos] != '\n' {
		pos++
	}
	return pos
}

// findMinLevel returns the minimum header level in the slice.
func findMinLevel(headers []headerInfo) int {
	minLevel := maxHeaderLevel + 1
	for _, h := range headers {
		if h.level < minLevel {
			minLevel = h.level
		}
	}
	return minLevel
}

// applyHeaderShift modifies the source by shifting all header levels.
func applyHeaderShift(source []byte, headers []headerInfo, shift int) string {
	result := source
	// Process in reverse order to maintain correct positions
	for i := len(headers) - 1; i >= 0; i-- {
		h := headers[i]
		newLevel := h.level + shift
		if newLevel > maxHeaderLevel {
			newLevel = maxHeaderLevel
		}

		if h.isSetext {
			newHeader := strings.Repeat("#", newLevel) + " " + h.text
			result = replaceRange(result, h.lineStart, h.fullEnd, []byte(newHeader))
		} else {
			oldHashes := strings.Repeat("#", h.level)
			newHashes := strings.Repeat("#", newLevel)
			if h.lineStart+len(oldHashes) <= len(result) &&
				string(result[h.lineStart:h.lineStart+len(oldHashes)]) == oldHashes {

				result = replaceRange(result, h.lineStart, h.lineStart+len(oldHashes), []byte(newHashes))
			}
		}
	}
	return string(result)
}

// replaceRange replaces bytes from start to end with replacement.
func replaceRange(data []byte, start, end int, replacement []byte) []byte {
	var buf bytes.Buffer
	buf.Write(data[:start])
	buf.Write(replacement)
	buf.Write(data[end:])
	return buf.Bytes()
}

func init() {
	normalizeCmd.Flags().BoolVar(&normalizeDryRun, "dry-run", false, "Preview changes without writing")

	rootCmd.AddCommand(normalizeCmd)
}
