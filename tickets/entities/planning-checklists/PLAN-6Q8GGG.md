---
id: PLAN-6Q8GGG
type: planning-checklist
title: 'Planning: Transform registry + view export (pdf/docx via external tools)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined
- [x] Acceptance criteria documented

**DECISION (user-approved):** exec engine = **extract cmdrunner's pure `run()` +
`cappedWriter` + `Probe` + templating consts into a shared `internal/cmdexec`
package**; both `attachment` and the new `transform` package depend on it. Clean
move (no attachment types referenced), guarded by the existing attachment tests.

**DESIGN-REVIEW OUTCOME (gate passed; findings RR-T3PDHN, RR-8C23IL, RR-C3M3BR,
RR-VYTL35, RR-6ZDPTQ, RR-PBTUK5 addressed; RR-UGKOI5 deferred). Two
design-changing results folded into the approach below: (1) the LIST renderer
must live in `internal/dataentry` (needs the ACL neighbor-gate); (2) a Lua
render override on export must reuse the existing gated `_documents` render
path, never call `ExecuteDocument` on a fresh surface.**

**Scope:**

IN scope (v1):
- Metamodel `transforms:` map: named `from: markdown` → format byte shuttles.
- Shared exec engine extracted to `internal/cmdexec` (argv, no shell, `{in}`/`{out}`,
cap, timeout, Probe).
- `Renderer` seam producing markdown: built-in **entity** renderer (in `transform`),
built-in **list table** renderer (in `dataentry`); Lua override via the gated
document path; Lua **document** via existing `handleV1Documents`.
- data-entry: `GET /api/transforms` + entity/list export endpoints streaming transform
output with content-type + filename, hardened like attachment downloads.
- CLI export command (entity renderer; metamodel-only, no dataentryconfig needed).

OUT of scope (v2): format chaining; built-in sectioned list export; async jobs +
global export concurrency limit (RR-UGKOI5); MCP exposure; per-view configurable
cap; per-format templates beyond the Lua override.

**Acceptance Criteria:**

1. `transforms:` entry parsed + validated (unique name, non-empty command, known `from`,
**well-formed `produces` MIME via mime.ParseMediaType**); malformed → whole load
fails. Test: metamodel loader table test (valid + each invalid incl. CRLF in
produces).
2. `GET /api/transforms` returns registered `from: markdown` transforms. Test: API test.
3. Export a visible entity → bytes + `produces` content-type + sanitized
`Content-Disposition` + nosniff + sandbox CSP + no-store. Test: fake transform
(`cp {in} {out}`), assert headers + body.
4. Export a non-visible entity → 404, never bytes. Test: deny-gate API test.
5. Export a filtered list → table over the whole filtered ACL set (not just page), with
relation cells = **visible** neighbor titles (hidden neighbors excluded). Test:
seed
   > pageSize + a hidden neighbor.
6. List > cap N → truncated + a markdown paragraph "Showing N of M rows (truncated)."
Test: seed >N. Boundary: exactly N → no notice.
7. Missing transform binary → clear "not found on PATH" error. Test: bad binary.
8. Per-type Lua render override (dataentryconfig / document config) used over built-in,
**through the gated document path**. Test: config with override asserts output +
that a denied caller gets 404.

## Research

- [x] Libraries — external tools are the "library"; no Go PDF lib (matches DOT-export).
- [x] Codebase patterns checked. [x] Reference implementations reviewed. [x] rela concepts.

**Research Doc:** N/A

**Prior art (file:line):**
- `internal/attachment/cmdrunner.go` — `run()` (116-193), `cappedWriter` (196-215),
`Probe` (74-83), templating consts (57-61): pure stdlib, no attachment types →
EXTRACT to `internal/cmdexec`. `Scan`/`Transform`/`Rejectedf`/`ProcessContext`
stay in attachment.
- `internal/metamodel/types.go:16` `Metamodel` → add `Transforms map[string]TransformDef`.
Loader `loader.go:44/134/747/223` (`checkUnknownKeys` must allow `transforms`).
- `internal/dataentry/api_v1.go:571` `handleV1ListEntities` → `scopedSortedEntities:275`
(full ACL slice pre-pagination). Relation cells: neighbor TITLES are NOT in the
wire response (only IDs, `entityserializer toV1`); reproducing them needs
`outgoingRelations`/`incomingRelations` (606-607), neighbor entities,
`inverseRelationKey`, and the batched visibility gate `visibleRelationIDs` (617)
— all unexported in dataentry.
- `internal/dataentry/api_v1.go:949` `handleV1GetEntity` → `visibleReader.getVisible:959`.
- **Download hardening** `internal/dataentry/handlers_attachment.go:87-145`: nosniff,
`Content-Security-Policy: sandbox; default-src 'none'`,
`safeAttachmentFilename`/ `unsafeFilenameRe` (494-508), gate-before-read. Export
copies this.
- **Gated Lua render** `internal/dataentry/api_v1.go:3172` `handleV1Documents` →
`gateReadOrNotFound:3221` BEFORE render → `documentService.Render` →
`renderScript` (`document.go:238`) → `script.ExecuteDocument`
(`executor.go:98`). Export reuses this.
- `internal/dataentryconfig/config.go:223` `List` → add `Render string` override here.
Separate file (`data-entry.yaml`) from metamodel; CLI loads metamodel always,
dataentryconfig optionally → CLI export works with metamodel + built-in entity
renderer.
- Features: `FEAT-OT4361` (pure model + serializers), `FEAT-KTZJIV` (cmd pipeline).

## Approach

**Technical Approach:**

`internal/cmdexec` (new leaf): `Runner{timeout, maxBytes, tempDir}` with
`Run(ctx, argv, stdin) ([]byte, error)` + `Probe(argv)`. `attachment.CmdRunner`
wraps it, keeping `Scan`/`Transform`/`Rejectedf`. arch-lint: new `cmdexec`
component (empty mayDependOn); add `cmdexec` to `attachment` and `dataentry`
mayDependOn.

`internal/transform` (new): `Def{From, Command []string, Produces}`, `Registry`,
`FromMarkdown()`; `Renderer interface { Render(ctx) ([]byte, error) }`
(call-site interface); `Engine` over `cmdexec.Runner`: `Run(ctx, name, Renderer)
(bytes, ct, err)`. Built-in **entity** renderer here (single entity: title→H1,
props table, its own resolved relation titles supplied to it, body — no ACL
machinery, no store read). arch-lint: new `transform` component, mayDependOn
`entity, metamodel, script, dataentryconfig` — **NOT `dataentry`**.

**List-table renderer lives in `internal/dataentry`** (RR-T3PDHN): it implements
`transform.Renderer`, uses `entityReader` + `visibleReader` + the batched
neighbor gate + `inverseRelationKey` to build a markdown table of the view's
`dataentryconfig.ListColumn`s with **visible** neighbor titles; appends the
truncation paragraph when capped at `const listExportCap` (RR-6ZDPTQ). The
transform `Engine`/`Registry`/`Renderer` interface stay in `transform`;
dataentry supplies the implementation. No dataentry import in transform.

Metamodel: `Transforms` + `TransformDef`; parse + `checkUnknownKeys` +
`validateTransforms` (unique names; non-empty command; `from` ∈ {markdown} at
load; `produces` via `mime.ParseMediaType`, reject CRLF/control) — whole load
fails on a bad entry (RR-VYTL35).

data-entry endpoints (all READ; gate-before-render; hardened like attachment
download — nosniff + sandbox CSP + no-store + sanitized filename, RR-C3M3BR):
- `GET /api/transforms`.
- Entity export: `getVisible` (404 on deny) → entity renderer OR, if a Lua override is
configured, **route through the gated `_documents` render** (RR-8C23IL) →
`Engine.Run`.
- List export: `scopedSortedEntities` (pre-pagination) → cap → dataentry list renderer →
`Engine.Run`.
- Frontend "Export as ▾" fed by `GET /api/transforms`.

CLI: `rela export <entity-id> --transform <name> --out <file>` (entity renderer;
works with metamodel alone).

**Files to modify:** new `internal/cmdexec/*`, `internal/transform/*` (+ tests);
`internal/attachment/cmdrunner.go` (wrap);
`internal/metamodel/{types,loader}.go`; `internal/dataentryconfig/config.go`
(`Render`); `internal/dataentry/export.go` (new: endpoints + list renderer) +
routes; `internal/cli/*` (export cmd); `.go-arch-lint.yml`; `frontend/*`; docs
(`metamodel.md`, `data-entry*`, `cli-reference.md`, CLAUDE.md).

**Alternatives considered:** stdin→stdout-only (rejected: PDF tools need file
paths; `{in}`/`{out}` covers both); per-pair hand-wiring (rejected: defeats the
cross product); render override in metamodel `EntityDef` (rejected: presentation
stays out of metamodel); list renderer in `transform` (rejected by design
review: needs ACL gate → dataentry); transform imports attachment / fresh engine
(rejected for the cmdexec extract).

## Security Considerations

- [x] Input sources identified [x] Allowlist validation [x] Sensitive ops [x] No leaks

**Input Sources & Validation:** command templates = `metamodel.yaml` (config
trust, validated at load, `produces` MIME-checked); request inputs = a **format
name** (allowlist vs registry) + a view reference the caller is already
authorized for — never a command/flag/path; bad name → 4xx, unknown entity →
404.

**Security-Sensitive Operations:** subprocess argv no-shell, rela-owned temp,
timeout, cap, cleanup, missing-binary PATH error; ACL via
`getVisible`/`scopedSortedEntities` only, no new `acl.Op`; **relation cells
filtered through the visibility gate** so hidden neighbor titles never leak
(RR-T3PDHN); **Lua override only through the gated `_documents` path**
(RR-8C23IL); download response hardened (nosniff + sandbox CSP + sanitized
filename, RR-C3M3BR). Read-path Lua is a bounded single-subject render
(permitted case), not a per-row predicate.

## Test Plan

- [x] Scenarios per criterion [x] Edge cases [x] Negative tests [x] Integration approach

**Test Scenarios:** loader table test incl. CRLF-in-produces (AC1); registry
serialization (AC2); dataentry API tests with fake transform `cp {in} {out}` (no
pandoc in CI) for AC3–AC8, incl. hidden-neighbor exclusion (AC5), truncation
boundary (AC6), denied Lua override → 404 (AC8); cmdexec/engine unit tests
(extend cmdrunner tests) + re-run attachment suite after extraction.

**Edge Cases:** empty list (empty table, no notice); entity no relations/body;
exactly N vs N+1; unicode/markdown-special chars escaped in cells; hidden
neighbor excluded from cell; transform timeout (cleaned); over-cap (error not
OOM); `from:` != markdown rejected at load; malformed `produces` rejected at
load.

**Negative Tests:** unknown format → 4xx not 500; malformed `transforms:` (empty
cmd, dup name, bad produces) → load error; deny-ACL entity export → 404;
deny-ACL Lua override export → 404.

## Risk Assessment

- [x] Technical risks + mitigations [x] Security risks [x] Effort estimated

**Risks:** cmdexec extraction touches attachment → pure move, attachment suite
guards it; tool availability per deploy → `Probe` warns at startup, clear
runtime error; slow converters block request → v1 fast tools, timeout+cap bound
it, async is v2; **no global export concurrency limit in v1** (RR-UGKOI5
deferred) → per-call timeout+cap bound each, semaphore is v2; list cost on huge
graphs → cap bounds render + subprocess input, truncation visible.

**Effort:** l. Stages: (1) cmdexec + transform pkg + metamodel + CLI entity
export; (2) data-entry entity export + menu + download hardening; (3) list
export (dataentry renderer) + truncation; (4) Lua override via gated document
path.

## Documentation Planning

- [x] User-facing docs identified [x] Docs-checklist created at implementation

**Documentation Impact:** docs/metamodel.md (`transforms:` + security),
docs/cli-reference.md (export cmd), docs/data-entry.md ("Export as" + `render:`
override + hardening), CLAUDE.md (renderer/transform pattern; "export downstream
of a gated view"; list renderer lives in dataentry for the ACL gate; Lua
override via gated `_documents`).

## Design Review

- [x] Run `/design-review` before implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** RR-8C23IL (critical, addressed), RR-T3PDHN
(significant, addressed), RR-C3M3BR (significant, addressed), RR-VYTL35
(significant, addressed), RR-6ZDPTQ (significant, addressed), RR-PBTUK5 (minor,
addressed), RR-UGKOI5 (minor, deferred to v2 with reason). No open
critical/significant. Confirmed-correct: `transforms:` in metamodel (CLI reaches
it); cmdexec extraction is clean.
