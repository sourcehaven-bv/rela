package dataentry

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"io"
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
	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/state"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// documentScriptEngine is the minimum contract documentService needs from
// script.Engine to run a Lua document renderer. Defined at the consumer
// side (per CLAUDE.md) so tests can substitute a fake and the engine
// stays decoupled from the data-entry package.
type documentScriptEngine interface {
	ExecuteDocument(path string, deps lua.WriteDeps, stdout io.Writer,
		documentID, entryID string, timeout time.Duration) error
}

// documentDeps yields the lua.WriteDeps the script engine needs. The App
// constructs these from its current metamodel snapshot, so we keep the
// dependency as a function to avoid stale deps after reload.
type documentDepsFunc func() lua.WriteDeps

// docCacheSubdir is the subdirectory under .rela/ for document cache files.
const docCacheSubdir = "documents"

// documentRenderConfig is the internal render configuration — the
// external config is dataentryconfig.DocumentConfig (YAML), which
// toDocumentRenderConfig converts.
type documentRenderConfig struct {
	// ConfigID is the key under `documents:` in data-entry.yaml. It is
	// the document identity seen by scripts as rela.document.id, and
	// participates in the singleflight/cache key so concurrent renders
	// of different documents against the same entry don't collapse.
	ConfigID string
	// Command is the external render command. Placeholders:
	//   {id}       - entry ID
	//   {id_lower} - lowercase entry ID
	// Mutually exclusive with Script.
	Command string
	// Script is a relative path under scripts/ to a Lua file. When set,
	// the renderer runs the Lua script via script.Engine.ExecuteDocument
	// and captures its stdout as markdown. Mutually exclusive with Command.
	Script string
	// Timeout is the render timeout. Defaults to 30s. Applies to both
	// renderers.
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

// documentService renders documents by invoking an external command or a
// Lua script and caches command-renderer results on disk keyed by an FNV
// hash of the source entities. It is safe for concurrent use: render
// requests are deduped via singleflight on (entryID, configID) so two
// documents against the same entry do not collapse onto one render.
//
// Disk cache policy: only command: renders read and write .rela/documents/.
// script: renders bypass disk cache on both sides — Lua's in-process
// rela.cache.memoize is the caching story for scripts, and reading an old
// command:-era cache file for a script: request would serve stale HTML.
type documentService struct {
	store        store.Store
	state        state.KV
	projectRoot  string
	scriptEngine documentScriptEngine
	luaDeps      documentDepsFunc
	group        singleflight.Group
}

// newDocumentService builds a documentService. scriptEngine and luaDeps
// may be nil in tests that only exercise the command: path.
func newDocumentService(st store.Store, kv state.KV, projectRoot string,
	engine documentScriptEngine, deps documentDepsFunc) *documentService {
	return &documentService{
		store:        st,
		state:        kv,
		projectRoot:  projectRoot,
		scriptEngine: engine,
		luaDeps:      deps,
	}
}

// GetCached returns a cached document if available and still valid.
// Returns nil if the cache is missing, stale, or on any error.
//
// Script renders do NOT populate this cache and callers should not read
// it for script: docs (see Render); a stale command:-era file at the
// same path would otherwise shadow the Lua render.
func (s *documentService) GetCached(entryID string) *DocumentResult {
	entities, contentHash, err := s.computeDocumentHash(entryID)
	if err != nil {
		return nil
	}

	cacheFile := fmt.Sprintf("%s/%s-%s.html", docCacheSubdir, entryID, contentHash)
	cachedHTML, _ := s.state.Get(context.Background(), cacheFile)
	if cachedHTML == nil {
		return nil
	}

	return &DocumentResult{
		HTML:        string(cachedHTML),
		ContentHash: contentHash,
		Entities:    entities,
	}
}

// Render renders a document via the configured renderer (command or
// script). Singleflight dedupes concurrent requests for the same
// (entryID, ConfigID) pair — renders of the same entry under *different*
// document configs proceed in parallel. Command renders cache to disk;
// script renders do not.
func (s *documentService) Render(entryID string, cfg documentRenderConfig) (*DocumentResult, error) {
	entities, contentHash, err := s.computeDocumentHash(entryID)
	if err != nil {
		return nil, fmt.Errorf("computing document hash: %w", err)
	}

	cacheFile := fmt.Sprintf("%s/%s-%s.html", docCacheSubdir, entryID, contentHash)

	// Singleflight key must include ConfigID: if two documents (different
	// configs) target the same entry, they are distinct renders and must
	// not collapse onto one another's HTML (RR-4QSBN).
	sfKey := entryID + "|" + cfg.ConfigID
	result, err, _ := s.group.Do(sfKey, func() (interface{}, error) {
		return s.doRender(entryID, cfg, entities, contentHash, cacheFile)
	})
	if err != nil {
		return nil, err
	}

	docResult, _ := result.(*DocumentResult)
	return docResult, nil
}

// doRender performs the actual rendering work. Dispatches on Script vs.
// Command — these are mutually exclusive at config load (see
// dataentryconfig.validateDocuments) so exactly one branch fires.
func (s *documentService) doRender(
	entryID string, cfg documentRenderConfig, entities []*entity.Entity, contentHash, cacheFile string,
) (*DocumentResult, error) {
	var markdown string
	var err error
	if cfg.Script != "" {
		markdown, err = s.renderScript(entryID, cfg)
	} else {
		markdown, err = s.renderCommand(entryID, cfg)
	}
	if err != nil {
		return nil, err
	}

	htmlContent, err := markdownToHTML(markdown)
	if err != nil {
		return nil, fmt.Errorf("markdown conversion: %w", err)
	}

	// Disk cache is only populated for command: renders. Lua renders
	// have their own process-lifetime cache via rela.cache.memoize; the
	// disk-cache filename is renderer-agnostic (FNV of the entry entity)
	// so writing script-render output here would make a subsequent
	// command: run read stale bytes from the wrong renderer.
	if cfg.Script == "" {
		if writeErr := s.state.Put(context.Background(), cacheFile, []byte(htmlContent)); writeErr != nil {
			slog.Warn("document cache write failed", "error", writeErr)
		}
	}

	return &DocumentResult{
		HTML:        htmlContent,
		ContentHash: contentHash,
		Entities:    entities,
	}, nil
}

// renderCommand invokes the external render command and returns its stdout
// as markdown. Placeholder substitution happens on the command string.
func (s *documentService) renderCommand(entryID string, cfg documentRenderConfig) (string, error) {
	command := cfg.Command
	command = strings.ReplaceAll(command, "{id}", entryID)
	command = strings.ReplaceAll(command, "{id_lower}", strings.ToLower(entryID))
	return s.executeCommand(command, cfg.Timeout)
}

// renderScript executes a Lua document script and returns its captured
// stdout as markdown.
func (s *documentService) renderScript(entryID string, cfg documentRenderConfig) (string, error) {
	if s.scriptEngine == nil || s.luaDeps == nil {
		return "", errors.New("script rendering not available (engine or deps not wired)")
	}
	var buf bytes.Buffer
	if err := s.scriptEngine.ExecuteDocument(cfg.Script, s.luaDeps(), &buf,
		cfg.ConfigID, entryID, cfg.Timeout); err != nil {
		return "", fmt.Errorf("script render: %w", err)
	}
	return buf.String(), nil
}

// computeDocumentHash computes a content hash for cache validation.
// Uses the entry entity for hashing. Returns the entities and their hash.
func (s *documentService) computeDocumentHash(entryID string) ([]*entity.Entity, string, error) {
	e, err := s.store.GetEntity(context.Background(), entryID)
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

// commandDefaultTimeout is the fallback render timeout for shell-command
// documents when data-entry.yaml omits `timeout:`. Script-backed documents
// fall back separately inside script.Engine.ExecuteDocument (via
// lua.DefaultTimeout). Keeping the default per-renderer prevents a zero
// value from producing an already-expired context.
const commandDefaultTimeout = 30 * time.Second

// executeCommand runs an external command and returns its stdout.
func (s *documentService) executeCommand(command string, timeout time.Duration) (string, error) {
	if timeout <= 0 {
		timeout = commandDefaultTimeout
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	cmd.Dir = s.projectRoot

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
