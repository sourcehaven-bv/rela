---
id: REV-L53B
type: review-checklist
title: 'Review: Add task list (checkbox) support to Lua markdown AST'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Coverage maintained (`just coverage-check`)

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:**

Design-review findings (PLAN-KUJE phase):
- RR-AT2I (significant): Strikethrough markers stripped — ADDRESSED
- RR-1KSJ (significant): Multi-block list items not supported — ADDRESSED
- RR-5ZCF (significant): Mixed list goldmark behavior unverified — ADDRESSED
- RR-GQJT (significant): Renderer must produce parseable output — ADDRESSED
- RR-872O (minor): task=false semantics — ADDRESSED
- RR-340I (minor): Constructor field validation — ADDRESSED
- RR-YJ0U (nit): TaskCheckBox is inline child — ADDRESSED

Code-review findings (cranky-code-reviewer):
- RR-MJK3 (critical): renderListItemTable swallows malformed items — ADDRESSED
- RR-ZBIK (critical): Multi-block items concatenate text — ADDRESSED
- RR-NI64 (critical): Inline marker preservation inconsistent — ADDRESSED
- RR-JNVD (significant): TestMdMixedListBehavior asserts nothing — ADDRESSED
- RR-1HLF (significant): Non-string task value coverage — ADDRESSED
- RR-VCIY (significant): TrimLeft doesn't handle tabs — ADDRESSED
- RR-HMRP (significant): Sparse table truncation — ADDRESSED
- RR-WKA2 (significant): Empty task text parse — ADDRESSED
- RR-BD7O (significant): Transformations not tested — ADDRESSED
- RR-V3UT (significant): luaMdList docstring stale — ADDRESSED
- RR-908A (nit): detectTaskCheckbox first-block undocumented — ADDRESSED
- RR-MJMQ (nit): renderListItemTable not a method — ADDRESSED
- RR-OVHE (nit): Bold/italic dropping called "limitation" — ADDRESSED
- RR-4A6G (nit): Strikethrough non-list test gap — ADDRESSED

**Open review responses: 0** (all 21 closed as `addressed`).

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

| AC | Status | Test |
|----|--------|------|
| AC1: Parse task items into table form | PASS | TestMdTaskListParse |
| AC2: Render task syntax (- [x]/- [ ]) | PASS | TestMdTaskListRender, TestMdTaskListRoundTrip |
| AC3: Constructor accepts task tables | PASS | TestMdTaskListRender/constructor_with_task_items |
| AC4: Backward compat (string items) | PASS | Existing TestMdRender/* still pass |
| AC5: task=false / missing → plain | PASS | TestMdTaskListNonBoolTaskValues |
| AC6: Missing/non-string text → defensive | PASS | TestMdTaskListNonStringText |
| AC7: Strikethrough preserved | PASS | TestMdInlineTextPolicy |

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-RTH3 (inline below — no separate checklist entity
created since the work was a single section addition).

**Docs work performed:**

- Added a new "Markdown AST: Task Lists" section to `docs/lua-scripting.md`
under `## API Reference` covering: task item table shape, reading checkbox
state, building task lists, mutating existing checklists, the inline marker
preservation policy (strikethrough + code spans preserved; bold/italic/links
dropped), and the documented limitations (first-text-block-only, mixed list
asymmetry, sparse table behavior).
- `markdownlint docs/lua-scripting.md` clean.

**Pre-existing doc gap (out of scope, follow-up filed):**

The rest of the `rela.md.*` API surface (parse, render, headers, list,
shift_headers, set_min_header_level, extract_section, first_paragraph, concat,
node constructors, generation helpers) was undocumented before TKT-RTH3 — that
gap dates back to TKT-XKRH which shipped the AST API without docs. Filed as
**TKT-CVG6** "Document the full rela.md (markdown AST) Lua API" (kind=docs,
priority=low).

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (Deferred: awaiting user approval before PR/commit)
- [x] ~~All CI checks pass~~ (Deferred: PR not yet created)
- [x] ~~PR URL documented below~~ (Deferred: PR not yet created)

**PR:** *Pending — will create after user approval.*
