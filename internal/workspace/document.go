package workspace

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"os/exec"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
	"golang.org/x/sync/singleflight"

	"github.com/Sourcehaven-BV/rela/internal/htmlutil"
	"github.com/Sourcehaven-BV/rela/internal/model"
	"github.com/Sourcehaven-BV/rela/internal/views"
)

// docCacheDir is the subdirectory under .rela/ for document cache files.
const docCacheDir = "documents"

// DocumentConfig defines how to render a document from an entry entity.
type DocumentConfig struct {
	// View is the view name from views.yaml used to gather entities.
	View string
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
	// CacheHit indicates whether the result came from cache.
	CacheHit bool
	// Entities contains all entities involved in the document (for dependency tracking).
	Entities []*model.Entity
}

// docRenderGroup dedupes concurrent render requests for the same entry.
var docRenderGroup singleflight.Group

// RenderDocument renders a document for the given entry ID using the provided config.
// It handles disk-based caching, request deduplication, and external command execution.
func (w *Workspace) RenderDocument(entryID string, cfg DocumentConfig) (*DocumentResult, error) {
	// Compute content hash for cache validation
	entities, contentHash, err := w.computeDocumentHash(entryID, cfg.View)
	if err != nil {
		return nil, fmt.Errorf("computing document hash: %w", err)
	}

	// Check disk cache
	cacheFile := fmt.Sprintf("%s/%s-%s.html", docCacheDir, entryID, contentHash)
	if cachedHTML, cacheErr := w.ReadCacheFile(cacheFile); cacheErr == nil {
		return &DocumentResult{
			HTML:        string(cachedHTML),
			ContentHash: contentHash,
			CacheHit:    true,
			Entities:    entities,
		}, nil
	}

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
	entryID string, cfg DocumentConfig, entities []*model.Entity, contentHash, cacheFile string,
) (*DocumentResult, error) {
	// Build command with ID substitution
	command := cfg.Command
	command = strings.ReplaceAll(command, "{id}", entryID)
	command = strings.ReplaceAll(command, "{id_lower}", strings.ToLower(entryID))

	// Execute render command (caller should set default timeout if needed)
	markdown, err := w.executeCommand(command, cfg.Timeout)
	if err != nil {
		return nil, err
	}

	// Convert markdown to HTML
	htmlContent, err := markdownToHTML(markdown)
	if err != nil {
		return nil, fmt.Errorf("markdown conversion: %w", err)
	}

	// Cache the result to disk (ignore write errors, cache is optional)
	_ = w.WriteCacheFile(cacheFile, []byte(htmlContent))

	return &DocumentResult{
		HTML:        htmlContent,
		ContentHash: contentHash,
		CacheHit:    false,
		Entities:    entities,
	}, nil
}

// computeDocumentHash computes a content hash for cache validation.
// It executes the view to get all involved entities and hashes their content.
// Returns the entities and their hash.
func (w *Workspace) computeDocumentHash(entryID, viewName string) ([]*model.Entity, string, error) {
	// Load view definition
	viewsFile, err := w.LoadViews()
	if err != nil {
		// If views.yaml doesn't exist, fall back to hashing just the entry entity
		entry, ok := w.graph.GetNode(entryID)
		if !ok {
			return nil, "", fmt.Errorf("entry %s not found", entryID)
		}
		entities := []*model.Entity{entry}
		return entities, hashEntities(entities), nil
	}

	viewDef, ok := viewsFile.Views[viewName]
	if !ok {
		// View not found, fall back to hashing just the entry entity
		entry, ok := w.graph.GetNode(entryID)
		if !ok {
			return nil, "", fmt.Errorf("entry %s not found", entryID)
		}
		entities := []*model.Entity{entry}
		return entities, hashEntities(entities), nil
	}

	// Execute view to get all entities
	engine := views.NewEngine(w.graph, w.meta)
	result, err := engine.Execute(viewDef, entryID)
	if err != nil {
		return nil, "", fmt.Errorf("executing view: %w", err)
	}

	// Collect all entities from the view result
	var entities []*model.Entity
	if result.Entry != nil {
		entities = append(entities, result.Entry)
	}
	for _, collection := range result.Collections {
		entities = append(entities, collection...)
	}

	return entities, hashEntities(entities), nil
}

// hashEntities computes a SHA256 hash of the given entities' content.
func hashEntities(entities []*model.Entity) string {
	h := sha256.New()

	// Sort entities by ID for deterministic hashing
	sorted := make([]*model.Entity, len(entities))
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

	// Use first 16 chars (64 bits) for cache filenames - sufficient for collision avoidance
	return hex.EncodeToString(h.Sum(nil))[:16]
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

// editLinkRegex matches edit:// URLs in href attributes.
var editLinkRegex = regexp.MustCompile(`href="edit://([^/]+)/([^"]+)"`)

// RewriteEditLinks replaces edit:// URLs with actual form URLs.
// The returnPath is the path to return to after editing.
// The return URL includes the entity ID as a hash fragment for scroll preservation.
func RewriteEditLinks(htmlContent, returnPath string) string {
	return editLinkRegex.ReplaceAllStringFunc(htmlContent, func(match string) string {
		parts := editLinkRegex.FindStringSubmatch(match)
		if len(parts) != 3 {
			return match
		}
		entityType := parts[1]
		entityID := parts[2]
		// URL format: /form/{type}/{id}?return={encoded-path-with-hash}
		// The hash fragment is included in the encoded return value so it's sent to the server.
		// Without encoding, browsers treat # as a page fragment and don't send it.
		returnWithHash := returnPath + "#" + strings.ToLower(entityID)
		return fmt.Sprintf(`href="/form/%s/%s?return=%s"`, entityType, entityID, url.QueryEscape(returnWithHash))
	})
}

// createLinkRegex matches create:// URLs in href attributes.
// Format: create://entity_type or create://entity_type?prop.name=value&rel.type=id
var createLinkRegex = regexp.MustCompile(`href="create://([^"?]+)(\?[^"]*)?"`)

// RewriteCreateLinks replaces create:// URLs with actual form URLs.
// The returnPath is the path to return to after creating.
// Query params are preserved and passed through to the form.
func RewriteCreateLinks(htmlContent, returnPath string) string {
	return createLinkRegex.ReplaceAllStringFunc(htmlContent, func(match string) string {
		parts := createLinkRegex.FindStringSubmatch(match)
		if len(parts) < 2 {
			return match
		}
		entityType := parts[1]
		queryString := ""
		if len(parts) > 2 && parts[2] != "" {
			// Remove leading ? and keep the rest
			queryString = parts[2][1:]
		}
		// Build URL: /form/{type}?{params}&return={encoded-path}
		var result string
		if queryString != "" {
			result = fmt.Sprintf(`href="/form/%s?%s&return=%s"`, entityType, queryString, url.QueryEscape(returnPath))
		} else {
			result = fmt.Sprintf(`href="/form/%s?return=%s"`, entityType, url.QueryEscape(returnPath))
		}
		return result
	})
}
