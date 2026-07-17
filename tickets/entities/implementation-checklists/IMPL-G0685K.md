---
id: IMPL-G0685K
type: implementation-checklist
title: 'Implementation: CLI table output ignores display_property (uses literal title only)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (`TestWriter_TitleResolver`: resolver-used + nil-fallback)
- [x] Integration-ish: existing `TestGenerateDOT` + cli/output suites exercise the wired path end-to-end
- [x] Happy path implemented (Writer.Titles resolver + entityTitle helper; graph/export/delete wired via svc.Meta())
- [x] Edge cases handled (nil resolver → literal `title` fallback; graph ID==title → no redundant "ID\nID" label)
- [x] Error handling: n/a (pure formatting; DisplayTitle already falls back to ID)

## Test Quality

- [x] Table-driven / subtests where it fits (TestWriter_TitleResolver uses t.Run)
- [x] fakeTitleResolver distinguishes "resolver used" from "literal title read"
- [x] Fixed the graph_test fixture to declare `title` as a required property (it relied on the old e.Title() shortcut that bypassed the metamodel) — realistic, matches real metamodels
- [x] No hardcoded magic

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified
- [x] Edge cases verified

**Verification Evidence:**

Built `bin/rela`, tested against scratch projects:
- **AC2 (template):** `rela list persoon` with `display_property: "{voornaam} {tussenvoegsel} {achternaam}"` → TITLE column shows **"Jeroen Vloothuis"** / **"Jan van der Berg"** (was BLANK before this change).
- **AC1 (bare name):** `rela list persoon` with `display_property: achternaam` → **"Vloothuis"** (was blank).
- **AC3 (literal title unchanged):** `rela list ticket --project tickets` → titles render as before.
- **AC4 (graph):** `rela graph -f dot` → `"PERS-JV" [label="PERS-JV\nJeroen Vloothuis", ...]` — label honors the template. Export relation targets use DisplayTitle too.

## Quality

- [x] Follows project patterns — consumer-side `TitleResolver` interface in `output` (no dependency on metamodel package); `*metamodel.Metamodel` satisfies it. Injected once in kong.go after the project loads.
- [x] DRY — single `entityTitle` helper shared by table + detail (WriteEntity) paths.
- [x] No god-object growth — `Writer` gained one field (no max-fields cap on it); plimsoll passes.
- [x] No silent failures — nil resolver is an intentional, documented fallback (non-project commands).
- [x] No debug code left behind
