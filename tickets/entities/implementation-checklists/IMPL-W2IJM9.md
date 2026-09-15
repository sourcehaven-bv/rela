---
id: IMPL-W2IJM9
type: implementation-checklist
title: 'Implementation: export: documents export via transforms, like entity and list views'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

Every test drives the real router (`handleV1Documents`), not the export handler
directly, so the path split is exercised rather than assumed.

New tests:

| Test | Covers |
|---|---|
| `TestExportDocument_Anchored` | AC 1 + hardened headers |
| `TestExportDocument_Standalone` | AC 2, incl. `entry_id` absent not `""` |
| `TestExportDocument_TransformResolution` | AC 4 (4 cases) |
| `TestExportDocument_CommandRendererRefused` | AC 5 (both kinds) |
| `TestExportDocument_Gates` | AC 3 (5 gate cases) |
| `TestExportDocument_PermissionGate` | `permission:` on both kinds |
| `TestExportDocument_ElevatedGateAppliesToExport` | 5 ACL impls, all deny |
| `TestExportDocument_NoExistenceOracle` | AC 8, on render AND export |
| `TestExportDocument_ScriptErrorCarriesNoDetail` | AC 9 |
| `TestExportDocument_UsesSharedEngine` | shared pool |
| `TestExportBaseName` | filename stems incl. dotted names |
| `TestExportSegmentPremise` | routing premise guard |
| `TestRenderDocumentMarkdown_RefusesCommandRenderer` | render-layer backstop |
| `TestValidateDocuments_ReservedExportNameRejected` | AC 7 |
| `TestValidateDocuments_NameContainingExportAllowed` | only the exact name is reserved |
| `ExportMenu.test.ts` (4 cases) | AC 6 |
| `transforms.test.ts` (3 new cases) | URL builder, both shapes + encoding |

**Two tests were verified to actually fail without the fix** — a test that
passes against the broken code is worthless:

- `TestExportDocument_NoExistenceOracle`: reinstated the pre-fix ordering (raw
`GetEntity` + `entity_not_found` before the gate) and confirmed it fails on
*both* the render and export subtests, then restored.
- `TestRenderDocumentMarkdown_RefusesCommandRenderer`: removed the new backstop
and confirmed the `anchored` subtest fails (standalone still passed via its own
guard), then restored. This is how the backstop gap was found in the first place
— see Quality below.

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

`newDocExportApp` is the one builder; `anchoredDoc()` / `standaloneDoc()` build
the two document kinds. The fake engine echoes the entry id into its output so a
test proves *which* engine method ran rather than asserting on a literal.

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence**

Built a scratch project (`.ignored/docexport/`) with one standalone document,
one entity-anchored document, a real `ticket` entity, and a `txt` transform (`cp
{in} {out}`). Drove it through `appbuild.Discover` + `dataentry.NewApp` +
`app.NewRouter()` on an `httptest.Server` — the same wiring `cmd/rela-server`
uses, with a **real Lua runtime and a real sandboxed subprocess**, not the fake
engine the unit tests use.

Server log confirmed the real stack: `loaded project entities=1 relations=0` and
`external command confinement detail="sandbox sandbox-exec (no network,
temp-dir-only writes)"`.

| Check | Result |
|---|---|
| AC 2 standalone export | 200; body `entry_id is nil, as a standalone document requires`; `Content-Disposition: attachment` |
| AC 1 anchored export | 200; body `# Ticket Report: First ticket` / `entry_id = TKT-001`; filename `ticket_report-TKT-001` |
| No regression | both render routes still 200 |
| AC 4 | missing transform → 400; unknown → 404 |
| AC 8 | absent id → uniform `Entity not found`, id appears in no problem field except `instance` (which echoes the caller's own URL) |

The standalone result is the one that mattered most: it proves the Lua binding
really sees a **nil** `entry_id`, which is the whole reason
`RenderStandaloneMarkdown` exists rather than passing `""` through
`RenderMarkdown`.

The harness was removed from the package after use (archived under `.ignored/`)
so it does not ship as a test that depends on a scratch directory.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

**DRY**: the central move is the *opposite* of duplication — the ordered gate
chain became `resolveAnchoredDocument` / `resolveStandaloneDocument`, called by
both the render and export handlers. The three error-message builders were
extracted so the two resolvers read as sentences. `resolveTransform`,
`convertAndWrite`, `writeExportResponse` and `ExportMenu.vue` were all reused
unchanged.

**A real gap found during self-review**: the `command:` refusal initially lived
only in the handler. `RenderMarkdown` legitimately dispatches to `renderCommand`
(entity export supports command renderers and has an entry entity to feed them),
so the *anchored* branch had no backstop below the handler — exactly the shape
RR-T5POTJ warned about, one level further down than where it was predicted.
Added the guard in `RenderDocumentMarkdown` and pinned it with a test proven to
fail without it.

**Lint/verification status**

- `just lint` → **0 issues** (fixed 3 findings in the new test file: an unused
variadic, an always-same param, a whitespace rule).
- `just arch-lint` → **OK - No warnings found**.
- `go vet` clean; `vue-tsc` clean; `eslint` 0 errors on changed files.
- `internal/dataentry` 82.4% (floor 55), `internal/dataentryconfig` 91.2%
(floor 70). New code: 89–100% per function.
- `TestNavFilterStaysPresentational` (the `toDocumentRenderConfig` grep guard)
was **narrowed, not widened** — from 3 allowed files to 2 — because the
resolvers are now the only constructors of a render config.

**Known unrelated failure**: `TestMemoryLocker_AbandonedAcquireReleases`
(`internal/lock`) is flaky under `-count>1` and under the coverage recipe's
`-race -shuffle=on`. Reproduced on a clean `develop` worktree, and this branch
does not touch `internal/lock`. It is the only failure in `just coverage-check`.
