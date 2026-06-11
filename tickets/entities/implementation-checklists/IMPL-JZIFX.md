---
id: IMPL-JZIFX
type: implementation-checklist
title: 'Implementation: Extract shared widget registry from FieldRenderer'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (registry.test.ts, widgets.test.ts, wrapperWidgets.test.ts, FieldShell.test.ts — 56 new tests)
- [x] Integration tests written (existing FieldRenderer.test.ts exercises the full resolve→FieldShell→widget path; passes unchanged)
- [x] Happy path implemented (8 property widgets + factory registry + FieldShell)
- [x] Edge cases from planning handled (null/undefined value, unknown widget name, empty widget name, list-over-values precedence, empty values array)
- [x] Error handling in place (unknown widget → console.warn + fallback; unresolvable → throws; type mismatch → advisory warn)

## Test Quality

- [x] Using fixture builders or factories for test data (makeStub factory; per-case it.each table)
- [x] No hardcoded values in assertions when object is in scope (component refs compared by identity)
- [x] Only specifying values that matter for the test
- [x] ~~Interpolated values constructed from objects~~ (N/A: no interpolation in these tests)
- [x] ~~Property comparisons use original object~~ (N/A: widget tests assert emitted events, not preserved props)

## Manual Verification

- [x] Feature manually tested end-to-end (browser smoke test against tickets project)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified (hidden field filter, list-fallback multi-select)

**Verification Evidence:**

Built frontend + rela-server, ran against `tickets/` project, drove with
browser:

- `create_idea` form: title (text+placeholder+help), description (textarea), category (select with 8 enum options from metamodel), inspiration (text) — all render via the registry with FieldShell-owned labels/help/required asterisk.
- `create_ticket` form: kind/priority/effort (selects), `tags` property with no explicit widget renders as SlimSelect multi-select — confirms the `list:true → multi-select` multi-axis fallback survives the refactor (RR-0Z1P6). `status` (hidden) correctly not rendered. Relation pickers + `affects` multi-select unaffected.
- Typing into text input updates the bound value; select carries correct option set.

Automated gate:
- `npm run typecheck`: clean
- `npm run lint`: 0 issues in new files
- `npm run test:run`: 896 passed (was 840; +56 new). Existing FieldRenderer affordance/transition/option-verdict tests pass unchanged → no behaviour change.
- Coverage: widgets dir 95.5% stmts / 100% lines. The one `coverage:check` violation (`src/stores/schema.ts`) reproduces on a clean develop tree with this branch's changes stashed — pre-existing flake, not caused by this work.

## Quality

- [x] Code follows project patterns (factory mirrors consumer-side interface pattern; Vue `update:modelValue` idiom matches RruleBuilder/TagSelect)
- [x] Checked for DRY opportunities (FieldShell owns shared chrome; stringValue/empty-handling repeated per widget intentionally — 3-line trivial, not worth an abstraction)
- [x] No security issues introduced (no v-html added; widget names map to components, never to markup; existing escaping preserved)
- [x] No silent failures (unknown widget warns + falls back; unresolvable throws)
- [x] No debug code left behind
