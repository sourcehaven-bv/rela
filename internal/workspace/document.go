package workspace

import (
	"bytes"
	"context"
	"fmt"
	"hash/fnv"
	"log/slog"
	"net/url"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
	"golang.org/x/sync/singleflight"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/htmlutil"
)

// docCacheDir is the subdirectory under .rela/ for document cache files.
const docCacheDir = "documents"

// DocumentConfig defines how to render a document from an entry entity.
type DocumentConfig struct {
	// Command is the external render command. Placeholders:
	//   {id}       - entry ID
	//   {id_lower} - lowercase entry ID
	Command string
	// Timeout is the command execution timeout. Defaults to 30s.
	Timeout time.Duration
}

// DocumentResult holds the result of rendering a document.
type DocumentResult struct {
	// HTML is the rendered HTML content.
	HTML string
	// ContentHash is the hash of source entities used for cache validation.
	ContentHash string
	// Entities contains all entities involved in the document (for dependency tracking).
	Entities []*entity.Entity
}

// docRenderGroup dedupes concurrent render requests for the same entry.
var docRenderGroup singleflight.Group

// GetCachedDocument returns a cached document if available and still valid.
// Returns nil if the cache is missing, stale, or on any error.
func (w *Workspace) GetCachedDocument(entryID string, _ DocumentConfig) *DocumentResult {
	entities, contentHash, err := w.computeDocumentHash(entryID)
	if err != nil {
		return nil
	}

	cacheFile := fmt.Sprintf("%s/%s-%s.html", docCacheDir, entryID, contentHash)
	cachedHTML, _ := w.ReadCacheFile(cacheFile)
	if cachedHTML == nil {
		return nil
	}

	return &DocumentResult{
		HTML:        string(cachedHTML),
		ContentHash: contentHash,
		Entities:    entities,
	}
}

// RenderDocument renders a document by executing the configured command.
// Uses singleflight to dedupe concurrent requests. Caches the result to disk.
func (w *Workspace) RenderDocument(entryID string, cfg DocumentConfig) (*DocumentResult, error) {
	entities, contentHash, err := w.computeDocumentHash(entryID)
	if err != nil {
		return nil, fmt.Errorf("computing document hash: %w", err)
	}

	cacheFile := fmt.Sprintf("%s/%s-%s.html", docCacheDir, entryID, contentHash)

	// Use singleflight to dedupe concurrent render requests for the same entry
	result, err, _ := docRenderGroup.Do(entryID, func() (interface{}, error) {
		return w.doRenderDocument(entryID, cfg, entities, contentHash, cacheFile)
	})
	if err != nil {
		return nil, err
	}

	docResult, _ := result.(*DocumentResult)
	return docResult, nil
}

// doRenderDocument performs the actual rendering work.
func (w *Workspace) doRenderDocument(
	entryID string, cfg DocumentConfig, entities []*entity.Entity, contentHash, cacheFile string,
) (*DocumentResult, error) {
	command := cfg.Command
	command = strings.ReplaceAll(command, "{id}", entryID)
	command = strings.ReplaceAll(command, "{id_lower}", strings.ToLower(entryID))

	markdown, err := w.executeCommand(command, cfg.Timeout)
	if err != nil {
		return nil, err
	}

	htmlContent, err := markdownToHTML(markdown)
	if err != nil {
		return nil, fmt.Errorf("markdown conversion: %w", err)
	}

	// Cache the result to disk. The cache is optional — a failure here is
	// not fatal — but it must be visible: silent cache rejections previously
	// hid validation regressions where unsafe IDs caused every render to
	// re-execute the command.
	if writeErr := w.WriteCacheFile(cacheFile, []byte(htmlContent)); writeErr != nil {
		slog.Warn("document cache write failed", "error", writeErr)
	}

	return &DocumentResult{
		HTML:        htmlContent,
		ContentHash: contentHash,
		Entities:    entities,
	}, nil
}

// computeDocumentHash computes a content hash for cache validation.
// Uses the entry entity for hashing. Returns the entities and their hash.
func (w *Workspace) computeDocumentHash(entryID string) ([]*entity.Entity, string, error) {
	e, err := w.Store().GetEntity(context.Background(), entryID)
	if err != nil {
		return nil, "", fmt.Errorf("entity %q not found", entryID)
	}
	entities := []*entity.Entity{e}
	return entities, hashEntities(entities), nil
}

// hashEntities computes a FNV-64a hash of the given entities' content.
// FNV is a fast non-cryptographic hash suitable for cache keys.
func hashEntities(entities []*entity.Entity) string {
	h := fnv.New64a()

	// Sort entities by ID for deterministic hashing
	sorted := make([]*entity.Entity, len(entities))
	copy(sorted, entities)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].ID < sorted[j].ID
	})

	for _, e := range sorted {
		// Hash ID, type, properties, and content
		h.Write([]byte(e.ID))
		h.Write([]byte(e.Type))
		h.Write([]byte(e.Content))
		// Hash properties in sorted order
		propKeys := make([]string, 0, len(e.Properties))
		for k := range e.Properties {
			propKeys = append(propKeys, k)
		}
		sort.Strings(propKeys)
		for _, k := range propKeys {
			h.Write([]byte(k))
			fmt.Fprintf(h, "%v", e.Properties[k])
		}
	}

	return strconv.FormatUint(h.Sum64(), 16)
}

// executeCommand runs an external command and returns its stdout.
func (w *Workspace) executeCommand(command string, timeout time.Duration) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	cmd.Dir = w.Paths().Root

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("command failed: %w\nstderr: %s", err, stderr.String())
	}

	return stdout.String(), nil
}

// markdownToHTML converts markdown to HTML using goldmark.
func markdownToHTML(markdown string) (string, error) {
	md := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
			extension.Table,
			extension.Strikethrough,
			extension.TaskList,
		),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
			parser.WithAttribute(), // Enable {#custom-id} syntax for headings
		),
		goldmark.WithRendererOptions(
			html.WithUnsafe(), // Allow raw HTML in markdown
		),
	)

	var buf bytes.Buffer
	if err := md.Convert([]byte(markdown), &buf); err != nil {
		return "", fmt.Errorf("markdown conversion: %w", err)
	}

	result := buf.String()

	// Post-process: convert mermaid code blocks to mermaid-ready pre elements.
	result = htmlutil.ConvertMermaidBlocks(result)

	return result, nil
}

// customLinkRegex matches edit:// and create:// URLs in href attributes.
// Format: edit://type/id or create://type or create://type?params
var customLinkRegex = regexp.MustCompile(`href="(edit|create)://([^"]+)"`)

// RewriteDocumentLinks replaces edit:// and create:// URLs with actual form URLs.
// For edit links, the return URL includes the entity ID as a hash fragment for scroll preservation.
// For create links, query params are preserved and passed through to the form.
func RewriteDocumentLinks(htmlContent, returnPath string) string {
	return customLinkRegex.ReplaceAllStringFunc(htmlContent, func(match string) string {
		return rewriteDocumentLink(match, returnPath)
	})
}

// rewriteDocumentLink rewrites a single edit:// or create:// link match.
func rewriteDocumentLink(match, returnPath string) string {
	parts := customLinkRegex.FindStringSubmatch(match)
	if len(parts) != 3 {
		return match
	}

	scheme := parts[1]
	rawURL := scheme + "://" + parts[2]

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return match
	}

	entityType := parsed.Host
	if entityType == "" {
		return match
	}

	switch scheme {
	case "edit":
		return buildEditLink(entityType, parsed.Path, returnPath)
	case "create":
		return buildCreateLink(entityType, parsed.RawQuery, returnPath)
	default:
		return match
	}
}

// buildEditLink builds a form link for editing an existing entity.
// The return URL includes the entity ID as a hash fragment for scroll preservation.
func buildEditLink(entityType, path, returnPath string) string {
	entityID := strings.TrimPrefix(path, "/")
	if entityID == "" {
		return ""
	}
	// Include hash fragment in encoded return value so it's sent to the server.
	// Without encoding, browsers treat # as a page fragment and don't send it.
	returnWithHash := returnPath + "#" + strings.ToLower(entityID)
	return fmt.Sprintf(`href="/form/%s/%s?return_to=%s"`, entityType, entityID, url.QueryEscape(returnWithHash))
}

// buildCreateLink builds a form link for creating a new entity.
// Query params are preserved and passed through to the form.
func buildCreateLink(entityType, queryString, returnPath string) string {
	if queryString != "" {
		return fmt.Sprintf(`href="/form/%s?%s&return_to=%s"`, entityType, queryString, url.QueryEscape(returnPath))
	}
	return fmt.Sprintf(`href="/form/%s?return_to=%s"`, entityType, url.QueryEscape(returnPath))
}
