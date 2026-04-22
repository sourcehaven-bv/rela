---
id: TKT-CGBVW
type: ticket
title: Document the documents feature and add Lua script renderer
kind: enhancement
priority: medium
effort: m
status: review
---

## Problem

The data-entry documents feature (`DocumentConfig` in `data-entry.yaml`,
rendered by `internal/dataentry/document.go`) lets users define documents that
render as HTML panels in the entity view, composed via an external shell
`command:` that emits markdown. This feature is **undocumented** in the public
docs — `GUIDE-data-entry` never mentions it.

Additionally, the shell-out approach has real friction for projects that want to
compose documents from multiple entities: users must write an external tool
(e.g., `mdcomp` + Jinja2) and wire it in via shell. Since rela already has an
embedded Lua runtime with full read access to the entity graph
(`rela.list_entities`, `rela.trace_from`, `rela.get_entity`, etc.), a Lua-script
renderer would collapse this into a single in-tree script.

## Scope

### In scope

1. **Config**: add `script:` field to `DocumentConfig` alongside existing `command:`. Exactly one of `{command, script}` must be set (validation error at config load time if both or neither).
2. **Renderer**: when `script:` is set, execute the `.lua` file via `script.Engine` with a stdout buffer; captured stdout becomes the markdown input. The downstream pipeline (goldmark → HTML → `edit://`/`create://` rewriting) is unchanged.
3. **Document-mode context injection** (only when rendering a document — `nil` in all other contexts):
   - `rela.mode = "document"`
   - `rela.params.entry_id = <id>` (the entry entity being rendered)
   - `rela.document = { id = <config-key> }` (the key under `documents:` in `data-entry.yaml`)
4. **`rela.output` in document mode**: mirror the existing action-mode behavior (`internal/lua/runtime.go:686-691`) — `rela.output()` writes a warning line to captured stdout and returns, instead of emitting JSON. Message: `warning: rela.output() called in document mode; use print() to emit markdown`.
5. **Disk cache**: skip the existing `.rela/documents/<entry>-<hash>.html` disk cache for `script:` renders (Lua's in-memory `rela.cache.memoize` is the caching story for Lua docs). Keep the disk cache for `command:` renders — shelling out is expensive and process-lifetime doesn't help between CLI invocations.
6. **Documentation**: add a **Documents** section to `docs-project/entities/guides/GUIDE-data-entry.md` covering both `command:` and `script:` variants, the `edit://` + `create://` URL schemes, caching behavior, and SSE live-reload.
7. **Example**: ship a Lua document script under `prototypes/data-entry/project/scripts/docs/` demonstrating multi-entity composition.
8. **Update FEAT-023** to reflect shipped state (command-based renderer is in production) and add the Lua-script renderer to its scope.

### Out of scope (follow-up tickets)

- `rela.document.depends_on(id)` for SSE dependency tracking (V1 relies on refresh button + the existing entry-entity-change reload).
- Generalizing `rela.mode` to other contexts (script / flow / scheduled / validation). V1 sets it only in document mode.
- Removing the disk cache for `command:` renders.
- Export of rendered markdown/HTML as a download.

## Acceptance criteria

- AC1: A `data-entry.yaml` entry with `script: scripts/docs/foo.lua` (and no `command:`) renders via Lua, with stdout captured as markdown, goldmark-converted to HTML, and `edit://`/`create://` links rewritten.
- AC2: Config with both `command:` and `script:` set, or neither, fails validation at load time with a clear error.
- AC3: Inside the Lua script, `rela.mode == "document"`, `rela.params.entry_id` matches the entry ID, and `rela.document.id` matches the config key.
- AC4: In a non-document Lua context (script / flow / action / scheduled / validation), `rela.mode` and `rela.document` are `nil`.
- AC5: `rela.output({...})` inside a document-mode script writes a `warning: ...` line to the rendered document (visible in the HTML panel) and does **not** emit JSON.
- AC6: `rela.cache.memoize("key", fn)` inside a document-mode script caches across HTTP requests within the same `rela-server` process.
- AC7: Shell-command documents (`command:`) behave identically to today — including disk caching.
- AC8: `GUIDE-data-entry` in `docs-project/entities/guides/` has a Documents section covering both renderer variants, `edit://` + `create://` schemes, and caching.
- AC9: `prototypes/data-entry/project/` has at least one Lua document example that composes a markdown doc from multiple entities.
- AC10: FEAT-023 reflects shipped state and includes the Lua renderer.

## Test plan

- Unit tests in `internal/dataentry/document_test.go`:
  - Lua renderer happy path: script writes markdown to stdout → captured → HTML → returned.
  - Config validation: both set → error; neither set → error; only `command:` → OK; only `script:` → OK.
  - `rela.mode` / `rela.document` / `rela.params.entry_id` set correctly inside the script.
  - `rela.output` in document mode emits warning, not JSON.
  - Disk cache skipped for `script:` renders; used for `command:` renders.
- Integration: end-to-end via `rela-server` with a test data-entry project — request `/api/.../documents/.../render`, confirm rendered HTML.
- Manual: run `just dev`, open a Lua-scripted document in the browser, verify live-reload via SSE.

## Risks

- **Risk**: Long-running Lua render could block the data-entry request for the default Lua timeout. **Mitigation**: keep the existing goroutine + singleflight pattern; document the timeout in the guide; surface errors as HTTP 500 with the Lua error message.
- **Risk**: Config validation for mutual exclusion lives in two places (metamodel loader vs. data-entry config loader) if we're not careful. **Mitigation**: validate in the data-entry config loader only (`internal/dataentryconfig/`) — same layer that already validates other config fields.
- **Risk**: Scripts reading many entities can unexpectedly hit the cache max-entries cap (10k). **Mitigation**: document the cap in the docs; note that scripts should choose compact cache keys.

## Effort

m — touches `internal/dataentryconfig/config.go`,
`internal/dataentry/document.go` + `handlers_document.go`,
`internal/lua/runtime.go` (new context fields for document mode), the docs
guide, and a prototype example. No frontend changes (existing
`DocumentsPanel.vue` already posts to the render endpoint and consumes HTML).
