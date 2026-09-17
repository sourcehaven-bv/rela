---
id: PLAN-E0DQFU
type: planning-checklist
title: 'Planning: Markdown tables in the detail body render cramped: full body width + horizontal overflow'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Problem (measured, not assumed).** Reproduced the reported table in a headless
browser against the exact current CSS. At a 1200px body column:

| | columns (px) | header height |
|---|---|---|
| current | 148 / **58** / 385 / 352 / 256 | 68px (3 lines) |

A 58px column is why "Niveau" renders as "Nive au" and "voldoende" as "vold oen
de". `scrollWidth == clientWidth` (1200 == 1200), so the `overflow-x: auto`
added by TKT-YYZRGW **never activates** — its acceptance criterion 2 was never
actually true.

**Root cause.** `markdown-content.css:38` sets `overflow-wrap: anywhere` on
`.md-body`, inherited by every cell. Unlike `break-word`, `anywhere`
**contributes to min-content width**, collapsing each cell's minimum to a single
character. With `table-layout: auto` and `width: 100%`, the table therefore
always fits the container, so it squeezes instead of overflowing.

**Why the body also feels narrow.** The reported table's natural (max-content)
width is **2576px**, against a `.entity-detail` cap of 1200px
(`EntityDetail.vue:2377`). Wrapping is correct for this content, but the column
is also narrower than the user's screen warrants. Both are addressed.

## Research

- [x] For larger features: run `/research` — N/A, small CSS change
- [x] Searched for existing libraries — N/A, CSS layout
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A (effort `s`, CSS-scoped).

**Prior art in this codebase:**

- **TKT-YYZRGW** introduced the current rules; its review produced **RR-5ZVPC5**
("display:block breaks width:100% fill on narrow tables"), which already
rejected a `display: block` table. That finding is binding here and rules out
the most obvious fix.
- **App tables already use a wrapper element**: `.table-wrapper`
(`EntityDetail.vue:2838-2840`) and `.table-scroll-wrapper`
(`EntityList.vue:1707`). That is the in-repo idiom for scrollable tables.
- GitHub's markdown CSS wraps tables in a scrolling block and keeps cells
wrapping — the widely-deployed reference for this exact problem.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified

**A planning assumption was disproved, and the approach changed accordingly.**
An earlier draft of this plan asserted that a table can wrap its cells *or*
scroll, never both. That is **false**, and acting on it would have shipped a
worse fix. The correct statement: at a *given* column width a table either fits
(and wraps) or overflows (and scrolls) — and a `min-width` floor on cells is
what sets the width at which wrapping stops and scrolling begins. Measured in
Chrome at a 1200px column, with a wrapper and the table left as `display:
table`:

```css
.md-table-scroll { overflow-x: auto; }
.md-body table   { border-collapse: collapse; width: auto; min-width: 100%; }
.md-body th,
.md-body td      { min-width: 12ch; max-width: 40ch; overflow-wrap: break-word; }
```

| case | table width | scrolls? | result |
|---|---|---|---|
| reported 5-col table | 1200 | no | wraps; **Niveau 115px** (was 58), header **43px** (was 68) |
| 2-col narrow table | 1200 | no | still fills — **RR-5ZVPC5 satisfied** |
| 12-col table | 1381 | **yes** | scrolls, cells still wrap |
| cell with 300-char URL | 1200 | no | wraps, no page scrollbar (**AC3**) |
| surrounding prose | 1200 | — | does not move; no body/page scrollbar |

`display` stays `table` and `border-collapse` stays `collapse`, so table
semantics and screen-reader behaviour are untouched — the decisive advantage
over the `display: block` variant.

The `max-width: 40ch` cap is **load-bearing, not cosmetic**: without it the
`min-width` floor prevents a long-URL cell from shrinking and the table blew out
to 2111px (measured), breaking AC3. The two bounds must be introduced together.

*Change 1 — cell wrapping.* `overflow-wrap: break-word` + the `min-width` /
`max-width` bounds on `.md-body th, .md-body td`. `.md-body`'s own
`overflow-wrap: anywhere` stays for prose, where it is correct.

*Change 2 — per-table scroll.* Move `overflow-x` off `.md-body` onto a per-table
scroll container so prose no longer slides sideways. Mechanism, now decided (the
earlier draft deferred this, which was a gap): inject the wrapper **client-side
in a shared helper**, not in goldmark. DocumentView and DocumentsPanel run the
server HTML through `DOMPurify.sanitize` in the browser (`DocumentView.vue:61`),
so one shared post-sanitize step covers the server-rendered surfaces *and*
`renderMarkdown`'s output with a single implementation — no client/server drift,
and goldmark stays untouched.

*Change 3 — body width.* Raise `.entity-detail`'s `max-width` from 1200px toward
~1600px so wide screens are used. Prose line length is a real constraint, so
widen rather than remove, and verify the field layout from TKT-5V8704
(single-column default + authored span) still reads correctly at the new width.

**Surface-specific constraints discovered:**

- **EasyMDE preview has no wrapper** — its rules are mirrored by
`markdownContentMirror.test.ts`, which compares every `.editor-preview`
*descendant* rule. The cell bounds mirror fine; the wrapper does not exist
there, so that surface keeps body-level overflow. This must be stated in the CSS
comment rather than silently differing.
- **Milkdown must be excluded from the cell bounds.** Its column-resize handles
(`milkdownEditor.css:188-196`) set explicit per-column widths that a
`min-width`/`max-width` floor would fight. The ProseMirror root carries
`md-body` (`MilkdownEditor.vue:529`), so the bounds need a scope that excludes
it (or a Milkdown-side reset).

**Files to modify:**
- `frontend/src/styles/markdown-content.css` — cell bounds, table rule, the
`.md-body` overflow-x move, the `.editor-preview` mirror block, and the comment
at `:216-233` (which currently asserts something false)
- `frontend/src/app-editor/relaEditorTheme.css` — mirror (~207-221)
- `frontend/src/components/entity/EntityDetail.vue` — `.entity-detail` max-width (:2377)
- the shared sanitize/wrapper helper + its call sites in `DocumentView.vue`,
`DocumentsPanel.vue`, `utils/markdown.ts`
- `frontend/src/components/forms/milkdown/milkdownEditor.css` — scope/reset

**Alternatives rejected:**
- *`white-space: nowrap`* — produces scrolling (measured 2537px) but turns
sentence cells into single long lines.
- *`display: block` on the table* — measured working, but drops table display
semantics and re-triggers RR-5ZVPC5's narrow-table finding.
- *`table-layout: fixed`* — equalises columns; worse for content-varying tables.
- *Removing the width cap* — unbounded prose line length on ultrawide monitors.
- *Wrapper in `renderMarkdown` only* — misses the two server-rendered surfaces.

## Security Considerations

- [x] Input sources identified
- [x] Input validation approach defined
- [x] Security-sensitive operations identified
- [x] Error handling doesn't leak sensitive information

**Input sources & validation.** Markdown bodies are author-supplied and already
sanitized: client renders via `DOMPurify.sanitize` with an explicit `ADD_ATTR`
allowlist (`markdown.ts:115-118`); server renders via goldmark plus a second
client-side `DOMPurify.sanitize` (`DocumentView.vue:61`).

**The one real security-relevant decision is wrapper-injection ordering.** The
wrapper must be inserted **after** sanitization, by walking the sanitized DOM —
never by string-concatenating HTML before or after `DOMPurify.sanitize`, and
never by re-parsing sanitized output through an unsanitized path. Injecting a
static-class `div` via DOM APIs adds no attacker-controlled attribute. If the
helper is instead written to emit markup textually, it must run inside the
existing sanitize step, not around it. `markdown.ts:110-114` documents that
unlisted attributes vanish *silently*, so the wrapper class must be verified to
survive rather than assumed.

No ACL, storage, exec or network path is touched; per CLAUDE.md this changes
presentation of already-authorized, already-redacted content and adds no read
surface.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined
- [x] Integration test approach defined

jsdom does no layout, so width assertions run in a **real browser** (Puppeteer),
as TKT-YYZRGW's verification did. Baselines above are the regression floor.

- **AC1** → reported table at a 1200px column: Niveau column ≥ 80px (target 115)
and header row ≤ 2 line-heights (target 43px). Baseline 58px/68px.
- **AC2** → 12-column table: wrapper `scrollWidth > clientWidth` (target 1381),
sibling `<p>` `offsetWidth` == body width, body `scrollWidth == clientWidth`.
- **AC3** → 300-char URL cell: table stays at column width (target 1200, **not**
the 2111px seen without the `max-width` cap); `documentElement.scrollWidth ==
clientWidth`.
- **AC4** → `markdownContentMirror.test.ts` green; manual pass over
DocumentView, DocumentsPanel, Milkdown (incl. exercising column resize) and the
EasyMDE preview, evidence recorded in the implementation checklist.
- **AC5** → both themes; no new colour literals in the diff.

**Edge cases.** 2-column narrow table (must still fill — RR-5ZVPC5); 12+
columns; empty cells (the reported table has one); links/inline code in cells;
mobile (`@media max-width: 720px`) unaffected by the widened cap; Milkdown
user-resized columns; a table as the very first/last child (wrapper margin
collapsing). RTL is unsupported app-wide and is not tested.

**Negative tests.** A table that fits must assert `scrollWidth === clientWidth`
— the assertion whose absence let TKT-YYZRGW ship a false criterion.

**Integration.** Puppeteer against a live `just dev` server rendering a real
entity containing the reported table, in addition to the synthetic fixture that
reproduced the bug during planning.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed
- [x] Effort estimated

| Risk | Mitigation |
|---|---|
| `min-width` floor stops long-URL cells shrinking (**measured**: 2111px, breaks AC3) | Ship `max-width: 40ch` together with the floor; AC3 test pins it |
| Milkdown column-resize fights the cell bounds | Exclude Milkdown from the bounds; manually exercise resize before review |
| Mirror copies drift | `markdownContentMirror.test.ts` fails CI on drift |
| Wrapper injection bypasses sanitization | Wrap post-sanitize via DOM walk; never string-concatenate HTML |
| Widening the cap hurts prose or the TKT-5V8704 field layout | Widen, don't remove; visual check both themes; fall back to widening only the content column |
| `12ch`/`40ch` are magic numbers | Record the measurements in the CSS comment so the next person can retune with evidence |

**Effort:** `s` — CSS-scoped plus one shared helper; browser verification is the
bulk of the work.

## Documentation Planning

- [x] User-facing docs identified
- [x] Docs-checklist will be created when entering implementation

**Documentation impact:** no end-user docs — a rendering fix with no new
behaviour. The load-bearing documentation is the **code comment** at
`markdown-content.css:216-233`, which currently asserts wide tables scroll. That
claim is false; it must be replaced with the measured behaviour and the reason
the `min-width`/`max-width` pair exists, so the next person does not re-derive
it or remove one bound.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** RR-5SURSS (critical, addressed) and RR-B22351
(critical, addressed), both linked to TKT-8WH1KZ.

The review overturned two things this plan had asserted, and the approach
changed as a result rather than being defended:

- The `display: block` mechanism this plan preferred would have dropped table
  roles from the accessibility tree (RR-5SURSS). Dropped entirely; the table
  keeps `display: table` and a wrapper element owns the overflow.
- A scroll container with no `tabindex` is unreachable by keyboard, and the
  pre-existing `.md-body { overflow-x: auto }` had that defect **dormantly** —
  fixing the wrap bug would have activated it (RR-B22351). The wrapper now ships
  `tabindex="0"` + `role="region"` + an accessible name.

Also corrected during planning, before implementation: the "wrap XOR scroll"
claim was false (a `min-width` floor sets where wrapping stops and scrolling
starts), and the `min-width` floor alone breaks the long-URL case without the
`max-width` ceiling.

Deferred to its own ticket on the review's advice: widening the
`.entity-detail` 1200px cap. It is shared by four files, so widening one makes
the page jump width between list and detail, and it is a preference change
rather than part of this bug fix.
