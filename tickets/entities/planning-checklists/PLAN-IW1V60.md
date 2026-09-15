---
id: PLAN-IW1V60
type: planning-checklist
title: 'Planning: export: documents export via transforms, like entity and list views'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

IN:
- `_export` sub-resource on both document shapes (anchored + standalone).
- Routing in `handleV1Documents`, dispatching before the existing shapes.
- `ExportMenu` wired into `DocumentView.vue` + a `documentExportUrl` helper.
- Docs: a "Exporting a document" section in `docs/transforms.md`.

OUT:
- `command:` renderers — refused, see Approach.
- A `documents.<id>.export_render` override. A document IS the render.
- An `exportable:` opt-out flag (decided with the user: export inherits
`permission:` and reveals nothing the on-screen render does not, so a flag would
gate presentation, not confidentiality).
- Async export for slow converters — a documented v1 limit shared with the two
existing export surfaces.
- Caching/singleflight of export output.

**Acceptance Criteria:**

1. `GET /api/v1/_documents/{doc}/{entityId}/_export?transform=X` returns the
converted bytes with hardened download headers. → Test: register a `cat`
transform, assert body + `Content-Disposition`.
2. `GET /api/v1/_documents/{doc}/_export?transform=X` does the same for a
standalone document. → Test: same, against a doc with no `entity_type:`.
3. Every gate is enforced, one test per gate (see Test Plan).
4. Unknown transform → 404; missing `?transform=` → 400.
→ Test: `resolveTransform` is reused, so assert both codes on the new route.
5. A `command:` document is refused with a clear error.
→ Test: assert a 400-class error naming the limitation, not a partial render.
6. The Export menu appears in `DocumentView.vue` when transforms are registered
and is absent when none are. → Test: Vitest mount with a mocked `getTransforms`.
7. A document named `_export` is a config-load error.
→ Test: `validateDocuments` table case.

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A — the approach is fixed by two shipped siblings; there is
no open design question a survey would resolve.

**Existing Solutions:**

No new library. Every piece already exists in-tree; this ticket is wiring:

- `FEAT-5IUVGX` names the Lua document as one of its three renderers. Two are
shipped (`TKT-JF5JI8`, `TKT-95XU13`); this is the third. The feature is not
being extended, it is being completed.
- `internal/dataentry/export.go:115` `handleV1ExportEntity` — the handler shape
to mirror (gate → resolve transform → renderer → `convertAndWrite`).
- `internal/dataentry/export.go:212` `resolveTransform` — reused verbatim.
- `internal/dataentry/export.go:236` `convertAndWrite` — reused verbatim; it
already owns the shared engine, the error mapping, and `writeExportResponse`.
- `internal/dataentry/document.go:296` `RenderMarkdown` — already exported for
exactly this purpose; its godoc names view export as the caller and states that
callers must gate the read themselves.
- `internal/dataentry/export.go:295` `exportRenderer` — the precedent for
wrapping a document render in a `transform.RendererFunc`.
- `frontend/src/components/entity/ExportMenu.vue` — already generic over a
`urlFor` callback; no component change needed.
- `RenderStandalone` / `RenderListMarkdown` — the fail-closed precedent for
refusing a `command:` renderer where there is no entry entity.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

*Routing.* `handleV1Documents` splits the path into at most 2 segments today.
Change it to split into 3 and dispatch on a trailing `_export`:

```
/_documents/{doc}                    -> standalone render      (existing)
/_documents/{doc}/{entityId}         -> anchored render        (existing)
/_documents/{doc}/_export            -> standalone export      (new)
/_documents/{doc}/{entityId}/_export -> anchored export        (new)
```

The 2-segment ambiguity (`{entityId}` vs `_export`) resolves on the same premise
the entity router already relies on at `api_v1.go:219`: an entity id never
starts with `_`. The premise that is NOT already guaranteed is the document
name, so add a `validateDocuments` rule rejecting a document named `_export`.
That turns an unroutable config into a load error instead of a shadowed route.

*Handlers.* Rather than duplicating the gate chains, extract the gate sequence
of each existing handler into a helper that returns the resolved render config,
and have both the render and export handlers call it. Concretely, in
`standalone_document_handler.go` and the anchored handler:

```go
// returns (renderCfg, ok); writes the response itself on deny
func resolveStandaloneDocument(a *App, w, r, docName) (documentRenderConfig, bool)
func resolveAnchoredDocument(a *App, w, r, docName, entityID) (documentRenderConfig, bool)
```

This is the load-bearing part of the design. The gates are ordered and the order
matters (`gateReadOrNotFound` must precede the type-mismatch branch so a denied
principal gets a 404 rather than a 400 oracle). Two copies of an ordered gate
chain is exactly how one copy later loses a gate — so export must not get its
own copy. One helper, two callers, and a divergence becomes impossible rather
than merely discouraged.

*Rendering.* The export handler then:

1. `resolveTransform(w, r)` — 400/404 on bad input.
2. Refuse `len(cfg.Command) > 0`: a `command:` render is entity-anchored by
construction and its output is cached HTML keyed on an entry hash. Refusing
matches `RenderStandalone`/`RenderListMarkdown`. (Anchored `command:` docs
*could* work, but supporting one kind and not the other is a worse API than
supporting neither; ship script-only, note it as v2.)
3. `transform.RendererFunc` closing over `documents.RenderMarkdown(ctx, entryID,
cfg)` — entryID is `""` for standalone.
4. `convertAndWrite(...)` — shared engine, hardened headers.

*RenderMarkdown and standalone.* `RenderMarkdown` currently dispatches
Script→`renderScript`, else→`renderCommand`, and `renderScript` passes an
entryID through to `ExecuteDocument`. A standalone export needs
`ExecuteStandaloneDocument` instead (so `rela.document.entry_id` is nil, not
""). So the renderer selects the render call by document kind, not by a magic
empty id:

```go
if docCfg.IsStandalone() { -> ExecuteStandaloneDocument path }
else                      { -> RenderMarkdown(entryID, cfg) }
```

The cleanest expression is a new `documentService.RenderStandaloneMarkdown` that
is `RenderStandalone` minus the `markdownToHTML` step — mirroring exactly how
`RenderMarkdown` relates to `doRender`. `RenderStandalone` then becomes
`RenderStandaloneMarkdown` + `markdownToHTML`, so there is one render path, not
two that can drift.

*Filename.* Reuse `exportFilename`, with base `doc` for standalone and
`doc-entityId` for anchored.

*Frontend.* Add `documentExportUrl(name, entityId?, transform)` to
`api/transforms.ts` and mount `<ExportMenu :url-for="...">` in the
`DocumentView.vue` header next to Refresh.

**Alternatives considered:**

- *`?transform=` on the render endpoint* — rejected with the user. No routing
change, but one URL would return either JSON or a binary download, and it would
diverge from the two shipped export endpoints.
- *Copy the gate chains into the export handlers* — rejected. See above: an
ordered security chain must have one copy.
- *Support `command:` documents by feeding the entry entity as `{in}`* —
deferred. Works for anchored docs only; asymmetry is worse than absence.

**Files to modify:**

| File | Change |
|---|---|
| `internal/dataentry/api_v1.go` | 3-segment split + `_export` dispatch |
| `internal/dataentry/standalone_document_handler.go` | extract gate helper; standalone export handler |
| `internal/dataentry/export_document.go` (new) | both export handlers |
| `internal/dataentry/document.go` | `RenderStandaloneMarkdown`; `RenderStandalone` refactored onto it |
| `internal/dataentryconfig/validate.go` | reject a document named `_export` |
| `frontend/src/api/transforms.ts` | `documentExportUrl` |
| `frontend/src/views/DocumentView.vue` | mount `ExportMenu` |
| `docs/transforms.md` | "Exporting a document" section |

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

| Input | Source | Validation | On invalid |
|---|---|---|---|
| `{doc}` | URL path | `isSafePathSegment` (allowlist), then must be a key in `Documents` | 400 / 404 |
| `{entityId}` | URL path | `isSafePathSegment`, then must exist and match `entity_type` | 400 / 404 |
| `?transform=` | URL query | must be a key in the metamodel registry | 400 missing / 404 unknown |
| document `script:` | operator config | not request input; never selected by the caller | n/a |

The caller chooses a transform **name** and nothing else — never a command,
flag, or path. That is the same invariant the two shipped export routes hold.

**Security-Sensitive Operations:**

1. *Rendering Lua on a GET.* Already the case for the HTML render; export adds
no new execution surface. The ordering guarantee — every gate fires before
`RenderMarkdown` — is what keeps an unauthorized caller from triggering an
expensive aggregation.
2. *Elevated documents.* `gateElevatedDocument` is the ONLY boundary for a doc
with `allow_acl_bypass`. Skipping it on the export path would convert a
permission-gated report into an ungated download. This is the single highest
consequence in the ticket, and it is why the gate chain is extracted rather than
re-typed.
3. *Feeding user content to a converter.* Unchanged from the existing exports:
`internal/cmdexec` confines the process (no network, temp-dir writes, rlimits,
process-group kill, bounded pool) and the response is served with `nosniff` +
sandbox CSP + `no-store` via `writeExportResponse`.
4. *Error leakage.* A transform failure maps to a generic 500 + "check server
logs" (`convertAndWrite` already does this) so a command line never reaches the
client. Script errors route through the existing `writeV1ScriptError` path,
which already gates full detail on `allowFullScriptDetail`.

**Explicit non-goal:** export must make no ACL decision of its own. Every
decision is delegated to the same gate the HTML render uses.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

Integration tests through the real mux (the shape `export_test.go` and
`acl_documents_test.go` already use), with a transform registered as a
deterministic shell-free command so no external binary is needed:

| AC | Test |
|---|---|
| 1 | `TestExportDocument_Anchored` — 200, body is the transformed markdown, `Content-Disposition` attachment |
| 2 | `TestExportDocument_Standalone` — same, `entry_id` nil inside the script |
| 3 | one test per gate, below |
| 4 | `TestExportDocument_MissingTransform` (400), `_UnknownTransform` (404) |
| 5 | `TestExportDocument_CommandRendererRefused` |
| 6 | `DocumentView.spec.ts` — menu shown / hidden |
| 7 | `TestValidateDocuments_ReservedExportName` |

**Gate tests (AC 3)** — each asserts the render never ran:

- unsafe path segment → 400
- unknown document → 404
- kind mismatch both ways → 400
- entity not readable (`gateReadOrNotFound`) → 404, indistinguishable from missing
- `permission:` not held → 403
- elevated doc under `NopACL` / `ReadOnlyACL` → 403 (the fail-open class that
`authorizeElevatedDocument` exists to prevent — assert it on the export route
specifically, since a fresh route is exactly where that gate gets forgotten)
- entity type mismatch → 400, but only AFTER the read gate

**Edge Cases:**

- A document named `_export` → config load error (AC 7).
- An entity id beginning with `_` → cannot exist; still assert the route treats
it as not-found rather than as an export.
- `?transform=` present twice → `Query().Get` takes the first; assert it still
resolves against the registry (no injection surface, but pin the behavior).
- Empty script output → a valid empty document; the transform still runs.
- Script raises → `lua.ScriptError` path, detail gated by `allowFullScriptDetail`.
- Render exceeds the timeout → 500, no partial body.
- Concurrency: N simultaneous exports must not exceed the engine's pool of 4 —
guaranteed by reusing `h.engine`; assert the export handler holds the same
`*transform.Engine` pointer as the entity path rather than constructing one.

**Negative Tests:**

Every gate test above is a negative test. The two that matter most:
`command:`-renderer refusal (must not silently render entity-less), and the
elevated-document gate (must not fail open).

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| Export path drifts from the render path's gate order, losing a gate | medium | **high** — an ungated elevated report | Extract the chain into one helper used by both; a per-gate test on the export route |
| Route ambiguity between `{entityId}` and `_export` | low | medium | Entity ids never start with `_`; add the config-load rule for a doc named `_export` |
| Refactoring `RenderStandalone` onto a new markdown method regresses HTML rendering | low | medium | `RenderStandalone` keeps its signature and existing tests; only its body changes |
| A second `transform.Engine` gets constructed, voiding the concurrency cap | low | medium | Reuse `exportHandler`; assert the shared pointer in a test |
| `command:` docs look arbitrarily unsupported to an operator | medium | low | Refuse with a message naming the reason; record as v2 |

**Effort:** m — three focused Go changes plus a small frontend wiring, but the
gate-chain extraction touches security-sensitive ordering and deserves careful
review.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] `docs/transforms.md` — new "Exporting a document" section: both URL
shapes, that export inherits `permission:` and elevation, and the script-only
limitation.
- [x] `docs/data-entry.md` — note the Export menu on the document view (verify
the documents section exists there first).
- [x] ~~`docs/metamodel.md`, `docs/cli-reference.md`, `CLAUDE.md`, `README.md`~~
(N/A: no metamodel change, no CLI change, no new pattern, no project-level change)

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** RR-XIYP3I (significant, fixed — a pre-existing
entity-existence oracle in the chain this ticket extracts), RR-74LHU1
(significant, fixed — the plan's script-error claim did not match the code),
RR-LELF94, RR-T5POTJ, RR-MZE4IA (minor, all fixed). Two plan corrections came
out of it: export keeps the flat 500 rather than routing script errors through
`writeV1ScriptError`, and refusing `command:` renderers is a policy choice
rather than a structural impossibility.
