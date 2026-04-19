package fsstore

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/mitchellh/go-wordwrap"
	gmmarkdown "github.com/teekennedy/goldmark-markdown"
	"github.com/yuin/goldmark"
	"gopkg.in/yaml.v3"

	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// errConflictedFile is returned when a file has unresolved git conflict markers.
var errConflictedFile = errors.New("file has unresolved git conflicts")

const (
	frontmatterDelimiter = "---"
	defaultLineWidth     = 80
)

var conflictMarkerStart = []byte("<<<<<<<")

// orderedListPattern matches ordered list items (e.g., "1. ", "2. ").
var orderedListPattern = regexp.MustCompile(`^\d+\.\s`)

// --- document parsing ---

// document represents a parsed markdown file with YAML frontmatter.
type document struct {
	frontmatter map[string]interface{}
	content     string
}

func (d *document) getString(key string) string {
	if d.frontmatter == nil {
		return ""
	}
	if v, ok := d.frontmatter[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// parseDocument parses a markdown file with YAML frontmatter.
func parseDocument(raw string) (*document, error) {
	if strings.Contains(raw, string(conflictMarkerStart)) {
		return nil, errConflictedFile
	}

	fm, body, err := splitFrontmatter(raw)
	if err != nil {
		return nil, err
	}

	var parsed map[string]interface{}
	if fm != "" {
		if err := yaml.Unmarshal([]byte(fm), &parsed); err != nil {
			return nil, fmt.Errorf("failed to parse frontmatter: %w", err)
		}
	}

	return &document{frontmatter: parsed, content: body}, nil
}

func splitFrontmatter(content string) (frontmatter, body string, err error) {
	scanner := bufio.NewScanner(strings.NewReader(content))
	var lines []string
	inFrontmatter := false
	frontmatterEnded := false
	var frontmatterLines []string

	for scanner.Scan() {
		line := scanner.Text()

		if !inFrontmatter && !frontmatterEnded && strings.TrimSpace(line) == frontmatterDelimiter {
			inFrontmatter = true
			continue
		}

		if inFrontmatter && strings.TrimSpace(line) == frontmatterDelimiter {
			inFrontmatter = false
			frontmatterEnded = true
			continue
		}

		if inFrontmatter {
			frontmatterLines = append(frontmatterLines, line)
		} else if frontmatterEnded || !inFrontmatter {
			lines = append(lines, line)
		}
	}

	if err := scanner.Err(); err != nil {
		return "", "", err
	}

	frontmatter = strings.Join(frontmatterLines, "\n")
	body = strings.TrimPrefix(strings.Join(lines, "\n"), "\n")

	return frontmatter, body, nil
}

// --- document formatting ---

func formatDocumentOrdered(fm map[string]interface{}, content string, keyOrder []string) (string, error) {
	var sb strings.Builder

	if len(fm) > 0 {
		sb.WriteString(frontmatterDelimiter)
		sb.WriteString("\n")

		var yamlBytes []byte
		var err error

		if len(keyOrder) > 0 {
			yamlBytes, err = marshalOrdered(fm, keyOrder)
		} else {
			yamlBytes, err = yaml.Marshal(fm)
		}
		if err != nil {
			return "", err
		}
		sb.Write(yamlBytes)
		sb.WriteString(frontmatterDelimiter)
		sb.WriteString("\n")
	}

	if content != "" {
		sb.WriteString("\n")
		sb.WriteString(content)
		if !strings.HasSuffix(content, "\n") {
			sb.WriteString("\n")
		}
	}

	return sb.String(), nil
}

func marshalOrdered(data map[string]interface{}, keyOrder []string) ([]byte, error) {
	node := &yaml.Node{Kind: yaml.MappingNode}
	added := make(map[string]bool)

	for _, key := range keyOrder {
		val, ok := data[key]
		if !ok {
			continue
		}
		node.Content = append(node.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: key},
		)
		valNode, err := valueToNode(val)
		if err != nil {
			return nil, err
		}
		node.Content = append(node.Content, valNode)
		added[key] = true
	}

	var remaining []string
	for key := range data {
		if !added[key] {
			remaining = append(remaining, key)
		}
	}
	sort.Strings(remaining)

	for _, key := range remaining {
		node.Content = append(node.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: key},
		)
		valNode, err := valueToNode(data[key])
		if err != nil {
			return nil, err
		}
		node.Content = append(node.Content, valNode)
	}

	return yaml.Marshal(node)
}

func valueToNode(val interface{}) (*yaml.Node, error) {
	var node yaml.Node
	if err := node.Encode(val); err != nil {
		return nil, err
	}
	return &node, nil
}

// --- markdown content formatting ---

func formatMarkdown(content string) string {
	if content == "" {
		return ""
	}

	content = strings.TrimRight(content, " \t")

	r := gmmarkdown.NewRenderer(
		gmmarkdown.WithHeadingStyle(gmmarkdown.HeadingStyleATX),
		gmmarkdown.WithIndentStyle(gmmarkdown.IndentStyleSpaces),
	)

	md := goldmark.New(goldmark.WithRenderer(r))

	var buf bytes.Buffer
	if err := md.Convert([]byte(content), &buf); err != nil {
		return content
	}

	result := wrapParagraphs(buf.String(), defaultLineWidth)
	result = strings.TrimRight(result, "\n") + "\n"
	return result
}

func wrapParagraphs(content string, lineWidth int) string {
	lines := strings.Split(content, "\n")
	var result []string
	paragraphLines := make([]string, 0, 10)
	inCodeBlock := false
	codeBlockMarker := ""

	if lineWidth <= 0 {
		lineWidth = defaultLineWidth
	}

	flushParagraph := func() {
		if len(paragraphLines) > 0 {
			text := strings.Join(paragraphLines, " ")
			text = strings.TrimSpace(text)
			if text != "" {
				wrapped := wordwrap.WrapString(text, uint(lineWidth)) //nolint:gosec // lineWidth is validated positive
				result = append(result, wrapped)
			}
			paragraphLines = paragraphLines[:0]
		}
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			flushParagraph()
			switch {
			case !inCodeBlock:
				inCodeBlock = true
				codeBlockMarker = trimmed[:3]
				result = append(result, line)
			case strings.HasPrefix(trimmed, codeBlockMarker):
				inCodeBlock = false
				codeBlockMarker = ""
				result = append(result, line)
			default:
				result = append(result, line)
			}
			continue
		}

		if inCodeBlock {
			result = append(result, line)
			continue
		}

		if isSpecialLine(trimmed) {
			flushParagraph()
			result = append(result, line)
			continue
		}

		if trimmed == "" {
			flushParagraph()
			result = append(result, "")
			continue
		}

		paragraphLines = append(paragraphLines, trimmed)
	}

	flushParagraph()

	return strings.Join(result, "\n")
}

func isSpecialLine(line string) bool {
	if line == "" {
		return true
	}
	if strings.HasPrefix(line, "#") {
		return true
	}
	if strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") || strings.HasPrefix(line, "+ ") {
		return true
	}
	if orderedListPattern.MatchString(line) {
		return true
	}
	if strings.HasPrefix(line, ">") {
		return true
	}
	if line == "---" || line == "***" || line == "___" {
		return true
	}
	if strings.HasPrefix(line, "    ") || strings.HasPrefix(line, "\t") {
		return true
	}
	if strings.HasPrefix(line, "<!--") {
		return true
	}
	if strings.HasPrefix(line, "|") {
		return true
	}
	return false
}

// --- entity I/O ---

// readEntityFile reads and parses an entity from a markdown file.
func (s *FSStore) readEntityFile(path string) (*entity.Entity, error) {
	data, err := s.fs.ReadFile(path)
	if err != nil {
		return nil, err
	}

	doc, err := parseDocument(string(data))
	if err != nil {
		return nil, err
	}

	id := doc.getString("id")
	entityType := doc.getString("type")

	e := entity.New(id, entityType)

	if info, err := s.fs.Stat(path); err == nil {
		e.UpdatedAt = info.ModTime()
	}

	// If the frontmatter carries any encrypted keys or an envelope,
	// unseal through the Crypto layer. Cleartext files bypass the
	// encryption path entirely (nil-crypto fast path).
	frontmatter, body, _, unErr := unsealProperties(s.crypto, doc.frontmatter, doc.content)
	if unErr != nil {
		return nil, unErr
	}
	e.Content = body

	for key, value := range frontmatter {
		if key != "id" && key != "type" {
			e.Properties[key] = entity.CloneValue(value)
		}
	}

	return e, nil
}

// formatEntity formats an entity as markdown with YAML frontmatter.
// crypto may be nil (cleartext-only stores).
func formatEntity(e *entity.Entity, propertyOrder []string, crypto Crypto) (string, error) {
	// Split props (user-defined) from the fixed id/type pair.
	props := make(map[string]any, len(e.Properties))
	for key, value := range e.Properties {
		props[key] = value
	}

	// Body conflict check (criterion 15): if this entity type's body
	// is declared encrypted AND Content is cleartext + non-empty,
	// that's the normal first-write case — seal the content.
	// But if the caller supplies both a pre-existing _encrypted_body
	// and non-empty Content, refuse. Here we only see cleartext
	// input, so the body conflict is structurally impossible at this
	// layer — the decoder side ensures _encrypted_body never lands
	// in e.Properties (it gets moved to e.Content on read).

	sealedFM, sealedOrder, sealedBody, err := sealProperties(
		crypto, e.Type, props, e.Content, propertyOrder)
	if err != nil {
		return "", err
	}

	fm := make(map[string]any, len(sealedFM)+2)
	fm["id"] = e.ID
	fm["type"] = e.Type
	for k, v := range sealedFM {
		fm[k] = v
	}

	keyOrder := make([]string, 0, 2+len(sealedOrder))
	keyOrder = append(keyOrder, "id", "type")
	keyOrder = append(keyOrder, sealedOrder...)

	content := sealedBody
	if content != "" {
		content = formatMarkdown(content)
	}

	return formatDocumentOrdered(fm, content, keyOrder)
}

// writeEntityFile writes an entity to a markdown file using temp-file + rename.
func (s *FSStore) writeEntityFile(e *entity.Entity) error {
	path := s.entityFilePath(e.Type, e.ID)
	tempPath := path + ".new"

	order := s.propertyOrder(e.Type)
	content, err := formatEntity(e, order, s.crypto)
	if err != nil {
		return err
	}

	dir := filepath.Dir(tempPath)
	if err := s.fs.MkdirAll(dir, 0755); err != nil {
		return err
	}

	if err := s.fs.WriteFile(tempPath, []byte(content), 0644); err != nil {
		return err
	}
	if err := s.fs.Rename(tempPath, path); err != nil {
		return err
	}
	s.recordHash(path, []byte(content))
	return nil
}

// --- relation I/O ---

// readRelationFile reads and parses a relation from a markdown file.
func (s *FSStore) readRelationFile(path string) (*entity.Relation, error) {
	data, err := s.fs.ReadFile(path)
	if err != nil {
		return nil, err
	}

	doc, err := parseDocument(string(data))
	if err != nil {
		return nil, err
	}

	r := entity.NewRelation(
		doc.getString("from"),
		doc.getString("relation"),
		doc.getString("to"),
	)
	r.Content = doc.content

	if info, err := s.fs.Stat(path); err == nil {
		r.UpdatedAt = info.ModTime()
	}

	for key, value := range doc.frontmatter {
		if key != "from" && key != "relation" && key != "to" {
			if r.Properties == nil {
				r.Properties = make(map[string]interface{})
			}
			r.Properties[key] = entity.CloneValue(value)
		}
	}

	return r, nil
}

// formatRelation formats a relation as markdown with YAML frontmatter.
func formatRelation(r *entity.Relation) (string, error) {
	fm := map[string]interface{}{
		"from":     r.From,
		"relation": r.Type,
		"to":       r.To,
	}
	for key, value := range r.Properties {
		fm[key] = value
	}

	keyOrder := []string{"from", "relation", "to"}

	content := r.Content
	if content != "" {
		content = formatMarkdown(content)
	}

	return formatDocumentOrdered(fm, content, keyOrder)
}

// writeRelationFile writes a relation to a markdown file using temp-file + rename.
func (s *FSStore) writeRelationFile(r *entity.Relation) error {
	path := s.relationFilePath(r.From, r.Type, r.To)
	tempPath := path + ".new"

	content, err := formatRelation(r)
	if err != nil {
		return err
	}

	dir := filepath.Dir(tempPath)
	if err := s.fs.MkdirAll(dir, 0755); err != nil {
		return err
	}

	if err := s.fs.WriteFile(tempPath, []byte(content), 0644); err != nil {
		return err
	}
	if err := s.fs.Rename(tempPath, path); err != nil {
		return err
	}
	s.recordHash(path, []byte(content))
	return nil
}

// hashContent returns the hex-encoded SHA256 of content. Used by the
// external-change watcher to suppress self-echoes from fsnotify.
func hashContent(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}

// recordHash stores the hash of content written to path. The LRU is
// self-synchronized so no store lock is required.
func (s *FSStore) recordHash(path string, content []byte) {
	s.recentHashes.Put(path, hashContent(content))
}

// forgetHash removes any recorded hash for path (e.g. after delete).
func (s *FSStore) forgetHash(path string) {
	s.recentHashes.Delete(path)
}
