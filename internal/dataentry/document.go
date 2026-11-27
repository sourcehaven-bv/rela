package dataentry

import (
	"bytes"
	"cmp"
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"io"
	"log/slog"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
	"golang.org/x/sync/singleflight"

	"gopkg.in/yaml.v3"

	"github.com/Sourcehaven-BV/rela/internal/cmdexec"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/htmlutil"
	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/principal"
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
	return !slices.Contains(segments, "")
}

// documentScriptEngine is the minimum contract documentService needs from
// script.Engine to run a Lua document renderer. Defined at the consumer
// side (per CLAUDE.md) so tests can substitute a fake and the engine
// stays decoupled from the data-entry package.
type documentScriptEngine interface {
	ExecuteDocument(ctx context.Context, path string, deps lua.WriteDeps, stdout io.Writer,
		documentID, entryID string, timeout time.Duration) error
	ExecuteListDocument(ctx context.Context, path string, deps lua.WriteDeps, stdout io.Writer,
		documentID string, lrc lua.ListRenderContext, timeout time.Duration) error
	ExecuteStandaloneDocument(ctx context.Context, path string, deps lua.WriteDeps, stdout io.Writer,
		documentID string, timeout time.Duration) error
}

// documentDeps yields the lua.WriteDeps the script engine needs. The App
// constructs these from its current metamodel snapshot, so we keep the
// dependency as a function to avoid stale deps after reload.
type documentDepsFunc func() lua.WriteDeps

// documentElevation is the raw read capability plus its audit recorder,
// handed to a render that declares allow_acl_bypass (TKT-Y3JVFK). It mirrors
// script.ReadElevation, declared here at the consumer per CLAUDE.md
// "interfaces at the call site" — documentService needs exactly these two
// fields and must not depend on the script package's bundle type.
type documentElevation struct {
	Reader   lua.ElevatedReader
	Recorder lua.ElevationRecorder
}

// documentElevationFunc resolves the elevation bundle per render. A func
// rather than a value so test builders can rebind it after construction,
// matching documentDepsFunc.
type documentElevationFunc func() documentElevation

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
	// Command is the external render command as an argument array, run
	// directly with no shell. The single placeholder is {in}: a temp file
	// holding the entry entity's markdown (frontmatter included, so the id
	// travels in the file rather than on the command line — TKT-QGHNVA).
	// Mutually exclusive with Script.
	Command []string
	// Script is a relative path under scripts/ to a Lua file. When set,
	// the renderer runs the Lua script via script.Engine.ExecuteDocument
	// and captures its stdout as markdown. Mutually exclusive with Command.
	Script string
	// Timeout is the render timeout. Defaults to 30s. Applies to both
	// renderers.
	Timeout time.Duration
	// Elevated, when true, grants this render's Lua an elevated READER, so
	// the script may call rela.bypass_acl(fn) and read entities the request
	// principal cannot see (TKT-Y3JVFK). Reads only: no elevated Mutator is
	// ever supplied here, so the admin handle carries no write methods.
	//
	// Per-render rather than part of the shared luaDeps bundle: elevation is
	// a property of THIS document's declaration, and putting it in the shared
	// bundle would hand it to every render.
	//
	// The caller must have passed authorizeElevatedDocument first. Like
	// `permission:`, this struct carries the decision's INPUT, not the
	// decision — the renderer makes no ACL choice of its own.
	Elevated bool

	// Capabilities is the document's declared ambient capability grant
	// (TKT-YH52OM). Per-render for the same reason Elevated is: the grant
	// belongs to THIS document's declaration, and putting it in the shared
	// luaDeps bundle would hand it to every render.
	//
	// Zero value grants nothing, which is what an export or a document with no
	// `capabilities:` block gets.
	Capabilities lua.Capabilities
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
	store store.Store
	state state.KV
	// projectRoot is retained for script rendering and diagnostics. It is NOT
	// the working directory of a `command:` renderer any more: cmdexec runs
	// commands without setting cwd (and sandboxes them), so a relative program
	// path such as ["render.sh"] no longer resolves against the project root.
	// Operator configs must name the program on PATH or by absolute path — see
	// the migration note in docs/data-entry.md.
	projectRoot  string
	scriptEngine documentScriptEngine
	luaDeps      documentDepsFunc
	// elevation supplies the raw read capability for documents that declare
	// allow_acl_bypass. Nil when the wiring site granted none, in which case
	// an elevated document renders WITHOUT bypass_acl rather than failing —
	// lua raises per-method if the script calls it, naming the missing
	// capability (deps.go: nil reader is a DENY, not a fallback).
	elevation documentElevationFunc
	group     singleflight.Group
}

// newDocumentService builds a documentService. scriptEngine and luaDeps
// may be nil in tests that only exercise the command: path.
func newDocumentService(st store.Store, kv state.KV, projectRoot string,
	engine documentScriptEngine, deps documentDepsFunc,
	elevation documentElevationFunc) *documentService {
	return &documentService{
		store:        st,
		state:        kv,
		projectRoot:  projectRoot,
		scriptEngine: engine,
		luaDeps:      deps,
		elevation:    elevation,
	}
}

// elevatedDeps returns deps carrying the elevated READ capability when cfg
// declares it, and deps unchanged otherwise. It never sets ElevatedManager, so
// the script cannot write past the ACL; ordinary writes bounded by the caller's
// ACL remain available (TKT-PX5YL7). The recorder travels with the reader so an
// elevated read leaves the same `acl-bypass-read` audit row a cascade one does.
// The ACL decision was made by the caller (authorizeElevatedDocument).
func (s *documentService) elevatedDeps(cfg documentRenderConfig) lua.WriteDeps {
	deps := s.luaDeps()
	deps.Capabilities = cfg.Capabilities
	if !cfg.Elevated || s.elevation == nil {
		return deps
	}
	el := s.elevation()
	deps.ElevatedReader = el.Reader
	deps.ElevationRecorder = el.Recorder
	return deps
}

// GetCached returns a cached document if available and still valid.
// Returns nil if the cache is missing, stale, or on any error.
//
// Script renders do NOT populate this cache and callers should not read
// it for script: docs (see Render); a stale command:-era file at the
// same path would otherwise shadow the Lua render.
func (s *documentService) GetCached(ctx context.Context, entryID string) *DocumentResult {
	entities, contentHash, err := s.computeDocumentHash(ctx, entryID)
	if err != nil {
		return nil
	}

	cacheFile := fmt.Sprintf("%s/%s-%s.html", docCacheSubdir, entryID, contentHash)
	cachedHTML, _ := s.state.Get(ctx, cacheFile)
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
//
// The singleflight key includes the config id and the principal, so two
// configs or two callers never share one render.
func (s *documentService) Render(
	ctx context.Context, entryID string, cfg documentRenderConfig,
) (*DocumentResult, error) {
	entities, contentHash, err := s.computeDocumentHash(ctx, entryID)
	if err != nil {
		return nil, fmt.Errorf("computing document hash: %w", err)
	}

	cacheFile := fmt.Sprintf("%s/%s-%s.html", docCacheSubdir, entryID, contentHash)

	p := principal.From(ctx)
	sfKey := entryID + "|" + cfg.ConfigID + "|" + p.User + "|" + p.Tool
	if at, past := asOfFromContext(ctx); past {
		sfKey += "|" + at.UTC().Format(time.RFC3339Nano)
	}
	result, err, _ := s.group.Do(sfKey, func() (any, error) {
		return s.doRender(ctx, entryID, cfg, entities, contentHash, cacheFile)
	})
	if err != nil {
		return nil, err
	}

	docResult, _ := result.(*DocumentResult)
	return docResult, nil
}

// RenderMarkdown renders a document to its raw markdown (the pre-HTML step),
// dispatching on Script vs. Command exactly like doRender. Used by view export,
// which feeds the markdown to a format transform rather than converting it to
// HTML. It does NOT cache (export output is per-request) and does NOT convert to
// HTML. Callers MUST gate the read before invoking this (see handleV1ExportEntity
// / handleV1Documents) — RenderMarkdown performs no ACL decision of its own.
func (s *documentService) RenderMarkdown(
	ctx context.Context, entryID string, cfg documentRenderConfig,
) (string, error) {
	if cfg.Script != "" {
		return s.renderScript(ctx, entryID, cfg)
	}
	return s.renderCommand(ctx, entryID, cfg)
}

// RenderListMarkdown renders a LIST export override (`lists.<id>.export_render`)
// to markdown for a view export transform. It is script-only: a `command:`
// renderer needs an entry entity as {in}, and a list has none, so it fails
// closed. It does not hash, cache or singleflight: a correct dedupe key would
// need the caller's ACL scope, and a wrong one is a cross-principal leak.
// Callers MUST have resolved the rows through the ACL read path; this makes no
// ACL decision of its own.
func (s *documentService) RenderListMarkdown(
	ctx context.Context, cfg documentRenderConfig, lrc lua.ListRenderContext,
) (string, error) {
	if len(cfg.Command) > 0 {
		return "", errors.New("list export render must be a script, not a command")
	}
	if s.scriptEngine == nil || s.luaDeps == nil {
		return "", errors.New("script rendering not available (engine or deps not wired)")
	}
	var buf bytes.Buffer
	if err := s.scriptEngine.ExecuteListDocument(ctx, cfg.Script, s.elevatedDeps(cfg), &buf,
		cfg.ConfigID, lrc, cfg.Timeout); err != nil {
		var se *lua.ScriptError
		if errors.As(err, &se) {
			return "", se.AttachCapturedOutput(buf.Bytes())
		}
		return "", fmt.Errorf("list script render: %w", err)
	}
	return buf.String(), nil
}

// RenderStandalone renders a STANDALONE document (no `entity_type:`) to HTML.
// It returns a string, not a *DocumentResult, because a standalone document has
// no entry entity whose dependency footprint could fill that struct. Like
// RenderStandaloneMarkdown it is script-only and does not hash, cache or
// singleflight, since a dedupe key would need the caller's full ACL scope.
// Callers MUST apply the document's `permission:` gate first; Lua reads stay
// ACL-bound regardless.
func (s *documentService) RenderStandalone(
	ctx context.Context, cfg documentRenderConfig,
) (string, error) {
	markdown, err := s.RenderStandaloneMarkdown(ctx, cfg)
	if err != nil {
		return "", err
	}

	htmlContent, err := markdownToHTML(markdown)
	if err != nil {
		return "", fmt.Errorf("markdown conversion: %w", err)
	}
	return htmlContent, nil
}

// RenderStandaloneMarkdown renders a standalone document to its raw markdown;
// [documentService.RenderStandalone] wraps it, so both share one render path.
// Every guard lives here: a `command:` renderer is refused (fail-closed, there
// is no entry entity for {in}), and elevatedDeps(cfg) carries both the
// elevation grant and the declared capabilities. It does not hash, cache or
// singleflight. Callers MUST apply the document's `permission:` and elevation
// gates first; it makes no ACL decision of its own.
func (s *documentService) RenderStandaloneMarkdown(
	ctx context.Context, cfg documentRenderConfig,
) (string, error) {
	if len(cfg.Command) > 0 {
		return "", errors.New("a document without an entity_type must use a script renderer, not a command")
	}
	if s.scriptEngine == nil || s.luaDeps == nil {
		return "", errors.New("script rendering not available (engine or deps not wired)")
	}

	var buf bytes.Buffer
	if err := s.scriptEngine.ExecuteStandaloneDocument(ctx, cfg.Script, s.elevatedDeps(cfg), &buf,
		cfg.ConfigID, cfg.Timeout); err != nil {
		var se *lua.ScriptError
		if errors.As(err, &se) {
			return "", se.AttachCapturedOutput(buf.Bytes())
		}
		return "", fmt.Errorf("standalone script render: %w", err)
	}
	return buf.String(), nil
}

// RenderDocumentMarkdown renders EITHER document kind to markdown, dispatching
// on entryID: anchored to ExecuteDocument, standalone to
// ExecuteStandaloneDocument, so rela.document.entry_id is nil rather than ""
// for a standalone script. The anchored branch refuses a `command:` renderer as
// a fail-closed backstop, because RenderMarkdown does not. Callers MUST have run
// the document's gate chain first; this makes no ACL decision of its own.
func (s *documentService) RenderDocumentMarkdown(
	ctx context.Context, entryID string, cfg documentRenderConfig,
) (string, error) {
	if len(cfg.Command) > 0 {
		return "", errors.New("document export requires a script renderer, not a command")
	}
	if entryID == "" {
		return s.RenderStandaloneMarkdown(ctx, cfg)
	}
	return s.RenderMarkdown(ctx, entryID, cfg)
}

// doRender performs the actual rendering work. Dispatches on Script vs.
// Command — these are mutually exclusive at config load (see
// dataentryconfig.validateDocuments) so exactly one branch fires.
func (s *documentService) doRender(
	ctx context.Context, entryID string, cfg documentRenderConfig,
	entities []*entity.Entity, contentHash, cacheFile string,
) (*DocumentResult, error) {
	var markdown string
	var err error
	if cfg.Script != "" {
		markdown, err = s.renderScript(ctx, entryID, cfg)
	} else {
		markdown, err = s.renderCommand(ctx, entryID, cfg)
	}
	if err != nil {
		return nil, err
	}

	htmlContent, err := markdownToHTML(markdown)
	if err != nil {
		return nil, fmt.Errorf("markdown conversion: %w", err)
	}

	if _, past := asOfFromContext(ctx); cfg.Script == "" && !past {
		if writeErr := s.state.Put(ctx, cacheFile, []byte(htmlContent)); writeErr != nil {
			slog.Warn("document cache write failed", "error", writeErr)
		}
	}

	return &DocumentResult{
		HTML:        htmlContent,
		ContentHash: contentHash,
		Entities:    entities,
	}, nil
}

// renderCommand invokes the external render command and returns its stdout as
// markdown. The entry entity is serialized to markdown and handed to the command
// as the {in} temp file. Nothing request-derived reaches the argument vector:
// the command array is operator config and the only substitution is {in}, a
// runner-owned temp path. A renderer that needs the id reads it from the file's
// frontmatter `id:`.
func (s *documentService) renderCommand(ctx context.Context, entryID string, cfg documentRenderConfig) (string, error) {
	e, err := s.loadEntry(ctx, entryID)
	if err != nil {
		return "", fmt.Errorf("load entity %q for command render: %w", entryID, err)
	}

	in, err := renderEntityMarkdown(e)
	if err != nil {
		return "", fmt.Errorf("serialize entity %q for command render: %w", entryID, err)
	}

	return s.executeCommand(ctx, cfg.Command, []byte(in), cfg.Timeout)
}

// renderEntityMarkdown serializes an entity to the markdown shape the store
// persists: YAML frontmatter followed by the body.
//
// The `id` key is what makes a command-line placeholder unnecessary, so it is
// written unconditionally and asserted by TestRenderEntityMarkdown_CarriesID.
// `id` and `type` lead, then the remaining properties in a stable order so a
// renderer sees deterministic input.
//
// This builds the frontmatter with yaml directly rather than reusing
// internal/markdown: dataentry must not depend on that package (arch-lint), and
// what a renderer needs is the frontmatter contract, not the store's exact
// byte-for-byte formatting.
func renderEntityMarkdown(e *entity.Entity) (string, error) {
	mapping := &yaml.Node{Kind: yaml.MappingNode}
	appendPair := func(k string, v any) error {
		key := &yaml.Node{Kind: yaml.ScalarNode, Value: k}
		val := &yaml.Node{}
		if err := val.Encode(v); err != nil {
			return fmt.Errorf("encode %q: %w", k, err)
		}
		mapping.Content = append(mapping.Content, key, val)
		return nil
	}

	if err := appendPair("id", e.ID); err != nil {
		return "", err
	}
	if err := appendPair("type", e.Type); err != nil {
		return "", err
	}

	rest := make([]string, 0, len(e.Properties))
	for k := range e.Properties {
		if k != "id" && k != "type" {
			rest = append(rest, k)
		}
	}
	slices.Sort(rest)
	for _, k := range rest {
		if err := appendPair(k, e.Properties[k]); err != nil {
			return "", err
		}
	}

	fm, err := yaml.Marshal(mapping)
	if err != nil {
		return "", fmt.Errorf("marshal frontmatter: %w", err)
	}

	var b strings.Builder
	b.WriteString("---\n")
	b.Write(fm)
	b.WriteString("---\n")
	if e.Content != "" {
		b.WriteString("\n")
		b.WriteString(e.Content)
		if !strings.HasSuffix(e.Content, "\n") {
			b.WriteString("\n")
		}
	}
	return b.String(), nil
}

// renderScript executes a Lua document script and returns its captured
// stdout as markdown.
func (s *documentService) renderScript(
	ctx context.Context, entryID string, cfg documentRenderConfig,
) (string, error) {
	if s.scriptEngine == nil || s.luaDeps == nil {
		return "", errors.New("script rendering not available (engine or deps not wired)")
	}
	var buf bytes.Buffer
	if err := s.scriptEngine.ExecuteDocument(ctx, cfg.Script, s.elevatedDeps(cfg), &buf,
		cfg.ConfigID, entryID, cfg.Timeout); err != nil {
		var se *lua.ScriptError
		if errors.As(err, &se) {
			return "", se.AttachCapturedOutput(buf.Bytes())
		}
		return "", fmt.Errorf("script render: %w", err)
	}
	return buf.String(), nil
}

// computeDocumentHash computes a content hash for cache validation.
// Uses the entry entity for hashing. Returns the entities and their hash.
func (s *documentService) computeDocumentHash(ctx context.Context, entryID string) ([]*entity.Entity, string, error) {
	e, err := s.loadEntry(ctx, entryID)
	if err != nil {
		return nil, "", fmt.Errorf("load entity %q: %w", entryID, err)
	}
	entities := []*entity.Entity{e}
	return entities, hashEntities(entities), nil
}

// loadEntry reads the entry entity by its ADDRESS (`ID` or `ID@face`).
//
// The entry id is an address because the document route renders the row its
// gates cleared, which for a faced type is never the zero face. GetEntity takes
// a bare id and answers not-found for an address on every backend, so reading
// through it turned each faced render into a 500 (BUG-8J3LSB). This performs
// no ACL decision: the caller has already gated the address.
func (s *documentService) loadEntry(ctx context.Context, addr string) (*entity.Entity, error) {
	id, face, err := entity.ParseStateRef(addr)
	if err != nil {
		return nil, fmt.Errorf("parse entry address %q: %w", addr, err)
	}
	return s.store.GetEntity(ctx, entity.Ref{ID: id, Face: face})
}

// hashEntities computes a FNV-64a hash of the given entities' content.
// FNV is a fast non-cryptographic hash suitable for cache keys.
func hashEntities(entities []*entity.Entity) string {
	h := fnv.New64a()

	sorted := make([]*entity.Entity, len(entities))
	copy(sorted, entities)
	slices.SortFunc(sorted, func(a, b *entity.Entity) int { return cmp.Compare(a.ID, b.ID) })

	for _, e := range sorted {
		h.Write([]byte(e.ID))
		h.Write([]byte(e.Type))
		h.Write([]byte(e.Content))
		propKeys := make([]string, 0, len(e.Properties))
		for k := range e.Properties {
			propKeys = append(propKeys, k)
		}
		slices.Sort(propKeys)
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

// maxCommandOutputBytes caps a document renderer's stdout. The rendered
// markdown is held in memory and then converted to HTML, so an unbounded
// converter could exhaust the host; cmdexec enforces the cap as it reads.
const maxCommandOutputBytes = 32 << 20 // 32 MiB

// executeCommand runs an external render command over `in` and returns its
// stdout.
//
// It delegates to internal/cmdexec, the reviewed "bytes in → bytes out" core
// already used by attachment processing and view export: argv array (never a
// shell string), {in}/{out} templated to runner-owned temp paths, a timeout, an
// output cap, a no-network sandbox, and rlimits. There is deliberately no
// `sh -c` here any more — see renderCommand.
func (s *documentService) executeCommand(
	ctx context.Context, command []string, in []byte, timeout time.Duration,
) (string, error) {
	if len(command) == 0 {
		return "", errors.New("document command is empty")
	}
	if timeout <= 0 {
		timeout = commandDefaultTimeout
	}

	runner, err := cmdexec.New(timeout, maxCommandOutputBytes, cmdexec.WithPurpose(cmdexec.PurposeTransform))
	if err != nil {
		return "", fmt.Errorf("build document command runner: %w", err)
	}

	out, _, err := runner.Run(ctx, command, in, true)
	if err != nil {
		return "", fmt.Errorf("command failed: %w", err)
	}
	return string(out), nil
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
			parser.WithAttribute(),
		),
		goldmark.WithRendererOptions(
			html.WithUnsafe(),
		),
	)

	var buf bytes.Buffer
	if err := md.Convert([]byte(markdown), &buf); err != nil {
		return "", fmt.Errorf("markdown conversion: %w", err)
	}

	result := buf.String()

	result = htmlutil.ConvertDiagramBlocks(result)

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

// RewriteDocumentLinks rewrites internal href links in rendered HTML so the SPA
// can offer a back affordance. It runs AFTER the document-render cache and never
// writes to it, so cached HTML never holds a `return_to=` token. Author-supplied
// `return_to` values on internal links are always stripped, and ours is injected
// when returnPath is set. Form routes also get a stable id="edit-<id>-<n>" or
// id="create-<form>-<n>" scroll anchor. External, mailto and anchor links pass
// through; legacy edit:// and create:// links are logged and passed through.
// The rewrite is idempotent for a given returnPath.
func RewriteDocumentLinks(htmlContent, returnPath string, log *slog.Logger) string {
	if log == nil {
		log = slog.Default()
	}
	occ := map[string]int{}
	return anchorStartTagRegex.ReplaceAllStringFunc(htmlContent, func(tag string) string {
		m := anchorStartTagRegex.FindStringSubmatch(tag)
		if len(m) != 2 {
			return tag
		}
		return rewriteAnchorTag(m[1], returnPath, log, occ)
	})
}

// rewriteAnchorTag rewrites the inside of an `<a …>` start tag per
// RewriteDocumentLinks and returns a re-serialized tag with normalized quoting.
// A tag without href is returned unchanged. Any existing `id` is dropped, since
// the rewriter owns `id` on form routes; that keeps it idempotent on its own
// output. Other attributes keep their order.
func rewriteAnchorTag(attrs, returnPath string, log *slog.Logger, occ map[string]int) string {
	parsed := parseAttrs(attrs)
	var href string
	hrefIdx := -1
	var out []parsedAttr
	for _, a := range parsed {
		name := strings.ToLower(a.name)
		if name == "id" {
			continue
		}
		if name == "href" {
			href = a.value
			hrefIdx = len(out)
		}
		out = append(out, a)
	}
	if hrefIdx < 0 {
		return `<a ` + serialiseAttrs(parsed) + `>`
	}

	newHref, anchorID, ok := rewriteHref(href, returnPath, log, occ)
	if !ok {
		return `<a ` + serialiseAttrs(out) + `>`
	}

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
		const unquotedValueGroup = 4
		switch {
		case m[4] >= 0:
			a.value = get(2)
			a.quoted = true
		case m[6] >= 0:
			a.value = get(3)
			a.quoted = true
		case m[8] >= 0:
			a.value = get(unquotedValueGroup)
			a.quoted = true
		default:
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
//
// Any existing return_to is always stripped from internal paths.
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
		return "", "", false
	}

	base, existingQuery, fragment := splitHref(href)

	cleanedQuery, dropped := stripQueryKey(existingQuery, "return_to")
	if dropped {
		log.Warn("document link sets reserved key return_to; overwriting", "href", href)
	}

	if isFormRoute(base) {
		anchorID = formAnchorID(base, occ)
	}

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
	_, after, ok := strings.Cut(rest, "/")
	var key string
	if !ok {
		key = "create-" + strings.ToLower(rest)
	} else {
		entityID := after
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
	type pair struct {
		prevSep string
		raw     string
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
