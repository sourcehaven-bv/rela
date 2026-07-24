---
id: PLAN-OMOBSG
type: planning-checklist
title: 'Planning: export: route entity/list export + export_render through visibility.Reader; thread request principal into ExecuteDocument (closes #1188 IB finding)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

IN: on the **#1188 branch** (`feat/transform-registry`, develop merged in — the
export code exists only there, and the CISO CHANGES_REQUESTED blocks that PR
until fixed): route entity export + list export through `visibility.Reader`;
redact neighbor titles; thread request ctx/principal into
`script.Engine.ExecuteDocument`; negative tests; `docs/transforms.md` §Security
update; reply on PR #1188. Small addition to `internal/visibility`: export a
`Redact(ctx, FieldRedactor, e)` helper (the copy-strip primitive) for
already-gated, already-loaded entities.

OUT: Lua bindings reading redacted (PR 3 / TKT-ZF2DTV — the export_render
override's *in-script* reads stay raw this PR, documented residual); MCP; CLI
render.

**Acceptance Criteria:**

1. Entity export for a principal with field-visibility policy: hidden property value appears nowhere in the exported bytes; H1/title falls back to ID when the display property is hidden.
2. List export: hidden property columns render empty; title column falls back to ID; whole-row ACL unchanged (scoped set).
3. Neighbor titles (entity-export relation groups AND list relation columns): a **visible** neighbor whose title property is hidden renders as its ID (the RR-5N4K35 class, export edition).
4. `ExecuteDocument` takes ctx; the `export_render:` Lua path runs under the request principal (`rela.principal` reflects the caller; scheduler/document paths unaffected in behavior).
5. NopACL byte-parity: without acl.yaml every export output is byte-identical to today (pinned by existing tests staying green).
6. Entity-level ACL behavior unchanged: denied entity → 404 no bytes (existing tests regression-pin).

## Research

- [x] For larger features: run `/research` (RES-PSZZKU, arc-level)
- [x] Existing patterns: `visibility.Reader`/conformance suite (PR 1, #1194); dataentry `readGate`/`affordanceService` adapters; export test fixtures (`export_test.go` mustNewACL + policy-per-test)
- [x] Codebase reuse checked; [x] reference impls consulted; [x] concepts reviewed

**Research Doc:** RES-PSZZKU · DEC-ZBI39P · builds directly on TKT-7I07IX
(merged #1194).

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

*visibility (tiny addition):* export `Redact(ctx, red FieldRedactor, e)
*entity.Entity` — the copy-strip primitive `PolicyReader.redacted` already
implements; PolicyReader delegates to it. For consumers holding an
already-gated, already-loaded entity (neighbor titles) where a type-claimed
`Get` doesn't fit.

*dataentry adapters (new small file):* `ctxRowGate{}` implements
`visibility.RowGate` by delegating to `readGateFromContext(ctx)` per call —
inherits dataentry's per-request nop/acl gate selection, so NopACL parity is
automatic. `affRedactor` implements `visibility.FieldRedactor` over
`affordanceService.hiddenProperties`.

*exportHandler:* gains `visReader visibility.Reader` (a
`PolicyReader{ctxRowGate, affRedactor, store}`) + `redactor
visibility.FieldRedactor`; wired in `newExportHandler` and the test builder.
- `handleV1ExportEntity`: `visibleReader.getVisible` + caller type-check → `visReader.Get` (which owns the stored-type check per RR-SRZK6X; remove the now-redundant caller check).
- `entityRelationGroups` + `memoNeighborTitle` (list): after the existing neighbor visibility gate + load, wrap the node in `visibility.Redact(ctx, redactor, node)` before `DisplayTitle` — closes the hidden-title-on-visible-neighbor leak (AC3).
- `handleV1ExportList`: after `scopedEntities`, `entities = visReader.Filter(ctx, entities)` — redacts every row (row-gate idempotent over the already-scoped set); `columnCell`/title derivations then operate on redacted copies with no code change.
- `exportRenderer` (override path): pass request ctx through `RenderMarkdown` (already does) → `renderScript(ctx, ...)` → `ExecuteDocument(ctx, ...)`.

*script:* `Engine.ExecuteDocument(ctx context.Context, ...)` — first param ctx;
adds `lua.WithContext(ctx)` + `lua.WithPrincipal(principal.From(ctx))`,
mirroring `execute()`/`ExecuteAction`. Update dataentry's consumer-side
`scriptEngine` interface + `renderScript(ctx, ...)` + test fakes (3 files).

*docs:* `docs/transforms.md` §Security — document: exports are field-redacted
like every other read-out; the `export_render:` override now runs under the
caller's principal but its **in-script reads remain unredacted until the
Lua-read seam lands** (TKT-ZF2DTV) — operators authoring override scripts must
treat them as trusted until then.

**Alternatives rejected:** redaction inside `transform.EntityRenderer` (stays
ACL-free per DEC-ZBI39P); per-call `stripHiddenProperties` at the three sites
(the by-convention pattern this arc exists to kill); new PR off develop (export
code lives only on #1188; the CISO block must be resolved on that PR).

**Files to modify:** `internal/visibility/policyreader.go` (+Redact export),
`internal/dataentry/{export.go, export_list.go, document.go,
visibility_adapters.go(new), app.go wiring, test_helpers_test.go}`,
`internal/script/executor.go`, test fakes (`document_script_test.go`,
`e2e_test.go`, `script/executor_test.go`), `internal/dataentry/export_test.go`
(+field-visibility cases), `docs/transforms.md`.

## Security Considerations

- [x] Input sources identified — no new external input; same request surface as #1188
- [x] Validation approach — verdict engines unchanged (allowlist semantics)
- [x] Security-sensitive operations identified
- [x] Error handling doesn't leak

- The change IS the security fix (CISO finding on #1188). Redaction happens on copies at the dataentry boundary; `transform` stays ACL-free; the sandboxed converter receives only redacted markdown.
- Fail-closed inherited from the seam: gate errors → 404/drop; `Get`'s stored-type check now replaces the caller-side check (strictly stronger).
- Residual (documented, tracked): `export_render:` scripts read raw entities via their own Lua bindings until PR 3 — the principal is now threaded so PR 3 flips them to redacted with zero further plumbing.

## Test Plan

- [x] Scenarios per AC; [x] edge cases; [x] negative tests; [x] integration approach

**Test Scenarios:** extend `export_test.go` with a field-visibility policy
fixture (`visible:` allowlist on the exported type):
- AC1: export entity as limited principal → response bytes do NOT contain the hidden value; DO contain granted values; H1 = ID when display prop hidden.
- AC2: list export → hidden column cell empty, title column = ID fallback; full-set semantics unchanged.
- AC3: relation group / relation column where the neighbor is visible but its title is hidden → neighbor rendered as ID, never the title value.
- AC4: `export_render:` override test asserts the script observes `rela.principal.user` == request principal (extends `TestExport_Entity_RenderOverride`).
- AC5/AC6: existing export tests unchanged and green (parity + denied-404 pins).

**Edge Cases:** entity whose EVERY property is hidden (export still renders
H1=ID + body); hidden display property + `?document=` override; list column
naming a hidden property explicitly (empty cell, not error); neighbor vanished
between gate and load (existing skip).

**Negative:** denied entity export → 404 no bytes (existing); unstamped
principal under real policy → gate error path (fail closed).

**Integration:** full dataentry HTTP tests (httptest) over real memstore +
Declarative ACL + affordances resolver — same engines as the conformance suite.

## Risk Assessment

- [x] Risks + mitigations; [x] security risks; [x] effort (m)

- `ExecuteDocument` signature change ripples to fakes/docs paths → compiler-driven, 3 test fakes + 1 interface; `doRender` (cached HTML path) also threads ctx — behavior-neutral there (principal already on request ctx for handleV1Documents).
- Double row-gating in list export (scopedEntities + Filter) → one extra batched probe per type; acceptable, structural-consistency wins; note in godoc.
- Merge-base risk: branch = #1188 + develop merge (conflicts resolved: app.go, test_helpers_test.go — both-sides-add). Full test suite green post-merge.

## Documentation Planning

- [x] User-facing docs identified
- [x] Docs-checklist will be created when entering review/done (enhancement)

**Documentation Impact:** `docs/transforms.md` §Security notes (redaction +
override residual). No metamodel/CLI-reference changes.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** see ticket has-review-response after review runs.
