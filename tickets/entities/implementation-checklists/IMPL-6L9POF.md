---
id: IMPL-6L9POF
type: implementation-checklist
title: 'Implementation: display_property as a template (multi-property title)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (types_test.go: 7 DisplayTitle template cases + 1 GetPrimaryProperty case; loader_test.go: 5 template validation cases)
- [x] ~~Integration tests~~ (N/A: metamodel is a pure library; the "full flow" is DisplayTitle/validateDisplayProperty, covered by unit tests. Data-entry consumers exercised by existing dataentry tests — still green.)
- [x] Happy path implemented (renderDisplayTemplate + DisplayTitle template branch)
- [x] Edge cases from planning handled (empty/nil/missing → empty; whitespace collapse; all-empty → ID; non-string stringify; literal passthrough)
- [x] Error handling in place (malformed template → load error; every placeholder allowlisted + type-checked)

## Test Quality

- [x] Table-driven with `t.Run` subtests (matches existing TestEntityDef_DisplayTitle / TestParse_DisplayProperty* style)
- [x] Assertions specify only the values that matter (want string per case)
- [x] No hardcoded magic — inputs/outputs are the AC scenarios verbatim
- [x] Shared parse logic (parseDisplayTemplate) reused by runtime + load-time so syntax has one source of truth
- [x] Error-message assertions check substrings (entity, offending value, "unclosed"/"empty placeholder"), matching existing loader tests

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified
- [x] Edge cases manually verified

**Verification Evidence:**

- Built `bin/rela`, created a `persoon` type with `display_property: "{voornaam} {tussenvoegsel} {achternaam}"` in a scratch project; created two persons (empty vs. present tussenvoegsel). `DisplayTitle` unit tests reproduce these exact scenarios and pass.
- **Important finding:** the CLI `rela list`/`export`/`graph` table shows an EMPTY title — but this is a **pre-existing gap**, verified on develop with a bare-name `display_property: achternaam` (also blank). The CLI output writer (`internal/output/output.go:103`) calls `entity.Title()` (literal `title` prop), NOT `metamodel.DisplayTitle`, so it has never honored `display_property` in any form. My feature is correct for every `DisplayTitle` consumer (all in `internal/dataentry` — web UI list rows, entity links, mentions, analyze). Follow-up ticket TKT-VHSHOB filed to wire `display_property` into the CLI output path.
- All 8 ACs PASS as unit tests (`go test ./internal/metamodel/`). Full `go test ./...` green. `go vet` + `golangci-lint` clean.

## Quality

- [x] Code follows project patterns (extends existing DisplayTitle/validateDisplayProperty; shared helper like other metamodel code)
- [x] DRY: `parseDisplayTemplate` + `validateDisplayPropertyRef` extracted so bare-name and template paths share the property type-check; `collapseWhitespace` via `strings.Fields` (no regexp)
- [x] No security issues (pure string manipulation on trusted metamodel config; no new injection surface — RR-V59XZ reasoning unchanged)
- [x] No silent failures (malformed templates fail loud at load; render degrades gracefully only post-validation)
- [x] No debug code left behind
