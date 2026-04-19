---
id: IMPL-W898R
type: implementation-checklist
title: 'Implementation: Fix misfiled entity files in docs-project/entities/'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] ~~Unit tests written for new code~~ (N/A: pure data move, no code changes)
- [x] ~~Integration tests written (test full flow, not just units)~~ (N/A: existing rela CLI tests already cover fsstore plural-folder scanning; this ticket is a data fix for the consumer repo, not a fsstore change)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] ~~Error handling in place (errors surfaced, not swallowed)~~ (N/A: no code changes)

**Commands executed (from `docs-project/entities/`):**

```
git mv guide/GUIDE-best-practices.md guides/
git mv guide/GUIDE-cli-reference.md guides/
git mv guide/GUIDE-concepts.md guides/
git mv guide/GUIDE-data-entry.md guides/
git mv guide/GUIDE-export.md guides/
git mv guide/GUIDE-getting-started.md guides/
git mv guide/GUIDE-mcp-server.md guides/
git mv guide/GUIDE-metamodel.md guides/
git mv feature/FEAT-analysis.md features/
git mv feature/FEAT-commands.md features/
git mv feature/FEAT-data-entry.md features/
git mv feature/FEAT-entity-management.md features/
git mv feature/FEAT-export.md features/
git mv feature/FEAT-graph.md features/
git mv feature/FEAT-grouped-navigation.md features/
git mv feature/FEAT-mcp.md features/
git mv feature/FEAT-metamodel.md features/
git mv feature/FEAT-migrations.md features/
git mv feature/FEAT-relations.md features/
git mv feature/FEAT-templates.md features/
git mv feature/FEAT-tracing.md features/
git mv feature/FEAT-views.md features/
git mv tutorial tutorials
git mv scenario scenarios
git mv concept concepts
rmdir guide feature
```

## Test Quality

- [x] ~~Using fixture builders or factories for test data~~ (N/A: no new tests)
- [x] ~~No hardcoded values in assertions when object is in scope~~ (N/A: no new tests)
- [x] ~~Only specifying values that matter for the test~~ (N/A: no new tests)
- [x] ~~Interpolated values constructed from objects, not hardcoded~~ (N/A: no new tests)
- [x] ~~Property comparisons use original object, not hardcoded strings~~ (N/A: no new tests)

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

- **AC1 (plural-only folders):** `ls docs-project/entities/` returns exactly `concepts features guides scenarios tutorials`. No singular folders remain.

- **AC2 (content preserved):** `git diff --cached --stat` shows `34 files changed, 0 insertions(+), 0 deletions(-)`. `git diff --cached --diff-filter=R --name-status | wc -l` reports 34 renames. Filter for non-rename changes (`--diff-filter=dmACt --name-only`) is empty. Every move is R100.

- **AC3 (rela loads correctly):** Running `rela list` from `docs-project/` (fresh process, after deleting stale `.rela/cache.json`) reports **38 entities (31 published)** with every entity correctly classified:
  - 7 concepts (CON-*)
  - 16 features (FEAT-*) — previous 2 + 14 moved
  - 10 guides (GUIDE-*) — previous 2 + 8 moved
  - 3 scenarios (SCN-*)
  - 2 tutorials (TUT-*)

Previously (with the singular folders), rela was classifying moved files under
garbage types like `guid`/`featur`/`concep`/`tutoria`/`scenari`, or the stale
cache at 38 entries was masking the misclassification. Post-fix, types resolve
correctly from folder names.

- **AC4 (analyze clean):** from `docs-project/`:
  - `rela analyze cardinality` → `✓ All cardinality constraints satisfied`
  - `rela analyze orphans` → `✓ No orphan entities found`
  - `rela analyze properties` → `✓ All entity and relation properties are valid`
  - `rela analyze validations` → `✓ No custom validation rules defined in metamodel`

No type-mismatch errors referencing `guid`/`featur`/etc.

**Note on MCP cache:** The long-running `rela-docs` MCP server in this session
only reports 16 entities after `refresh` because its process-lifetime scan
predates the directory moves. Restarting the MCP server (or the Claude session)
picks up the full 38. Fresh CLI processes see the correct view immediately. This
is an MCP-process-state quirk, not a data defect.

## Quality

- [x] Code follows project patterns (check similar code) — `tickets/entities/` uses the same plural-folder layout; `docs-project/` now matches.
- [x] No security issues introduced — only moves existing tracked files within the repo; no new content, no user input, no network.
- [x] No silent failures (errors logged AND returned) — N/A, no code changes.
- [x] No debug code left behind — N/A, no code changes.
