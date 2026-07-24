---
id: IMPL-1O8TLF
type: implementation-checklist
title: 'Implementation: Transform registry + view export (pdf/docx via external tools)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

All 4 stages implemented, tested, committed. Commits: 38b2b58 (1), 2e7b6f1 (2),
305afd1 (3), 41427ec (4).

Acceptance criteria coverage (Go tests use a `cp {in} {out}` fake transform — no
pandoc in CI):
- **AC1** metamodel `transforms:` parse+validate incl. malformed/CRLF → `internal/metamodel/transforms_test.go` (TestParse_Transforms table).
- **AC2** `GET /_transforms` → `TestExport_TransformsList`.
- **AC3** visible entity export bytes + `produces` CT + hardened headers → `TestExport_Entity_VisibleReturnsBytesAndHeaders`.
- **AC4** denied entity export → 404 no bytes → `TestExport_Entity_DeniedReturns404NoBytes`.
- **AC5** list export whole ACL set (>page) + hidden-neighbor exclusion → `TestExport_List_WholeScopedSetAsTable`, `TestExport_List_HiddenNeighborExcluded`.
- **AC6** cap + truncation notice (boundary N vs N+1) → `TestExport_List_TruncationNotice`.
- **AC7** missing binary → clear PATH error → `internal/transform` TestEngine_Probe_MissingBinary + cmdexec Probe test.
- **AC8** render override used, and denied override → 404 (RR-8C23IL gated path) → `TestExport_Entity_DocumentOverride`, `_DeniedIs404`, `_UnknownDocument`. Built-in entity renderer → `internal/transform` TestEntityRenderer_Render + CLI TestRenderCmd.

Plus: CLI `rela render` end-to-end (`internal/cli/render_test.go`); frontend
`transforms.test.ts` (4 tests); ExportMenu on entity + list views.

Quality gates: `golangci-lint` 0 issues on all touched Go packages;
`go-arch-lint` clean (new `cmdexec`/`transform` components; `transform` does NOT
import `dataentry`); full `internal/...` suite green; frontend typecheck/lint (0
errors)/build green; EntityList's 14 existing tests still pass.

Docs: `docs/transforms.md` (hand-authored reference); CLAUDE.md transform/export
architecture note. A docs-project auto-gen entity is a reasonable follow-up
doc-task (the `docs/*.md` files are generated; transforms.md is standalone).

## Quality

- [x] Code follows project patterns (mirrors attachment download hardening, document gate, list read path)
- [x] Checked for DRY opportunities — shared `displayTitleOrTitle`, `visibleRelationIDs` reused; `cmdexec` extracted so attachment+transform share one reviewed exec core
- [x] No security issues introduced (design-review findings addressed; ACL gate authoritative; argv no-shell; hardened downloads)
- [x] No silent failures (errors surfaced via writeV1Error / returned; transform failures logged + 500, caller-input → 4xx)
- [x] No debug code left behind
