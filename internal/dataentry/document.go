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
	"regexp"
	"slices"
	"sort"
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
	Reader   lua.EntityReader
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
// declares it, and deps unchanged otherwise (TKT-Y3JVFK).
//
// Only ElevatedReader and ElevationRecorder are set — never ElevatedManager —
// so lua.newElevatedHandle omits the write methods entirely and the script
// cannot write PAST THE ACL.
//
// That is NOT the same as "a render cannot mutate", and the difference matters.
// A document renders on a WriterRuntime (script.runDocumentScript ->
// NewWriterRuntime -> lua.NewWriter), and registerBindings has no isDocument
// guard, so ordinary rela.create_entity / update_entity / delete_entity /
// write_file ARE present and callable in a document script — bounded by the
// caller's own ACL. That is pre-existing and tracked in TKT-PX5YL7; withholding
// the elevated Mutator here narrows elevation, it does not make the render
// read-only.
//
// The recorder travels with the reader so an elevated document read leaves the
// same `acl-bypass-read` audit row a cascade one does. Granting the capability
// without the trace is the exact gap TKT-ACSBSA closed; it must not reopen on
// a new surface.
//
// The ACL decision was made by the caller (authorizeElevatedDocument). This
// applies it; it does not re-decide.
func (s *documentService) elevatedDeps(cfg documentRenderConfig) lua.WriteDeps {
	deps := s.luaDeps()
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
func (s *documentService) Render(
	ctx context.Context, entryID string, cfg documentRenderConfig,
) (*DocumentResult, error) {
	entities, contentHash, err := s.computeDocumentHash(ctx, entryID)
	if err != nil {
		return nil, fmt.Errorf("computing document hash: %w", err)
	}

	cacheFile := fmt.Sprintf("%s/%s-%s.html", docCacheSubdir, entryID, contentHash)

	// Singleflight key must include ConfigID: if two documents (different
	// configs) target the same entry, they are distinct renders and must
	// not collapse onto one another's HTML (RR-4QSBN). It must ALSO include
	// the principal (RR-2QSGLU): renders run under the caller's identity
	// since TKT-L9Q669 (rela.principal, ctx cancellation), so collapsing two
	// principals' renders would hand one caller output produced under the
	// other's identity — and once script reads are principal-scoped
	// (TKT-ZF2DTV) it would be a cross-principal data leak, not just an
	// attribution bug.
	p := principal.From(ctx)
	sfKey := entryID + "|" + cfg.ConfigID + "|" + p.User + "|" + p.Tool
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
// to markdown, for the same reason RenderMarkdown exists on the entity side:
// view export feeds the markdown to a format transform rather than converting
// it to HTML.
//
// Script-only: a `command:` renderer is handed the entry entity as {in}, of an
// entry entity and a list has none. No caller sets Command today; the guard
// below is a fail-closed assertion so a future one gets an error instead of a
// silently entry-less substitution.
//
// It deliberately does NOT hash, cache, or singleflight, and none of those are
// oversights to be "optimized" back in later:
//   - computeDocumentHash loads ONE entity by id; a list has no entry entity,
//     so there is nothing for it to hash.
//   - The disk cache is keyed on that entry hash and holds HTML; list export
//     produces per-request markdown.
//   - A correct singleflight key would need the full resolved query AND the
//     caller's ACL scope, because two principals' row sets legitimately differ.
//     Getting that key wrong collapses two callers onto one render — a
//     cross-principal leak, the RR-2QSGLU hazard with more to lose. Not
//     deduping is the safe default.
//
// Callers MUST have resolved the rows through the ACL read path before calling
// this — it makes no ACL decision of its own.
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
	if err := s.scriptEngine.ExecuteListDocument(ctx, cfg.Script, s.luaDeps(), &buf,
		cfg.ConfigID, lrc, cfg.Timeout); err != nil {
		// Same shaping as renderScript: attach the output captured before the
		// script threw, then bubble up unchanged so the HTTP layer can branch
		// via errors.As.
		var se *lua.ScriptError
		if errors.As(err, &se) {
			return "", se.AttachCapturedOutput(buf.Bytes())
		}
		return "", fmt.Errorf("list script render: %w", err)
	}
	return buf.String(), nil
}

// RenderStandalone renders a STANDALONE document — one declared without an
// `entity_type:`, whose content is company-wide rather than about one entity
// (TKT-M1AX6P). Returns HTML.
//
// It returns a plain string rather than a *DocumentResult because two of that
// struct's three fields — ContentHash and Entities — describe an entry
// entity's dependency footprint, and a standalone document has no entry
// entity. Returning a struct whose fields this constructor can never populate
// would invite a caller to wire the SSE live-reload subscription to an always-
// empty Entities and ship a document that silently never refreshes. The
// narrower type makes that a compile error instead of a comment nobody reads.
// (Standalone documents do refresh on any entity change — the SPA's SSE feed
// is type-scoped, not keyed on this return value — but on-demand Refresh is
// the reliable path until TKT-E1FO1 lets scripts declare dependencies.)
//
// Script-only, for the same reason RenderListMarkdown is: a `command:`
// renderer is handed an entry entity as {in}, and a
// standalone document has none. Rejecting is fail-closed — the alternative is
// substituting an empty id into a shell command, which is how a renderer
// silently produces a document about the wrong thing (or nothing).
//
// Like RenderListMarkdown it deliberately does NOT hash, cache, or
// singleflight, and none of those are oversights:
//   - computeDocumentHash loads ONE entity by id; there is no entry entity to
//     hash, so there is no content hash to key a cache on.
//   - A correct singleflight key would need the caller's full ACL scope,
//     because two principals' aggregates legitimately differ. Getting that key
//     wrong collapses two callers onto one render — the RR-2QSGLU
//     cross-principal hazard. Not deduping is the safe default.
//
// Callers MUST apply the document's `permission:` gate before invoking this —
// RenderStandalone makes no ACL decision of its own. (Its Lua reads are still
// ACL-bound via lua.ReadDeps.VisibleReader, so content is gated regardless;
// the caller's gate is what keeps a denied principal from triggering the
// render at all.)
func (s *documentService) RenderStandalone(
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
		// Same shaping as renderScript/RenderListMarkdown: attach the output
		// captured before the script threw, then bubble up unchanged so the
		// HTTP layer can branch via errors.As.
		var se *lua.ScriptError
		if errors.As(err, &se) {
			return "", se.AttachCapturedOutput(buf.Bytes())
		}
		return "", fmt.Errorf("standalone script render: %w", err)
	}

	htmlContent, err := markdownToHTML(buf.String())
	if err != nil {
		return "", fmt.Errorf("markdown conversion: %w", err)
	}
	return htmlContent, nil
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

	// Disk cache is only populated for command: renders. Lua renders
	// have their own process-lifetime cache via rela.cache.memoize; the
	// disk-cache filename is renderer-agnostic (FNV of the entry entity)
	// so writing script-render output here would make a subsequent
	// command: run read stale bytes from the wrong renderer.
	if cfg.Script == "" {
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
// markdown.
//
// The entry entity is serialized to markdown and handed to the command as the
// {in} temp file. NOTHING request-derived reaches the argument vector: the
// command array is operator-authored config, and the only substitution is {in}
// → a runner-owned temp path. That is the structural fix for TKT-QGHNVA —
// previously the entry id was spliced into a string run through `sh -c`, so an
// id such as "-rf" arrived as an option flag rather than an operand, and the
// only thing standing between a stored id and that shell was input filtering.
//
// A renderer that needs the id reads it from the file: the serialized
// frontmatter always carries `id:` (see renderEntityMarkdown).
func (s *documentService) renderCommand(ctx context.Context, entryID string, cfg documentRenderConfig) (string, error) {
	e, err := s.store.GetEntity(ctx, entryID)
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
	// yaml.v3 has no MapSlice, so build a mapping Node to control key order.
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
	sort.Strings(rest)
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
		// On a Lua failure the engine returns *lua.ScriptError; attach
		// the print() output we captured before it threw, then bubble
		// up unchanged so the HTTP layer can branch via errors.As.
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
	e, err := s.store.GetEntity(ctx, entryID)
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

	// Temp files land in the OS temp dir, not the project root: the {in} file
	// is runner-owned scratch, and a project checkout may legitimately be
	// read-only.
	runner, err := cmdexec.New(timeout, maxCommandOutputBytes)
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

	// Post-process: convert diagram code blocks (mermaid, plantuml) to the
	// pre elements the SPA upgrades client-side.
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
		// Capture-group index (within attrRegex) for the unquoted value
		// alternative — the fourth value-bearing group.
		const unquotedValueGroup = 4
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
			a.value = get(unquotedValueGroup)
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
	_, after, ok := strings.Cut(rest, "/")
	var key string
	if !ok {
		// create form: /form/<name>
		key = "create-" + strings.ToLower(rest)
	} else {
		// edit form: /form/<name>/<entity-id>
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
