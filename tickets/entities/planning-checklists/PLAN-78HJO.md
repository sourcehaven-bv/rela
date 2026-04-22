---
id: PLAN-78HJO
type: planning-checklist
title: 'Planning: Document the documents feature and add Lua script renderer'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

In scope:

1. Add `script:` string field to `DocumentConfig` (mutually exclusive with `command:`, validated at config-load time). `entity_type:` **remains required** for every document (both renderers).
2. Extend `documentService.Render` to dispatch: `command:` → existing shell-out + disk cache (both read and write); `script:` → new Lua path, **bypasses disk cache on both read and write**. Singleflight key must include `configID` (not just `entryID`).
3. HTTP handler enforces `entity.Type == docCfg.EntityType` before rendering; mismatch → HTTP 400.
4. New `script.Engine.ExecuteDocument(path, deps, stdout, documentID, entryID, timeout)` — typed method, no variadic opts (mirrors how `ExecuteAction` wires its opts internally).
5. New Lua runtime option `WithDocumentMode(documentID, entryID string)` (parallel to existing `WithActionMode`). Sets `rela.mode = "document"` and `rela.document = {id = documentID, entry_id = entryID}`. All other contexts leave `rela.mode` and `rela.document` as `nil`.
6. `DocumentConfig.Timeout` applies to script: renders (wired as `lua.WithTimeout`), matching existing semantics for command: renders.
7. In document mode, `rela.output` writes a warning line to captured stdout and returns early, instead of emitting JSON (same pattern as action mode).
8. Documentation added to `docs-project/entities/guides/GUIDE-data-entry.md` for the full documents feature (both variants), including caveats around entry-only SSE reload, cache-namespace-per-script-path, and `html.WithUnsafe` trust boundary.
9. Example Lua doc script shipped under `prototypes/data-entry/project/scripts/docs/`.
10. `FEAT-023` updated to reflect shipped state.

Out of scope (tracked separately):

- Explicit `rela.document.depends_on(id)` for SSE dependency tracking → **TKT-E1FO1**.
- Generalizing `rela.mode` to other contexts → **TKT-CGPYW**.
- Removing the disk cache from `command:` renders (command: behavior unchanged).
- Export of rendered markdown/HTML as a file download.

**Cache namespace policy** (resolving RR-I5WME):

`rela.cache` stays namespaced by script path (not by `configID`). Intentional:
shared helper scripts (e.g. `scripts/docs/lib/entity_summary.lua`) derive their
value from caching work across all callers. Auto-namespacing by `configID` would
defeat this. Scripts that legitimately need doc-scoped keys can write them
explicitly: `rela.cache.memoize("doc:" .. rela.document.id .. ":" .. id, ...)`.
Documented as a caveat in the guide.

**Acceptance Criteria:**

Testable (executable checks):

1. **AC1 — Lua happy path.** `data-entry.yaml` with `script: scripts/docs/foo.lua` renders via Lua; stdout → markdown → HTML → `edit://`/`create://` rewritten.
*Test:* unit test in `document_test.go` using a fake `script.Engine` that writes
known markdown to its stdout buffer, asserts HTML result contains converted
content plus rewritten links.

2. **AC2 — Config validation.** Mutual exclusion of `command:`+`script:`; both set → error; neither set → error; only one set → OK. `entity_type:` remains required in all cases.
*Test:* table-driven test in `internal/dataentryconfig/validate_test.go`
covering: both set; neither set; only command; only script; script without
entity_type; command without entity_type.

3. **AC3 — Context injection in document mode.** Inside a Lua doc script: `rela.mode == "document"`, `rela.document.entry_id == <entryID>`, `rela.document.id == <configID>`.
*Test:* unit test in `internal/lua/runtime_test.go` that calls `NewWriter(...,
WithDocumentMode("release-notes", "REL-001"))`, runs a script that writes those
values to a sentinel file via `rela.write_file`, asserts file content matches.

4. **AC4 — Context absent elsewhere.** In CLI / flow / action / scheduled / validation runs, `rela.mode` and `rela.document` are `nil`.
*Test:* existing runtime tests; add assertions that `rela.mode == nil` and
`rela.document == nil` in non-document paths.

5. **AC5 — `rela.output` in document mode.** Calling `rela.output({foo=1})` writes `warning: rela.output() called in document mode; use print() to emit markdown\n` to captured stdout and does NOT emit JSON.
*Test:* `runtime_test.go` — mirror the existing `TestLuaOutputActionMode*`
pattern: build `NewWriter` with `WithDocumentMode(...)` and a `bytes.Buffer`
stdout; run `rela.output({foo=1})`; assert stdout contains the warning and does
not contain JSON. Tests directly against the runtime (does not route through
documentService) so the stdout buffer is accessible.

6. **AC6 — In-process cache works across requests.** A Lua doc script using `rela.cache.memoize("k", fn)` caches across HTTP requests within the same `rela-server` process.
*Test:* `document_test.go` — script calls `rela.cache.memoize("counter",
function() rela.write_file("counter.log", "1") end)`; render twice via
`documentService.Render`; inspect `output/counter.log` and assert it has exactly
one line (the compute fn ran once across two render calls).

7. **AC7 — Shell-command path unchanged.** Existing `command:` docs render identically to today, including disk cache at `.rela/documents/<entry>-<hash>.html`.
*Test:* existing `document_test.go` tests continue to pass; add an assertion
that `.rela/documents/` is populated after a command-based render.

8. **AC8 — Singleflight keyed on (entryID, configID).** Two concurrent renders for the same entry but different document configs must not collapse.
*Test:* `document_test.go` — table-driven: launch two goroutines rendering
different `configID`s against the same entry; each script writes a distinct
marker to its captured stdout; assert both markers appear, not just one.

9. **AC9 — Handler enforces EntityType.** Request to `/api/v1/_documents/<docName>/<entryID>` where entity's type ≠ `docCfg.EntityType` returns HTTP 400 (or 404) without running the script.
*Test:* `api_v1_test.go` — create a doc with `entity_type: release`; request it
against a `ticket` entity; assert HTTP error and script is not invoked (use a
script that would write a sentinel file and verify file absent).

10. **AC10 — `script:` bypasses disk cache on both read and write.** `GetCached` is skipped for script: renders; a stale disk cache file from a prior `command:` config at the same path is not served.
*Test:* `document_test.go` — pre-populate
`.rela/documents/<entryID>-<hash>.html` with fake content; render with `script:`
config; assert rendered output is from the Lua script, not from the cache file;
assert no new file is written to `.rela/documents/`.

11. **AC11 — Timeout honors `cfg.Timeout` for script renders.** A script that `while true do end`s terminates at `cfg.Timeout`, not at `lua.DefaultTimeout`.
*Test:* `document_test.go` — script with infinite loop; config `timeout: 1`;
assert render returns a timeout error within ~2s wall clock.

Inspection-only (verified by human review, not executable):

- **AC-DOC1 — Guide section present.** `docs-project/entities/guides/GUIDE-data-entry.md` has a Documents section covering: YAML schema for both variants with `entity_type:` shown as required; `edit://` + `create://` URL schemes; caching (disk cache for command:; in-process `rela.cache` for script: with a note that cache is namespaced by script path); SSE live reload caveat (entry entity only; other entities require refresh button); `rela.mode == "document"` contract; `rela.document.id` and `rela.document.entry_id`; `rela.output` behavior; `html.WithUnsafe` trust boundary (frontend DOMPurify is the sanitizer; future HTML consumers need their own); config hot-reload caveat (editing `data-entry.yaml` changes which script backs a doc on next refresh).
- **AC-DOC2 — FEAT-023 updated.** Content reflects shipped state (command renderer) and the new Lua renderer.
- **AC-DOC3 — Prototype example.** `prototypes/data-entry/project/scripts/docs/<name>.lua` exists, composes a document from multiple entities (uses `rela.trace_from` or `rela.list_entities`), demonstrates `rela.cache.memoize`, and is wired into `prototypes/data-entry/project/data-entry.yaml` under `documents:` with `entity_type:` set.

## Research

- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Existing Solutions:**

- **Reused**: `script.NewWriterRuntime` (`internal/script/runtime.go:20-29`) already builds a Lua runtime wired with AI provider and per-script secrets for a given script path, taking an `io.Writer` stdout. Exactly what we need.
- **Reused pattern — typed Engine method**: `Engine.ExecuteAction` (`internal/script/action.go`) already demonstrates the "typed method that wires its own opts internally" pattern. `ExecuteDocument` mirrors this shape.
- **Reused**: `rela.cache.memoize` (`internal/lua/cache.go`, shipped in #556) handles in-process caching. Namespaced per script path — intentional and preserved.
- **Reused pattern — action mode**: `WithActionMode` (`internal/lua/runtime.go:139-145`) + `isAction` flag + the branching in `luaOutput` (`runtime.go:686-691`). Copy the pattern with s/Action/Document/, carrying `documentID` + `entryID` as fields.
- **Reused — goldmark + link rewriting**: `markdownToHTML` + `RewriteDocumentLinks` in `document.go` are renderer-agnostic; both paths call them after capturing markdown.
- **Reused — config-load existence check**: existing `CheckActionScriptExists` (`internal/script/action.go:105`) fails fast at server startup for missing action scripts. Apply the same pattern to document scripts so operators get startup errors rather than deferred HTTP 500s.
- **Rejected — implicit dependency tracking**: wrapping `rela.get_entity` / `list_entities` / `trace_*` to accumulate dep IDs. Rejected for V1 because exploratory reads would pollute dep sets and `rela.cache.memoize` would hide reads from the tracker. Tracked as TKT-E1FO1.
- **Rejected — variadic-opts Engine method**: an earlier iteration proposed `ExecuteFileWithWriter(path, deps, stdout, opts...)`. Rejected per RR-UPOQZ — variadic opts invite misuse (forged `WithOutputDir`, etc.). Typed `ExecuteDocument` preferred.
- **Rejected — auto-namespacing cache by configID in document mode**: per RR-I5WME the reviewer raised collision risk, but shared helper scripts deliberately want to share cache state across all docs that use them. Auto-namespacing would defeat reusability. Documented caveat instead.

**Prior art in rela:**

- PLAN-R7BQ (action scripts): same shape of captured-stdout + mode flag. Document mode is modeled after this.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

### 1. Config layer (`internal/dataentryconfig/config.go` + `validate.go`)

```go
type DocumentConfig struct {
    Title      string `yaml:"title,omitempty"      json:"title,omitempty"`
    EntityType string `yaml:"entity_type"          json:"entity_type"`           // still required
    Command    string `yaml:"command,omitempty"    json:"command,omitempty"`     // was required; now optional
    Script     string `yaml:"script,omitempty"     json:"script,omitempty"`      // new
    Timeout    int    `yaml:"timeout,omitempty"    json:"timeout,omitempty"`
}
```

Validation updates (`internal/dataentryconfig/validate.go:validateDocuments`):

- `entity_type` is required (unchanged from today; make it explicit in error message).
- Exactly one of `{command, script}` must be non-empty:
  - both set → `"document %q: command and script are mutually exclusive"`.
  - neither set → `"document %q: one of command or script must be set"`.
- If `script:` is set, run the existing startup existence check pattern (equivalent of `CheckActionScriptExists(projectRoot, scriptPath)`) so missing scripts fail at config-load time rather than at render time (addresses RR-1FG6X).
- Path validation is deferred to `script.loadScript` at render time, which already blocks `..` and absolute paths.

### 2. Lua runtime layer (`internal/lua/runtime.go`)

Add fields to `Runtime`:

```go
isDocument  bool
documentID  string
documentEntryID string
```

New option:

```go
func WithDocumentMode(documentID, entryID string) Option {
    return func(r *Runtime) {
        r.isDocument = true
        r.documentID = documentID
        r.documentEntryID = entryID
    }
}
```

In `registerContextBindings`, when `r.isDocument`:

```go
r.L.SetField(rela, "mode", lua.LString("document"))
docTable := r.L.NewTable()
r.L.SetField(docTable, "id", lua.LString(r.documentID))
r.L.SetField(docTable, "entry_id", lua.LString(r.documentEntryID))
r.L.SetField(rela, "document", docTable)
```

In `luaOutput`, add a branch identical to `isAction`:

```go
if r.isDocument {
    fmt.Fprintln(r.stdout, "warning: rela.output() called in document mode; use print() to emit markdown")
    return 0
}
```

### 3. `script.Engine.ExecuteDocument` (`internal/script/executor.go`)

```go
// ExecuteDocument loads and runs a Lua script from scripts/ in document
// rendering mode. Captured stdout is the markdown the caller uses.
// documentID is the key under documents: in data-entry.yaml.
// entryID is the ID of the entity being rendered.
// timeout overrides lua.DefaultTimeout when non-zero.
func (e *Engine) ExecuteDocument(
    path string,
    deps lua.WriteDeps,
    stdout io.Writer,
    documentID string,
    entryID string,
    timeout time.Duration,
) error {
    code, err := loadScript(deps.ProjectRoot, path)
    if err != nil {
        return err
    }
    opts := []lua.Option{lua.WithDocumentMode(documentID, entryID)}
    if timeout > 0 {
        opts = append(opts, lua.WithTimeout(timeout))
    }
    runtime, err := NewWriterRuntime(deps, path, stdout, opts...)
    if err != nil {
        return err
    }
    defer runtime.Close()
    return runtime.RunString(code)
}
```

No variadic opts escape hatch; matches `ExecuteAction` shape.

### 4. Data-entry document service (`internal/dataentry/document.go`)

`documentRenderConfig` gains `Script` and `ConfigID`:

```go
type documentRenderConfig struct {
    Command  string
    Script   string
    ConfigID string
    Timeout  time.Duration
}
```

`documentService` gains a consumer-side interface at the call site:

```go
// DocumentScriptEngine is what documentService needs from script.Engine.
// Defined here, not in script/, per CLAUDE.md's consumer-side interface rule.
type DocumentScriptEngine interface {
    ExecuteDocument(path string, deps lua.WriteDeps, stdout io.Writer,
        documentID, entryID string, timeout time.Duration) error
}
```

Render dispatch:

- **Singleflight key**: `entryID + "|" + cfg.ConfigID` (not just `entryID`) — addresses RR-4QSBN. Two concurrent renders for the same entry but different doc configs no longer collapse.
- **`GetCached` dispatch**: only called when `cfg.Script == ""`. Script renders never read or write the disk cache — addresses RR-25XXM.
- **Render path**:
  - `cfg.Script != ""` → build a `bytes.Buffer`, call `scriptEngine.ExecuteDocument(cfg.Script, deps, &buf, cfg.ConfigID, entryID, cfg.Timeout)`, use `buf.String()` as markdown.
  - `cfg.Command != ""` → existing path unchanged.

### 5. Handler (`internal/dataentry/api_v1.go`, `handlers_document.go`)

`handleV1Documents` **must** enforce EntityType before render (addresses
RR-FLCXC):

```go
ent, err := a.store.GetEntity(ctx, entryID)
if err != nil { /* 404 */ }
if ent.Type != docCfg.EntityType {
    http.Error(w, fmt.Sprintf("entity type %q does not match document's entity_type %q", ent.Type, docCfg.EntityType), http.StatusBadRequest)
    return
}
```

`toDocumentRenderConfig` now populates `ConfigID` (the map key under
`documents:`) in addition to existing fields.

### 6. Documentation (`docs-project/entities/guides/GUIDE-data-entry.md`)

New `## Documents` section with these subsections:

- **What documents are** (rendered HTML panels composed from markdown, live-reloaded).
- **YAML config schema** with both `command:` and `script:` examples, `entity_type:` shown as required.
- **`edit://` and `create://` URL schemes** with rewrite behavior.
- **Caching behavior**:
  - command: uses `.rela/documents/<entry>-<hash>.html` disk cache.
  - script: uses in-process `rela.cache` (link to GUIDE-lua-scripting §Cache). Note: cache namespace is per script path — shared helper scripts share their cache state, which is usually what you want; scoped-by-doc keys require including `rela.document.id` explicitly.
- **SSE live-reload** with the caveat (RR-J3KA9): only changes to the entry entity trigger reload; for multi-entity composition, the refresh button is the escape hatch until TKT-E1FO1 ships.
- **Document-mode context**: `rela.mode`, `rela.document.id`, `rela.document.entry_id`, `rela.params` (noting that `rela.params` stays for author-configured params only and is separate from document identity).
- **`rela.output` behavior**: emits a warning line to the rendered document (RR-AJC21: a loop that calls `rela.output` will produce many warning lines).
- **Security caveat** (RR-TTNT2): the rendered HTML uses `html.WithUnsafe`; DOMPurify in the frontend is the sanitization boundary. Any future consumer of the rendered HTML (PDF export, copy-HTML button) must add its own sanitization.
- **Config hot-reload caveat** (RR-BA10N): editing a document's `script:` value rebinds it on next refresh; open panels pick up the new script.

### 7. `GUIDE-lua-scripting.md` — short subsection

Cross-link to the Documents section in GUIDE-data-entry; state the document-mode
API (`rela.mode`, `rela.document.*`, `rela.output` warning behavior).

### 8. Example (`prototypes/data-entry/project/scripts/docs/release_notes.lua`)

Composes a markdown doc from the entry entity + its `trace_from` children.
Demonstrates `rela.cache.memoize` with a key that includes `rela.document.id` to
show per-doc scoping. Wired into `prototypes/data-entry/project/data-entry.yaml`
under `documents:` with `entity_type:` set.

### 9. FEAT-023 update

Content: V1 shipped (shell-command renderer); V2 adds Lua renderer (TKT-CGBVW).
Out-of-scope follow-ups linked (TKT-E1FO1, TKT-CGPYW).

**Files to modify:**

| File | Change |
|---|---|
| `internal/dataentryconfig/config.go` | Add `Script` field to `DocumentConfig` |
| `internal/dataentryconfig/validate.go` | Mutual-exclusion validation; preserve `entity_type` required; config-load existence check for `script:` |
| `internal/dataentryconfig/validate_test.go` | Table-driven tests |
| `internal/dataentry/document.go` | Dispatch; singleflight keyed on entryID+configID; skip GetCached for script:; skip disk-cache write for script:; wire cfg.Timeout for script: |
| `internal/dataentry/document_test.go` | Lua-path tests; singleflight collision test; stale-cache-not-served test; timeout test; cache-memoize-across-renders test |
| `internal/dataentry/handlers_document.go` | Forward ConfigID + Script + EntityType |
| `internal/dataentry/api_v1.go` | Enforce EntityType on handler; HTTP 400 on mismatch |
| `internal/dataentry/api_v1_test.go` | EntityType mismatch test |
| `internal/dataentry/services.go` | Wire DocumentScriptEngine dependency into App/documentService |
| `internal/lua/runtime.go` | `WithDocumentMode(id, entryID)`; `isDocument`/`documentID`/`documentEntryID` fields; `rela.document` binding; `luaOutput` document-mode branch |
| `internal/lua/runtime_test.go` | Context injection tests (including `nil` assertions for non-document contexts); rela.output warning test |
| `internal/script/executor.go` | New `ExecuteDocument` method |
| `docs-project/entities/guides/GUIDE-data-entry.md` | New `## Documents` section with caveats |
| `docs-project/entities/guides/GUIDE-lua-scripting.md` | Document-mode subsection cross-linking to data-entry guide |
| `prototypes/data-entry/project/scripts/docs/release_notes.lua` | New example |
| `prototypes/data-entry/project/data-entry.yaml` | Wire example under `documents:` |
| rela-issues-and-design-tickets: FEAT-023 | Update to reflect shipped state |

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

| Input | Source | Validation | On invalid |
|---|---|---|---|
| `script:` path in `data-entry.yaml` | project author (trusted) | Must end in `.lua`; `script.loadScript` blocks `..` and absolute paths; new config-load existence check fails fast on missing file | Startup error / HTTP 500 |
| `entry_id` path parameter in HTTP request | untrusted user (via URL) | Entity exists check; **new**: entity.Type must match docCfg.EntityType (RR-FLCXC) | HTTP 404 / 400 |
| `docName` path parameter | untrusted user (via URL) | Lookup in `cfg.Documents[docName]` — allowlist by config keys | HTTP 404 |
| script stdout | trusted Lua runtime output | DOMPurify sanitizes on frontend; Go side rewrites `edit://`/`create://` with existing regex | Malformed URLs pass through |

**Security-Sensitive Operations:**

- **EntityType enforcement** (new, RR-FLCXC): HTTP handler rejects cross-type requests before invoking the script. Removes the exfiltration path where a caller could make a release-scoped doc script run against a ticket.
- **Script execution**: inherits the existing sandbox (no `io`/`os`/`debug`/`loadfile`; file writes confined to `output/` via `rela.write_file`). AI provider wired if configured. Same trust model as actions.
- **Markdown → HTML**: `html.WithUnsafe()` (existing). Frontend DOMPurify sanitizes. **Caveat (RR-TTNT2)**: Lua docs can pull any entity's `content` into the stream, broadening paths by which user-submitted content reaches the unsafe HTML. DOMPurify is the boundary; any future consumer of the rendered HTML must add its own sanitization.
- **Error leakage**: Lua errors include script path and line number — acceptable (same trust domain).
- **DOS surface**: per-render timeout (now honoring `cfg.Timeout`), singleflight dedupes, Lua cache capped at 10k entries. Malicious script is out-of-scope threat model.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** see per-AC mapping above. Summary:

| Layer | Tests |
|---|---|
| Config validation | `validate_test.go` — {both set, neither set, only command, only script, missing entity_type, missing script file at startup} |
| Lua runtime | `runtime_test.go` — `TestWithDocumentMode` (rela.mode, rela.document.id, rela.document.entry_id, nil in other contexts); `TestLuaOutputDocumentMode` warning behavior |
| Script engine | `executor_test.go` — `ExecuteDocument` invokes script with correct opts; returns stdout |
| Document service | `document_test.go` — Lua happy path; singleflight collision with different configIDs; stale cache not served for script; cfg.Timeout honored; cache memoization across renders |
| Handler | `api_v1_test.go` — EntityType mismatch returns 400 without invoking script |
| Integration | `e2e_test.go` or equivalent — prototype project renders via real rela-server |
| Manual | `just dev`, open prototype, verify live reload + refresh button |

**Edge Cases:**

- Script does not exist → config-load error (startup) or render-time error if added later.
- Script writes empty stdout → empty HTML (valid).
- Script raises Lua error mid-render → partial stdout discarded; HTTP 500 with Lua error.
- Script exceeds `cfg.Timeout` → context cancelled; timeout error returned.
- Script writes non-UTF8 → goldmark accepts; HTML may be malformed — not a security issue.
- Entry ID contains shell metacharacters → Lua path is immune (passed via `rela.document.entry_id`, not interpolated into a shell command).
- Concurrent renders for same (entryID, configID) → singleflight dedupes (correct).
- Concurrent renders for same entryID but different configIDs → singleflight does NOT dedupe (correct after RR-4QSBN fix).
- Cache reaches 10k cap during render → LRU evicts; occasional recompute is graceful.
- Config swaps `script:` path while a render is in flight → next refresh picks up new script (documented caveat RR-BA10N).
- Previous `command:`-rendered disk cache file present when same doc is switched to `script:` → stale file not served (RR-25XXM fix).

**Negative Tests:**

- `command:` + `script:` both set → config load fails.
- Neither set → config load fails.
- `entity_type:` missing → config load fails.
- `script:` path missing on disk → config load fails (startup).
- `script: ../../etc/passwd` → `script.loadScript` rejects at render.
- Lua script references undefined global → render fails at script line.
- `rela.output({foo=1})` in document mode → warning emitted, no JSON (AC5).
- HTTP request for doc against wrong-type entity → 400, script not invoked (AC9).
- Concurrent renders for different configs against same entry → both produce their own output (AC8).
- Script: config with stale command:-era disk cache → render produces Lua output, cache file ignored (AC10).
- Infinite-loop script with `timeout: 1` → terminates within ~2s (AC11).

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

1. **Double-caching confusion.** Go-side disk cache (command:) + Lua in-process `rela.cache` (script:) both exist. Mitigation: disk cache explicitly disabled (both read and write) for `script:`; guide documents the split clearly.
2. **`ExecuteDocument` method bloat.** Adds a bespoke method to `script.Engine`; future contexts may add more. Mitigation: each context is a distinct mode; ExecuteAction precedent is already established; the Engine acting as "the knows-how-to-wire-each-Lua-use-case place" is a coherent design.
3. **SSE stale after multi-entity change.** Acknowledged in AC-DOC1 / TKT-E1FO1. Refresh button remains.
4. **Cache-namespace caveat not discovered by users** (RR-I5WME). Guide calls it out; idiomatic use (`rela.document.id` in keys when needed) is shown in the prototype example.
5. **Shell-metachar entry IDs no longer a concern** on Lua path; remains a concern for `command:` path (unchanged).
6. **Effort estimate**: **m** (same as original; the RR fixes tighten rather than expand scope).

## Documentation Planning

- [x] User-facing docs identified
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] `GUIDE-data-entry.md` — new Documents section (substantial; the feature was undocumented).
- [x] `GUIDE-lua-scripting.md` — short Document Mode subsection cross-linking.
- [x] ~~CLI help text~~ (N/A: no CLI commands changed)
- [x] ~~CLAUDE.md~~ (N/A: follows action-mode precedent; no new patterns)
- [x] ~~README.md~~ (N/A: no project-level changes)
- [x] ~~API docs~~ (N/A: `/api/.../documents/.../render` contract unchanged; new error surface covered in guide)

`docs-checklist` will be created manually when entering review.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:**

Critical (must fix, addressed in plan):
- **RR-4QSBN** — singleflight key on (entryID, configID). → AC8, approach §4.
- **RR-FLCXC** — handler enforces EntityType before render. → AC9, approach §5, security section.

Significant (must fix, addressed in plan):
- **RR-1FA8W** — entity_type remains required for script: docs. → AC2, approach §1.
- **RR-I5WME** — cache namespace policy documented (stays per-script-path; caveat in guide; prototype demonstrates explicit keys). → AC-DOC1, approach §6, rejected alternatives.
- **RR-UPOQZ** — typed `ExecuteDocument` method, no variadic opts. → approach §3, rejected alternatives.
- **RR-FTFJU** — `rela.document.entry_id` replaces `rela.params.entry_id`. → AC3, approach §2.
- **RR-J3KA9** — guide documents entry-only SSE limitation. → AC-DOC1.
- **RR-25XXM** — `GetCached` bypassed for script: renders. → AC10, approach §4.
- **RR-DWZKU** — AC5/AC6 test mechanisms concrete (rela.write_file sentinel, runtime-level test). → AC5, AC6.
- **RR-82D0N** — cfg.Timeout wired via lua.WithTimeout for script renders. → AC11, approach §3.

Minor/nit (addressed in plan text):
- **RR-1FG6X** — config-load existence check. → approach §1.
- **RR-TTNT2** — security caveat wording softened, future-consumer risk named. → AC-DOC1, security section.
- **RR-AJC21** — log-noise caveat in guide. → AC-DOC1.
- **RR-BA10N** — hot-reload caveat in guide. → AC-DOC1.
- **RR-PR7M0** — disk-cache scope phrasing tightened. → Scope section.
- **RR-ZB0XP** — ACs split into Testable vs Inspection-only. → Acceptance Criteria section.
