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

// isFormRoute reports whether the given path targets /form/:id (create)
// or /form/:id/:entityId (edit). Only form routes honor return_to, so
// that's the single decision the rewriter needs from the frontend route
// shape.
func isFormRoute(path string) bool {
	rest, ok := strings.CutPrefix(path, "/form/")
	if !ok || rest == "" {
		return false
	}
	segments := strings.Split(rest, "/")
	if len(segments) > 2 {
		return false
	}
	for _, s := range segments {
		if s == "" {
			return false
		}
	}
	return true
}

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
		// On a Lua failure the engine returns *lua.ScriptError; attach
		// the print() output we captured before it threw, then bubble
		// up unchanged so the HTTP layer can branch via errors.As.
		var se *lua.ScriptError
		if errors.As(err, &se) {
			return "", se.WithCapturedOutput(buf.Bytes())
		}
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

// anchorStartTagRegex matches the opening `<a ...>` tag so rewriteHref can
// process all its attributes as a unit. Matching the whole tag (rather than
// just the href="...") lets the rewriter:
//
//  1. Discover attributes in any order (goldmark always emits href first,
//     but authors or future pipelines may not).
//  2. Be idempotent on its own output: both the pre-existing `id="..."`
//     the rewriter planted on a prior pass AND any author-planted `id=`
//     get stripped before we (possibly) emit a fresh one. Without this,
//     rewriting `<a id="old" href="...">` twice produces two `id`
//     attributes.
//
// Group 1 is the raw attribute segment between "<a " and ">". We
// deliberately don't parse further here; attribute parsing lives in
// rewriteAnchorAttrs.
var anchorStartTagRegex = regexp.MustCompile(`<a\s+([^>]*)>`)

// attrRegex matches a single HTML attribute inside an anchor start tag.
// Accepts double-quoted, single-quoted, and unquoted values. Groups:
// (1) name; (2) double-quoted value (may be empty); (3) single-quoted
// value; (4) unquoted value. Exactly one of (2)/(3)/(4) matches per
// attribute; boolean attributes (no value) match (1) alone.
var attrRegex = regexp.MustCompile(`([a-zA-Z_:][-a-zA-Z0-9_:.]*)\s*(?:=\s*(?:"([^"]*)"|'([^']*)'|([^\s"'=<>` + "`" + `]+)))?`)

// legacySchemeRegex detects the now-unsupported edit:// and create:// schemes
// so we can emit a clear warning for users who haven't migrated yet.
var legacySchemeRegex = regexp.MustCompile(`^(edit|create)://`)

// RewriteDocumentLinks walks all href="..." attributes in rendered HTML and
// rewrites internal links so the SPA can offer a back affordance.
//
// The rewriter runs AFTER the document-render cache (see
// documentService.GetCached / Render in this package, and the call sites in
// api_v1.go). It never writes to the cache. This is load-bearing: the cache
// file is keyed on the entry entity's content hash and must NOT contain any
// `return_to=` tokens, so that two viewers requesting the same entry under
// different return_to values each get their own value rewritten in. Do not
// move this step into doRender.
//
// Behavior, by path class × returnPath presence:
//
//	| Path class                 | returnPath == ""              | returnPath != ""                      |
//	|----------------------------|-------------------------------|---------------------------------------|
//	| Form (/form/<id>[/...])    | strip return_to; emit id      | strip return_to; emit id; inject ours |
//	| Non-form internal (/...)   | strip return_to; pass through | strip return_to; inject ours          |
//	| External / mailto / anchor | passthrough unchanged         | passthrough unchanged                 |
//	| Legacy edit:// / create:// | log warning; passthrough      | log warning; passthrough              |
//
// Author-supplied `return_to` values on internal links are always stripped,
// whether or not we have a replacement: the rewriter is the single source of
// truth for the key on emitted HTML.
//
// Form routes additionally get a stable id="edit-<entityID>-<n>" or
// id="create-<form>-<n>" attribute so the SPA's document click handler can
// record a scroll-back anchor that survives title/content edits. The per-base
// counter (<n>) disambiguates multiple links to the same target within a
// single rendered document and is stable across re-renders that produce the
// same link sequence.
//
// The rewriter is idempotent: applying it twice with the same returnPath
// produces the same bytes as one pass. Applying it twice with different
// returnPaths yields the last one injected (the first is stripped, then the
// second is injected).
func RewriteDocumentLinks(htmlContent, returnPath string, log *slog.Logger) string {
	if log == nil {
		log = slog.Default()
	}
	occ := map[string]int{} // scroll-anchor id → next available suffix
	return anchorStartTagRegex.ReplaceAllStringFunc(htmlContent, func(tag string) string {
		m := anchorStartTagRegex.FindStringSubmatch(tag)
		if len(m) != 2 {
			return tag
		}
		return rewriteAnchorTag(m[1], returnPath, log, occ)
	})
}

// rewriteAnchorTag consumes the inside of an `<a …>` start tag (the part
// between `<a ` and `>`), rewrites the href per the decision table, and
// returns a re-serialized start tag. Attribute order, spacing, and quote
// style are normalised on output — browsers don't care about any of those.
//
// Behavior:
//   - Any `href="..."`/`href='...'`/`href=...` is located in the attribute
//     list. If absent, the tag is returned unchanged.
//   - Any pre-existing `id` attribute is dropped unconditionally. The
//     rewriter owns `id` on form routes (the scroll-anchor for the click
//     handler); on non-form routes no id is emitted. Dropping pre-existing
//     ids is what keeps the rewriter idempotent on its own output.
//   - All other attributes are preserved, in the order they appeared.
func rewriteAnchorTag(attrs, returnPath string, log *slog.Logger, occ map[string]int) string {
	parsed := parseAttrs(attrs)
	var href string
	hrefIdx := -1
	var out []parsedAttr
	for _, a := range parsed {
		name := strings.ToLower(a.name)
		if name == "id" {
			// Always drop pre-existing id; the rewriter owns it (see
			// docstring).
			continue
		}
		if name == "href" {
			href = a.value
			hrefIdx = len(out)
		}
		out = append(out, a)
	}
	// No href → leave the tag alone; the enclosing regex already
	// matched, but there's nothing to rewrite.
	if hrefIdx < 0 {
		return `<a ` + serialiseAttrs(parsed) + `>`
	}

	newHref, anchorID, ok := rewriteHref(href, returnPath, log, occ)
	if !ok {
		// External / mailto / anchor / legacy scheme — return the tag
		// with its attributes intact (we stripped pre-existing id
		// above, which is fine: it wasn't ours to preserve).
		return `<a ` + serialiseAttrs(out) + `>`
	}

	// Replace href value with rewritten one; prepend a fresh id when
	// the decision table called for one.
	out[hrefIdx].value = newHref
	out[hrefIdx].quoted = true
	if anchorID != "" {
		out = append([]parsedAttr{{name: "id", value: anchorID, quoted: true}}, out...)
	}
	return `<a ` + serialiseAttrs(out) + `>`
}

// parsedAttr is a single attribute on an HTML start tag, with enough
// metadata to round-trip reasonably faithfully.
type parsedAttr struct {
	name   string
	value  string
	quoted bool // true when value was parsed from a quoted literal or is being (re)serialized
	raw    string
}

// parseAttrs splits an anchor's attribute blob into ordered parsedAttr
// records. Boolean attributes (no value) and all quote styles are
// accepted; unknown junk is skipped. Attribute name case is preserved
// (callers that need case-insensitive lookup lower-case it themselves).
func parseAttrs(s string) []parsedAttr {
	matches := attrRegex.FindAllStringSubmatchIndex(s, -1)
	out := make([]parsedAttr, 0, len(matches))
	for _, m := range matches {
		get := func(i int) string {
			start, end := m[2*i], m[2*i+1]
			if start < 0 {
				return ""
			}
			return s[start:end]
		}
		name := get(1)
		if name == "" {
			continue
		}
		a := parsedAttr{name: name, raw: s[m[0]:m[1]]}
		// Group indices in m[]: 2*k = start, 2*k+1 = end. A missing
		// group has start == -1. A present-but-empty group (e.g. `""`)
		// has start == end and both >= 0 — distinguish these from
		// boolean attributes (no = sign at all).
		switch {
		case m[4] >= 0: // double-quoted, possibly empty
			a.value = get(2)
			a.quoted = true
		case m[6] >= 0: // single-quoted, possibly empty
			a.value = get(3)
			a.quoted = true
		case m[8] >= 0: // unquoted, never empty by regex definition
			a.value = get(4)
			a.quoted = true
		default:
			// boolean attribute (no = sign) — value stays "",
			// quoted stays false.
		}
		out = append(out, a)
	}
	return out
}

// serialiseAttrs renders parsedAttrs back into an HTML attribute blob
// with a single space between attributes and double-quoted values.
// Boolean attributes (quoted=false) are emitted as bare names.
func serialiseAttrs(as []parsedAttr) string {
	var b strings.Builder
	for i, a := range as {
		if i > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(a.name)
		if a.quoted {
			b.WriteString(`="`)
			b.WriteString(a.value)
			b.WriteByte('"')
		}
	}
	return b.String()
}

// rewriteHref inspects a single href value and returns the rewritten
// href, the scroll-anchor id (empty for non-form paths), and ok=true
// when the rewriter took ownership of the href. When ok=false, the
// caller should leave href intact — the link is external, a bare
// fragment, mailto:, or a legacy scheme we only warn about.
//
// occ is a per-render map tracking how many times each anchor-id base
// has been used, so duplicate form links get -0, -1, -2 suffixes.
func rewriteHref(
	href, returnPath string, log *slog.Logger, occ map[string]int,
) (newHref, anchorID string, ok bool) {
	switch {
	case href == "":
		return "", "", false
	case legacySchemeRegex.MatchString(href):
		log.Warn("document link uses removed scheme; rewrite to app-relative path", "href", href)
		return "", "", false
	case !strings.HasPrefix(href, "/"):
		// External, anchor-only (#foo), mailto:, tel:, relative — not our
		// concern.
		return "", "", false
	}

	base, existingQuery, fragment := splitHref(href)

	// Strip any pre-existing return_to on every internal path (form or
	// non-form). The rewriter is the single source of truth for this key,
	// so author-planted values are always discarded — this keeps vue-router
	// from parsing duplicates as arrays and prevents hostile values from
	// leaking into the user's URL bar.
	cleanedQuery, dropped := stripQueryKey(existingQuery, "return_to")
	if dropped {
		log.Warn("document link sets reserved key return_to; overwriting", "href", href)
	}

	// Form routes get a scroll-anchor id unconditionally so the click
	// handler has a stable target even when returnPath is empty.
	if isFormRoute(base) {
		anchorID = formAnchorID(base, occ)
	}

	// Inject return_to only when we have one to inject. An empty returnPath
	// means "the rewriter ran but no caller context was supplied" — the
	// stripped href, plus the form anchor id if applicable, is the final
	// output.
	finalQuery := cleanedQuery
	if returnPath != "" {
		if finalQuery != "" {
			finalQuery += "&"
		}
		finalQuery += "return_to=" + url.QueryEscape(returnPath)
	}

	out := base
	if finalQuery != "" {
		out += "?" + finalQuery
	}
	if fragment != "" {
		out += "#" + fragment
	}
	return out, anchorID, true
}

// formAnchorID returns a stable scroll-anchor id for a form-route path,
// incrementing the per-base counter so duplicate links get distinct ids.
//
//	/form/<name>/<entityID>  →  edit-<entityID-lowered>-<n>
//	/form/<name>             →  create-<name-lowered>-<n>
//
// The base lookup is lowercased for case-insensitive stability (entity
// ids are conventionally uppercase, but a typo "prs-bf-7hn6" in an href
// should still produce the same id).
func formAnchorID(base string, occ map[string]int) string {
	const formPrefix = "/form/"
	if !strings.HasPrefix(base, formPrefix) {
		return ""
	}
	rest := base[len(formPrefix):]
	slash := strings.Index(rest, "/")
	var key string
	if slash < 0 {
		// create form: /form/<name>
		key = "create-" + strings.ToLower(rest)
	} else {
		// edit form: /form/<name>/<entity-id>
		entityID := rest[slash+1:]
		if entityID == "" {
			return ""
		}
		key = "edit-" + strings.ToLower(entityID)
	}
	n := occ[key]
	occ[key] = n + 1
	return fmt.Sprintf("%s-%d", key, n)
}

// stripQueryKey removes every occurrence of key (and its value) from a raw
// query string, returning the cleaned query and whether anything was
// removed. Handles goldmark's HTML-entity-encoded separator (`&amp;`) in
// addition to the literal `&` so rendered HTML round-trips correctly.
func stripQueryKey(rawQuery, key string) (string, bool) {
	if rawQuery == "" {
		return "", false
	}
	// Split the query into logical pairs while tracking the separator
	// (`&` or `&amp;`) that preceded each one, so we can rejoin the
	// remaining pairs with the same encoding the author used.
	type pair struct {
		prevSep string // separator before this pair; "" for the first
		raw     string // "key" or "key=value"
	}
	var pairs []pair
	s := rawQuery
	prevSep := ""
	for s != "" {
		idx := strings.Index(s, "&")
		if idx < 0 {
			pairs = append(pairs, pair{prevSep: prevSep, raw: s})
			break
		}
		pairs = append(pairs, pair{prevSep: prevSep, raw: s[:idx]})
		if strings.HasPrefix(s[idx:], "&amp;") {
			prevSep = "&amp;"
			s = s[idx+len("&amp;"):]
		} else {
			prevSep = "&"
			s = s[idx+1:]
		}
	}

	dropped := false
	prefix := key + "="
	var out strings.Builder
	for _, p := range pairs {
		if p.raw == key || strings.HasPrefix(p.raw, prefix) {
			dropped = true
			continue
		}
		if out.Len() == 0 {
			out.WriteString(p.raw)
		} else {
			out.WriteString(p.prevSep)
			out.WriteString(p.raw)
		}
	}
	return out.String(), dropped
}

// splitHref slices an href into base path, raw query (without '?'), and
// fragment (without '#'). Missing parts come back as empty strings.
func splitHref(href string) (base, rawQuery, fragment string) {
	base = href
	if i := strings.Index(base, "#"); i >= 0 {
		fragment = base[i+1:]
		base = base[:i]
	}
	if i := strings.Index(base, "?"); i >= 0 {
		rawQuery = base[i+1:]
		base = base[:i]
	}
	return base, rawQuery, fragment
}
