---
id: PLAN-23EP1Q
type: planning-checklist
title: 'Planning: Render admin-authored header/footer markdown on kanban boards'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

IN: `header`/`footer` markdown info regions on kanban boards, implemented by
**sharing** the list implementation (resolvers + styles) rather than copying it;
plus a scroll-containment fix moving `overflow-x` off the page wrapper onto the
board containers.

OUT:
- No `description` field on kanban. On lists it exists only because it
predated the feature and was already present in configs; a kanban has no legacy
`description` to accommodate. The shared resolver's fallback is inert for kanban
because the field is absent from `KanbanConfig` — TypeScript enforces this, so
sharing the helper does not leak the semantic.
- Help-icon affordance for `description` → split to TKT-8CL0PO, so the
semantics are decided once and applied to lists and kanbans uniformly instead of
diverging here.
- No server-side markdown rendering or sanitization change — reuses the
existing client-side `renderMarkdown`.

**Acceptance Criteria:**

1. Kanban config with `header` markdown renders sanitized HTML above the board.
Test: `KanbanView` component test mounts with `header: '# Hi'`, asserts a
`.view-info--top` node containing the rendered `<h1>`.
2. Kanban config with `footer` renders sanitized HTML below the board.
Test: symmetric, asserts `.view-info--bottom`.
3. Markdown is sanitized via the same `renderMarkdown` path as lists.
Test: a `header` containing `<script>` / `onerror=` yields no script tag or
inline handler in the rendered HTML.
4. No header/footer configured → renders exactly as before, no empty region.
Test: mount without the fields, assert neither info node exists.
5. `_config` serves `header`/`footer` for kanbans, omits keys when unset.
Test: Go `TestKanbanHeaderFooter_YAMLAndJSON` +
`TestKanban_EmptyHeaderFooterOmittedFromJSON`, mirroring the existing `List`
pair at `internal/dataentryconfig/config_test.go:329,366`.
6. On a board wider than the viewport, page title / filter bar / truncation
banner / header / footer stay visible while columns scroll — both branches.
Test: assert `overflow-x` is NOT on `.kanban-view` and IS on the board
containers (structural assertion; jsdom does not lay out, so the visual claim is
confirmed by manual verification, documented below).
7. Existing list header/footer behavior unchanged after the extraction.
Test: the existing `EntityList` + `config.test.ts` suites must pass untouched in
substance, including `description` fallback precedence.

## Research

- [x] For larger features: run `/research` — N/A (small, direct precedent exists)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A — the approach reuses an already-reviewed in-tree
implementation; the only design question (`description` semantics) was settled
with the user during planning.

**Existing Solutions:**

Direct prior art in-tree — TKT-H7E611 / FEAT-RUGB28 built exactly this for
lists. This ticket **shares** that code rather than porting it:

- `internal/dataentryconfig/config.go:216-253` — `List.Header`/`Footer` fields
and the doc comment explaining the `description` alias.
- `internal/dataentryconfig/config_test.go:329-380` — the YAML/JSON round-trip
and `omitempty` test pair to mirror.
- `frontend/src/types/config.ts:143-176` — `ListConfig.header`/`footer` and the
`listHeaderMarkdown`/`listFooterMarkdown` resolvers (trim, `''` when unset).
- `frontend/src/components/lists/EntityList.vue:276-277,696-697,961-962` — the
`computed` + `v-if` + `v-html` render pattern with the eslint disable comment
justifying `v-html` as sanitized.
- `frontend/src/components/lists/EntityList.vue:978-1018` — `.list-info`
typography/spacing styles, including the first/last-child margin collapse.
- `docs/data-entry.md:691-735` — the list field table rows and the "Header and
footer info regions" section this will sit beside.

**Shared-styles convention already exists:** `frontend/src/styles/` holds
`back-button.css`, `markdown-content.css`, `mobile-bars.css`,
`text-utilities.css`, `tokens.css`, all imported in `main.ts:6-14`.
`back-button.css` is precedent for exactly this — one class shared across
EntityDetail, CustomView, and BackButton. So `.list-info` → shared
`view-info.css` follows an established pattern rather than inventing one.

No new library needed: `renderMarkdown` (`frontend/src/utils/markdown.ts`)
already sanitizes and is the exact function EntityList uses.

Also checked for a reusable tooltip/help-icon component (for the `description`
idea) — **none exists**; the SPA only uses bare `:title` attributes. That is a
reason the help-icon work is its own ticket (TKT-8CL0PO) rather than a rider
here.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

1. **Go config** (`internal/dataentryconfig/config.go`): add to `Kanban`, after
`Title`, matching `List`'s tags exactly: `Header string \`yaml:"header"
json:"header,omitempty"\``and the same for`Footer`. No validation rules — the
value is opaque markdown, and `validateKanbans`has nothing to check (consistent
with`List`).

2. **Shared TS resolvers** (`frontend/src/types/config.ts`): replace the two
list-specific helpers with view-generic ones taking a **structural** param type,
so both configs satisfy them without a union or a second copy:

   ```ts
   export function viewHeaderMarkdown(
     view: { header?: string; description?: string } | undefined
   ): string {
     return view?.header?.trim() || view?.description?.trim() || ''
   }
   export function viewFooterMarkdown(
     view: { footer?: string } | undefined
   ): string {
     return view?.footer?.trim() || ''
   }
   ```

`KanbanConfig` has no `description`, so the fallback branch is unreachable for
kanban — enforced by the type, not by having two functions. Add
`header?`/`footer?` to `KanbanConfig`. Update `EntityList.vue` to the new names
and update `config.test.ts` accordingly (keeping the existing
fallback-precedence cases, which now also serve AC7).

3. **Shared styles**: move the `.list-info` block from `EntityList.vue` to a
new `frontend/src/styles/view-info.css` as `.view-info`, imported in `main.ts`
beside the other shared sheets. Drop the `:deep()` wrappers — they exist only
because the rules were inside a scoped component block and are unnecessary (and
inert) in a global sheet. Update EntityList's template to `.view-info`. **Verify
the `--top` rule's `margin-top: -12px`**: it is tuned to `.list-header`'s
`margin-bottom: 24px` (per the comment at `EntityList.vue:987-988`);
`.page-header` in KanbanView also uses `margin-bottom: 24px`
(`KanbanView.vue:613`), so it should transfer — but confirm visually rather than
assume.

4. **KanbanView.vue**: import `renderMarkdown` + the shared resolvers; add
`headerHtml`/`footerHtml` computeds. Render the header region after the filter
bar and before the truncation banner (page context, above the board and its
warnings); render the footer as the last child of `.kanban-view`, after all
board branches, so it appears once for the loading / error / simple-board /
swimlane cases instead of being duplicated per branch. Both use `v-if` +
`v-html` with the eslint-disable comment, matching EntityList.

5. **Scroll containment fix**: `.kanban-view` drops `overflow-x: auto`
(keeping `max-width: 100%`); `.kanban-board` gains `overflow-x: auto`;
`.kanban-swimlane-board` changes `overflow: hidden` → `overflow: auto hidden`.
The swimlane grid's `overflow: hidden` exists to clip cells to its
`border-radius: 8px` (`KanbanView.vue:824-833`), so the two-value form keeps
that vertical clipping while enabling horizontal scroll. This also fixes the
pre-existing quirk where the title and filter bar scrolled sideways.

6. **Docs** (`docs/data-entry.md`): add `header`/`footer` rows to the kanban
field table and a subsection cross-referencing the list one, noting kanban has
no `description` alias.

7. **Example config**: add `header`/`footer` to a board in
`tickets/data-entry.yaml` for manual verification, as TKT-H7E611 did with the
`all_ideas` list.

**Files to modify:**

- `internal/dataentryconfig/config.go` — `Kanban` struct fields
- `internal/dataentryconfig/config_test.go` — YAML/JSON + omitempty tests
- `frontend/src/types/config.ts` — `KanbanConfig` + shared resolvers
- `frontend/src/types/config.test.ts` — resolver tests, renamed + kanban cases
- `frontend/src/styles/view-info.css` — **new**, extracted from EntityList
- `frontend/src/main.ts` — import the new stylesheet
- `frontend/src/components/lists/EntityList.vue` — use shared resolvers/class,
drop the local `.list-info` block
- `frontend/src/views/KanbanView.vue` — computeds, 2 regions, scroll fix
- `frontend/src/views/KanbanView.test.ts` — render/absence/sanitization/scroll
- `docs/data-entry.md` — kanban field table + info-regions subsection
- `tickets/data-entry.yaml` — example board header/footer

**Alternatives considered:**

- *Separate `kanbanHeaderMarkdown`/`kanbanFooterMarkdown` copies.* Rejected —
this was the earlier plan; the user correctly pushed back. The two helpers
differ only in one fallback field, which a structural param type handles without
duplication, and copying would leave two places to fix a sanitization or
trimming bug.
- *Union type `ListConfig | KanbanConfig` on the resolvers.* Rejected: couples
the helper to both concrete config types and would need widening for every
future view type. A structural param is open-ended.
- *Footer inside each board branch.* Rejected: duplicates the region across the
simple-board and swimlane branches, and would not render in loading/error.
- *Keeping `overflow-x` on `.kanban-view` and placing the footer outside it.*
Rejected — that was a workaround for the layout bug; fixing the scroll
containment is the real solution and also unsticks the title/filter bar.
- *Copy the `description` alias for symmetry.* Rejected — see Scope.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

The only input is `header`/`footer` markdown from `data-entry.yaml`, authored by
an admin with filesystem access to the project config — the same trust level as
the rest of the config (which already controls forms, lists, and Lua export
scripts). It is not end-user input and is not entity data.

The content is rendered as HTML via `v-html`, so the real control is
sanitization: `renderMarkdown` (`frontend/src/utils/markdown.ts`) is the same
function EntityList uses for the identical list regions, and it sanitizes
output. AC3 pins this with a test asserting a `<script>`/`onerror=` payload in
`header` produces no script tag or inline handler. Sharing one resolver (rather
than two copies) *improves* this posture: a single sanitization path means a
future fix cannot land on one view type and miss the other.

**Security-Sensitive Operations:**

- `v-html` rendering — protected by `renderMarkdown` sanitization + the
explicit eslint-disable comment documenting why the exception is safe (matching
EntityList's convention so the audit trail is uniform).
- No file access, auth, or crypto surface. No ACL interaction: the regions are
static admin-authored config, contain no entity data, and are served to anyone
already authorized to see the board — so there is no per-row visibility question
and nothing new for `internal/visibility` to gate.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** (mapped to ACs above)

- AC1/AC2 → `KanbanView.test.ts`: mount with `header`/`footer` set, assert the
`.view-info--top` / `.view-info--bottom` nodes exist with rendered HTML.
- AC3 → `KanbanView.test.ts`: `header` with `<script>alert(1)</script>` and an
`<img onerror=…>` payload → rendered HTML contains neither.
- AC4 → `KanbanView.test.ts`: mount without the fields, assert neither node
exists (guards the no-layout-shift claim structurally).
- AC5 → `config_test.go`: YAML decode + JSON marshal has both keys; empty
`Kanban{}` marshals with neither key present.
- AC6 → structural assertion on which selectors carry `overflow-x`, plus
manual verification (jsdom does not lay out, so a component test cannot prove
the visual behavior — see Manual Verification).
- AC7 → the existing list suites (`config.test.ts` fallback-precedence cases,
EntityList render tests) must still pass after the extraction; this is the
regression gate on the refactor.
- Resolver units → `config.test.ts`: set / unset / whitespace-only / undefined,
for both a list-shaped and a kanban-shaped input, plus the precedence case
(`header` wins over `description`) and a kanban-shaped object confirming the
fallback cannot apply.

**Integration:** the `_config` round-trip (AC5) is the integration seam — it
proves the SPA actually receives the fields rather than only that the struct
compiles. No E2E test: TKT-H7E611 judged E2E optional for the equivalent list
regions, and the render path is now literally shared.

**Manual Verification (required — covers AC6):** run `just dev` against the
tickets project with the example board, then confirm at a viewport narrower than
the board: (a) columns scroll horizontally; (b) title, filter bar, header, and
footer stay fixed; (c) the swimlane board scrolls horizontally with its rounded
corners still clipped; (d) the list view's header/footer are visually unchanged
after the style extraction.

**Edge Cases:**

- Unset → no region, no empty div (AC4).
- Whitespace-only (`header: "   "`) → treated as unset via `.trim() || ''`.
- Header set but footer unset (and vice versa) → only the one region renders.
- Very long markdown → wraps; region is outside the scroller so must not widen
the board or introduce a second horizontal scrollbar.
- Board in loading or error state → footer still renders (page-level context,
deliberately outside the branch).
- Swimlane vs simple board → identical regions; placement after both branches
guarantees this without duplication.
- Narrow viewport / mobile → the scroll change interacts with the existing
`@media` rules at `KanbanView.vue:890,912`; check the board still scrolls rather
than overflowing the page.
- Board narrower than viewport → no scrollbar appears (`auto`, not `scroll`).

**Negative Tests:**

- Script/handler injection in `header` → sanitized away, no execution (AC3).
- Malformed markdown (unclosed emphasis, stray backticks) → renders as text,
does not throw; `renderMarkdown` is total and returns a string.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- *Regressing list rendering via the shared extraction* — the main new risk,
since this ticket now touches working list code. Mitigated by AC7: the existing
list tests must pass unchanged in substance, plus manual visual confirmation
that list header/footer are unaffected.
- *Scroll change breaks board layout* — moving `overflow-x` affects every
existing board, including ones with no header/footer. The swimlane `overflow:
hidden` → `auto hidden` is the subtle part (it must keep clipping cells to the
rounded border). Mitigated by explicit manual verification of both branches at a
narrow viewport, and by the mobile-media-query edge case.
- *`margin-top: -12px` on `--top` may not transfer* — it is tuned to
`.list-header`'s spacing. `.page-header` uses the same `margin-bottom: 24px`, so
it should be fine; confirmed visually rather than assumed.
- *XSS via `v-html`* — low. Reuses the already-reviewed sanitizer, now on a
single shared path, pinned by AC3.
- *`KanbanView.vue` size* — 917 lines against the `max-lines: 500` eslint warn
threshold; this adds ~30 (less than before, since styles move out). Already
warning, warn-level only, does not fail CI. Worth a decomposition ticket later.

**Effort:** m (raised from s: the shared extraction touches working list code
and the scroll fix affects every board, so the testing surface is wider than a
pure additive change.)

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] `docs/data-entry.md` — kanban field table rows for `header`/`footer` +
an info-regions subsection cross-referencing the list one and noting the absence
of a `description` alias on kanban.
- [x] ~~`docs/metamodel.md`~~ (N/A: data-entry config, not metamodel)
- [x] ~~`docs/cli-reference.md`~~ (N/A: no CLI surface)
- [x] ~~`CLAUDE.md`~~ (N/A: no new pattern or convention — the shared-stylesheet
approach follows the existing `frontend/src/styles/` convention).
- [x] ~~`README.md`~~ (N/A: not project-level)

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** None outstanding. Two design questions were raised
and resolved with the user during planning: (1) `description` semantics — split
to TKT-8CL0PO; (2) whether to duplicate the list helpers — resolved in favor of
sharing, and (3) scroll containment — fixed properly here rather than worked
around. The remaining work builds on an implementation that already passed
review under TKT-H7E611.
