---
id: IMPL-4ELIHQU
type: implementation-checklist
title: 'Implementation: The tenant setting-up screen ignores the app language and name'
started: "2026-11-21"
completed: "2026-11-21"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (TestShellBootFrom, TestBootShell, TestBootShellHandler, TestLoadShellBoot; bootMeta.test.ts)
- [x] Integration tests written (test full flow, not just units) (TestShellBoot builds a temp project and reads it through SharedBase.ProjectConfig; TestBootShellHandler serves through the HTTP handler)
- [x] Happy path implemented
- [x] Edge cases from planning handled (no data-entry.yaml, no app block, invalid language tag dropped, HTML escaping, HEAD, asset paths untouched)
- [x] Error handling in place (errors surfaced, not swallowed) (a load or parse error is logged by shellBoot and the shell falls back to today's page)

## Test Quality

- [x] Using fixture builders or factories for test data (fstest.MapFS and a temp project; table tests)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end (built the SPA and served StaticShell with the real embedded shell)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified (escaping of a name with < and >)

**Verification Evidence:**
Built the SPA (npm run build) and served StaticShell(ShellBoot{AppName: "Harbour <master>", Language: "nl"}) through httptest. GET / and GET /s/portal/x return 200 with `<html lang="nl">`, `<title>Harbour &lt;master&gt;</title>` and the two meta tags before `</head>`. go test ./internal/dataentry/ ./cmd/rela-server/ and the vitest suite pass.

## Quality

- [x] Code follows project patterns (check similar code) (same sync.OnceValue shell rewrite pattern as the custom.js injection)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced (only app.name and app.language from the operator config; escaped; language must match a BCP 47 shape)
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
