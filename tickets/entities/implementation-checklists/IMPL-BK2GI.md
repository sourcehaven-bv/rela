---
id: IMPL-BK2GI
type: implementation-checklist
title: 'Implementation: Refactor document links to app-relative + add Lua router/URL helpers'
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

- **AC1–AC3** (form links get return_to; non-form internal/external unchanged): covered by `internal/dataentry/document_test.go:TestRewriteDocumentLinks`, 16 subtests spanning form, list, entity, kanban, external, mailto, anchor, empty href. All pass.
- **AC4** (unknown internal paths warn + passthrough): `TestRewriteDocumentLinks/unknown_internal_path_warns_and_passes_through` asserts the warning text and that the href is preserved.
- **AC5** (legacy `edit://` / `create://` schemes warn + passthrough): two dedicated subtests in the same table.
- **AC6** (path verification, deterministic query, encoding): `internal/lua/urls_test.go:TestURL_happyPath`, 10 subtests covering path-only, params, existing-query merge, fragment preservation, key ordering, number/bool values.
- **AC7** (unknown path raises): `TestURL_unknownPathRaises`. Error message contains the offending path.
- **AC8** (existing query + fragment preserved): subtest `fragment preserved`. Also see the integration case in the rewriter tests.
- **AC9** (type/validation errors): `TestURL_typeErrors` — 7 subtests: non-table arg, function value, nil value (dropped), keys with `&`/`=`/whitespace, empty key.
- **AC10** (`rela-server routes`): `cmd/rela-server/routes_test.go` — covers happy path table + json, invalid-format exit code, column contents including `form_id`/`entity_id` and `yes` for `AcceptsReturnTo`.
- **AC11** (frontend ↔ Go parity): `internal/frontendparity/parity_test.go` — regex-parses `frontend/src/router/index.ts` and diffs against `frontendroutes.All()`. Test passes; fails loudly with named-route diff if either side drifts.
- **AC12** (old tests migrated): `customLinkRegex`, `buildEditLink`, `buildCreateLink` deleted; tests rewritten. Lint clean, no dead code references.

Full CI run: `just check` → lint + arch-lint + lint-md + test → pass. `just
coverage-check` → total 73.9% (13370/18103), above 65% floor. `just build` →
clean. `just arch-lint` → clean after archfile update. `just docs` regenerates
with no spurious diffs.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

**Notes:**

- `internal/frontendroutes` is a stdlib-only leaf as planned. Package-level `All` / `Has` / `Match`, no constructed catalog value.
- `internal/lua/routes.go` declares the minimal `RouteCatalog` interface (`Has(path) bool` only). `RouteCatalogFunc` adapter avoids a wrapper type at every call site.
- `internal/lua.Runtime` gains a `routes` field; `WithRouteCatalog` is the option. `rela.url` registers conditionally in `registerContextBindings`, matching the `rela.cache` pattern.
- `internal/dataentry/document.go` declares its own `routeMatcher` interface locally (reads `AcceptsReturnTo`) plus a local adapter around `frontendroutes.Match`. Clean call-site separation from Lua's interface.
- Rewriter accepts a `*slog.Logger` parameter; `nil` falls back to `slog.Default()`. Tests capture warnings via a test-local TextHandler.
- `cmd/rela-server/main.go` adds one-line dispatch on `os.Args[1] == "routes"` before `flag.Parse()`; existing invocations untouched.
- `internal/script.NewWriterRuntime` wires `frontendroutes.Has` into every writer runtime. Reader runtimes (validation) intentionally skip it — `rela.url` is absent there.
- Prototype `scripts/docs/category_report.lua` migrated: `edit://ticket/` → `rela.url("/form/edit_ticket/" .. id)`, `create://ticket?belongs-to=` → `rela.url("/form/create_ticket", {["rel.belongs-to"] = entry_id})`.
- `docs-project/entities/guides/GUIDE-data-entry.md` — scheme reference table replaced with an app-relative link + `rela.url` section. `docs/data-entry.md` regenerated.
- `CLAUDE.md` — `frontendroutes` added to the architecture table.
- `.go-arch-lint.yml` — new `frontendroutes` component plus `cmdServer` / `dataentry` / `script` allow-edges.
