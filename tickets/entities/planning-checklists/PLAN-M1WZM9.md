---
id: PLAN-M1WZM9
type: planning-checklist
title: 'Planning: Render admin-authored header/footer markdown on list views'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

IN: Admin-authored markdown info regions at the top and bottom of a data-entry
list view, configured per-list in `data-entry.yaml`, rendered as
DOMPurify-sanitized HTML. Authoring is admin-only (YAML); no UI editing.

OUT: Entity-backed / query-backed info blocks (dashboard-card style) — left as
an additive seam. Per-entity-type default info in the metamodel. Bare-entity-ID
code-span resolution to titles (needs a server `mentions` map the `/_config`
endpoint doesn't supply; standard `[text](/entity/ID)` markdown links work
without it). End-user / in-UI authoring.

**Acceptance Criteria:**
1. List config with `header` markdown renders sanitized HTML above the search/filter row. Test: set header on a list, load `/list/:id`, assert rendered HTML present in header region.
2. List config with `footer` markdown renders sanitized HTML below the table/pagination. Test: symmetric to (1) at the bottom region.
3. Output is DOMPurify-sanitized. Test: header containing `<script>` / `<img onerror>` renders inert (no script tag survives) — covered by `renderMarkdown` reuse; add a unit test asserting sanitization at the call site.
4. A list with no header/footer renders exactly as before — no empty region, no layout shift. Test: `v-if` guards; component test asserts no `.list-info` node when unset.
5. Backward compatibility: existing `description` values keep working (rendered as the top/header region per the naming decision).

## Research

- [x] Checked codebase for similar patterns or reusable code
- [x] Reviewed relevant prior art

**Existing Solutions:**
- `renderMarkdown()` — `frontend/src/utils/markdown.ts:58`. Sanitizes via DOMPurify (line 111), GFM, optional `refResolver`/`interactive`. When no `refResolver` is passed it behaves as plain markdown — exactly our case.
- Render pattern to copy: `EntityDetail.vue:868` — `<div class="markdown-content" v-html="renderMarkdown(section.content || '', refResolver)"/>`. Existing `v-html` sites carry `eslint-disable vue/no-v-html` (rule is `warn`).
- `List.Description` — `internal/dataentryconfig/config.go:188` — plumbed end-to-end (Go → `/_config` JSON `description` → TS `ListConfig.description` at `config.ts:127`) but never rendered. Existing-but-unused hook.
- Render sites — `EntityList.vue`: header block ends at line 653 (top region goes after it, before `.search-row` at 665); bottom region goes after `<Pagination>` (910-914), inside the `v-if="listConfig"` wrapper (closes line 916).

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns
- [x] Alternatives considered
- [x] Dependencies identified

**Technical Approach (naming: `header`+`footer`, `description` as deprecated
alias for `header`):**

1. Go `internal/dataentryconfig/config.go` `List` struct: add
`Header string \`yaml:"header" json:"header,omitempty"\``and`Footer string
\`yaml:"footer" json:"footer,omitempty"\``. Keep the existing
`Description`field. In the config load/normalize path, if`Header == "" &&
Description != ""`, copy `Description`into`Header`(alias) so old configs render
in the top slot. (Locate the normalize step near where lists are loaded; if none
exists, do the fallback in the`/_config`assembly in`handleV1Config`.)
2. TS `frontend/src/types/config.ts` `ListConfig`: add `header?: string` and
`footer?: string` alongside existing `description?`.
3. `EntityList.vue`:
   - import `renderMarkdown` from `@/utils/markdown`.
   - Add a computed `headerHtml` = `renderMarkdown(listConfig.value?.header ?? listConfig.value?.description ?? '')` and `footerHtml` = `renderMarkdown(listConfig.value?.footer ?? '')`.
   - Template: after `</header>` (653), a `<div v-if="headerHtml" class="list-info list-info--top markdown-content" v-html="headerHtml"/>` with the eslint-disable comment. Symmetric `list-info--bottom` after `<Pagination>`.
   - Scoped CSS for `.list-info` spacing consistent with existing `.markdown-content`.
4. `data-entry.yaml` example usage added to one in-tree config (e.g. `tickets/data-entry.yaml` or a prototype) for manual verification.

**Alternative considered & rejected:** keep only `description` + add `footer`.
Rejected for asymmetric vocabulary; `header`/`footer` reads better and the alias
keeps zero migration cost.

**Files to modify:**
- `internal/dataentryconfig/config.go` (struct + alias fallback)
- `internal/dataentryconfig/config_test.go` (alias + parse tests)
- `internal/dataentry/api_v1.go` OR the config normalize site (whichever owns the alias fallback)
- `frontend/src/types/config.ts`
- `frontend/src/components/lists/EntityList.vue`
- `frontend/src/components/lists/EntityList.*.test.ts` (or a new test) if list-component tests exist
- one `data-entry.yaml` for the worked example
- `docs/data-entry.md` (document `header`/`footer`)

## Security Considerations

- [x] Input sources identified
- [x] Input validation approach defined
- [x] Security-sensitive operations identified

**Input Sources & Validation:** The only input is `header`/`footer` markdown
from `data-entry.yaml` — an admin-authored, git-versioned file (same trust level
as the rest of the config). It is nonetheless rendered through
`renderMarkdown()` which DOMPurify-sanitizes, so even a hostile/mistaken config
string cannot inject executable script. No new network input, no user-supplied
input on this path.

**Security-Sensitive Operations:** None new. Reuses the audited sanitization
pipeline. The `v-html` sink is the same one used across the app and is fed only
sanitized output.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified
- [x] Negative test cases defined
- [x] Integration test approach defined

**Test Scenarios:** see Acceptance Criteria (each has a concrete test).

**Edge Cases:**
- header/footer unset → no region rendered, no layout shift (AC5).
- both `header` and `description` set → `header` wins (alias only fills when header empty).
- only `description` set (legacy) → renders in top slot (AC5/backcompat).
- markdown with an entity-ID code span → renders as inert code (no resolver); documented non-goal.
- empty string vs unset → both render nothing (`renderMarkdown('')` returns '').

**Negative Tests:** `<script>alert(1)</script>` and `<img src=x
onerror=alert(1)>` in header → sanitized to inert output (Go alias + TS/Vitest
render test).

**Integration test approach:** Go test asserts `/_config` JSON carries `header`
(incl. alias fallback from `description`). Frontend component/unit test asserts
header/footer regions render and are absent when unset. E2E is optional for a
config-render change; rely on component test + manual verification.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed
- [x] Effort estimated

**Risks:**
- XSS via config → mitigated by mandatory `renderMarkdown` sanitization (never raw `v-html` of the string).
- Layout shift / visual regression → mitigated by `v-if` guards and scoped CSS; verify manually.
- Alias ambiguity (`header` vs `description`) → mitigated by a single documented precedence rule + test.
- Effort: **s**.

## Documentation Planning

- [x] User-facing docs identified

**Documentation Impact:**
- [x] docs/data-entry.md — document `header`/`footer` list fields + `description` alias note.
- [x] N/A for metamodel.md/cli-reference.md — this is data-entry config, not metamodel or CLI.

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: effort:s change reusing an existing sanitized-markdown pipeline; user approved the plan directly.)
