---
id: PLAN-V6BB
type: planning-checklist
title: 'Planning: Resolve entity-ID code spans to titled links in data-entry views'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

In scope:

- Backend computes, for each view-fetch response, the set of entity IDs that
appear as bare-content code spans in any markdown body the response carries
(entry content + content sections), resolves each to `{type, title}` via the
store, and attaches it as a new top-level `mentions` field on the `ViewResponse`
returned by `/_views/<type>/<id>`. This is an **implicit relation set** —
derived at read time, not declared.
- Frontend `renderMarkdown` learns an optional sync `refResolver:
(id) => {type, title} | null` parameter and uses marked's `walkTokens` to
rewrite `codespan` tokens whose `text` exactly matches a known entity ID into
`link` tokens (`href = /entity/<type>/<id>`, link text = title).
- `EntityDetail.vue` (the unified detail screen after the TKT-J5BET merge)
builds the resolver from `viewData.value?.mentions ?? {}` and passes it to every
`renderMarkdown` call it makes.

Out of scope:

- Lua/document-render path — `rela.md.resolve_refs` already covers it
(TKT-LXYHQ + FEAT-023).
- Wiki-style `[[ID]]` syntax (IDEA-011).
- Bare prose mentions outside code spans.
- Hover previews, backlinks panel, graph affordances.
- `MarkdownEditor` source-edit view — only **rendered** output is rewritten.
- Generic entity-fetch endpoint (`/api/v1/<plural>/<id>`) — only the view
fetch carries `mentions` for now, because that's what the detail screen
consumes. Other consumers can opt in later.
- Server-side rewriting of the markdown payload itself — the markdown stays
raw on the wire; the resolver is what gives the renderer everything it needs.

**Acceptance Criteria:**

1. **Known-ID code span → titled link.** A view response for an entity whose
content includes `` `TKT-LXYHQ` `` carries `mentions["TKT-LXYHQ"] =
{"type":"ticket","title":"Resolve entity-ID references…"}  `. The SPA renders
that code span as `<a href="/entity/ticket/TKT-LXYHQ">Resolve entity-ID
references…</a>  `.
2. **Manual-ID code span → titled link.** Same flow works for `id_type:
manual  ` entities (e.g. `` `data-entry-ui` `` → link to
`/entity/concept/data-entry-ui  `). Verified server-side via store resolution,
not a prefix map.
3. **Unknown-ID code span → unchanged.** `` `TKT-NOPE` `` (no such entity)
stays rendered as `<code>TKT-NOPE</code>  `. Server's `mentions  ` doesn't
include it; resolver returns null; renderer falls through.
4. **Multi-token code span → unchanged.** `` `TKT-1 and TKT-2` ``,
`` `TKT-LXYHQ extras` `` — exact-match-only, matching Lua semantics. No entry in
`mentions  `; renderer leaves them as `<code>  `.
5. **Code block / link contexts untouched.** Fenced/indented code blocks
containing IDs and existing `[label](url)  ` link text with IDs are not
rewritten. The walkTokens hook only fires on `codespan  ` (not `code  `), and
tokens inside an already-formed `link  ` are not match candidates because marked
never tokenizes link **text** as standalone `codespan  ` for the link-as-a-whole
shape `[TKT-1](url)  `.
6. **DOMPurify-safe.** Titles containing `<  `, `>  `, `"  `, `&  ` flow through
marked's link renderer, which HTML-escapes link text. Final DOMPurify pass keeps
the link intact. Unit test asserts both no-XSS and no-mangled-title.
7. **Self-reference.** Entity references itself in content (`` `<own-id>` ``)
→ still rewritten; href routes to its own detail screen — harmless.
8. **Inaccessible target.** Code span pointing at a git-crypt-encrypted
entity → mention carries `inaccessible: true, inaccessible_reason: "git-crypt"
`; SPA renders `<a href="…">ID 🔒</a> ` with the standard git-crypt tooltip copy.
Link still navigates to the target detail page.
9. **Manual end-to-end.** With `npm run dev  ` running, opening an entity
whose content references another entity by ID in backticks shows the link,
clicking navigates correctly.
10. **Playwright e2e.** New spec under `e2e/tests/ ` seeds a project where
entity A's content references entity B in backticks, visits A's detail page,
asserts the rendered link points at B's URL and carries B's title as text, and
clicks through to verify navigation. Covers the happy path end-to-end against
the built `rela-server ` binary that embeds the SPA bundle.

## Research

- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Existing Solutions:**

- **Server-side reference (Lua):** `internal/lua/markdown.go  ` (TKT-LXYHQ).
`rela.md.entity_refs  ` walks the store and builds an ID→link-text map;
`rela.md.resolve_refs  ` walks the AST and rewrites `code_span  ` nodes whose
entire text matches a known ID. We mirror the AST-walk semantics on the server
(goldmark) and the resolution-map shape on the wire.
- **Server-side renderer (goldmark):** `internal/dataentry/helpers.go  `
(`simpleMarkdownToHTML  ` for help content) and `internal/dataentry/document.go
` (FEAT-023 document panels) already use goldmark. We reuse goldmark's parser
for the code-span scan but do **not** render to HTML here — we just walk the AST
collecting matched IDs.
- **Frontend renderer (`marked  `):** `frontend/src/utils/markdown.ts  `'s
`renderMarkdown  ` is the only call site for `marked.parse  `. Marked exposes a
`walkTokens  ` option that fires once per parsed token, so we can mutate a
`codespan  ` token into a `link  ` token before render. Confirmed shape: a `link
` token is `{type:'link', href, title, text, tokens:[{type:'text', raw:...,
text:...}]}  `; marked renders it via its default link template, which
HTML-escapes link text correctly.
- **Unified detail screen (recent on develop):** commit 6aa0b0e
("Merge EntityDetail and CustomView into a single config-driven detail screen")
landed on 2026-05-10 — `CustomView.vue  ` is gone; every entity type uses
`EntityDetail.vue  ` via `EntityView.vue  `. One render path, one `fetchView  `
call, one place to plug in the resolver.
- **Wire shape:** `ViewResponse  ` (frontend `api/views.ts:105  ` + backend
views.go) is the natural carrier. `entry.relations  ` is a flat `Record<string,
string[]>  ` keyed by metamodel relation types — squatting on that would confuse
analyzers and the schema-keyed UI. A separate top-level `mentions  ` field keeps
the implicit/derived edges distinct from explicit metamodel relations.
- **Route + helper:** `/entity/:type/:id  ` (router/index.ts) via
`entityDetailHref  ` (utils/entityRoute.ts). Reused as the link target.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

1. **Server-side mentions collection** (`internal/dataentry/  `).
   - Add a small helper `collectMentions(state, contents []string) map[string]Mention  `
that:
     - Concatenates the content blobs and parses them once with goldmark.
     - Walks the AST collecting every `ast.KindCodeSpan  ` whose entire text
value matches the syntactic ID shape (one short non-space token; the precise
regex matches the Lua side — keep them in lock-step).
     - Looks each candidate ID up in `state.Store  ` (via the entitymanager /
store helpers already used by view-rendering code). On hit, records `{Type:
e.Type, Title: e.Title()}  `; if the entity is inaccessible (git-crypt) the
record also carries `Inaccessible: true, InaccessibleReason: "git-crypt" `. On
miss, drops it silently.
   - Wire it into the view-response builder so the returned `ViewResponse  `
carries `Mentions map[string]Mention  ` — populated from `entry.Content  ` and
from each section's `Content  ` field.
2. **API wire shape.**
   - Add `Mention { type: string, title: string, inaccessible?: bool,
inaccessible_reason?: string }  ` and `Mentions map[string]Mention
`json:"mentions,omitempty"`` to the view-response Go type. `inaccessible ` and
`inaccessible_reason ` are omitempty so the common case stays compact. Mirror in
`ViewResponse  ` TS interface in `frontend/src/api/views.ts  `.
3. **Frontend resolver.**
   - Mirror the wire shape in `ViewResponse.mentions  `.
   - In `EntityDetail.vue  `, derive a `refResolver  ` `computed  ` from
`viewData.value?.mentions  ` — `(id) => mentions[id] ?? null  `.
   - Pass the resolver to every `renderMarkdown(content)  ` call the component
makes (currently three sites for entry content, content sections, and
entity-card content).
4. **`renderMarkdown  ` extension** (`frontend/src/utils/markdown.ts  `).
   - Add optional second arg `refResolver?: (id: string) => { type: string;
title: string; inaccessible?: boolean; inaccessibleReason?: string } | null  `.
   - Configure `marked.parse  ` with `walkTokens: token => { ... }  `. For each
`codespan  ` whose `.text  ` matches the syntactic ID regex and whose resolver
lookup returns a hit, mutate the token in place into a `link  ` token: set
`type='link'  `, `href=/entity/<type>/<id>  `, `title=undefined  ` (don't reuse
the HTML `title  ` attribute — keeps the surface minimal), `text=<entity-title>
`, `tokens=[{type:'text', raw:title, text:title}]  `. Marked's link renderer
handles HTML-escaping the text.
   - **Inaccessible entities:** when the resolver returns `inaccessible:
true `, use the **ID** as link text (since the title isn't readable) and append
a 🔒 trailing affordance inside the link via a second `text ` token. Set the
`title ` attribute on the resulting `<a> ` to the existing `inaccessibleTooltip
` copy from `PropertyDisplay.vue ` (`"git-crypt encrypted (run 'git-crypt
unlock' to read)" ` for the `git-crypt ` reason; otherwise `"inaccessible
(<reason>)" `). This reuses the visual language users already see on
inaccessible properties.
   - No-resolver and no-hit cases: do nothing — output stays as today.
5. **No new sanitizer config.** DOMPurify already permits `<a href>  ` on
same-origin paths starting with `/  `. We don't add `data-*  ` attributes.

**Files to modify:**

Server:
- `internal/dataentry/sections.go  ` and/or
`internal/dataentry/default_view.go  ` — wire `collectMentions  ` into the view
response builder.
- `internal/dataentry/api_v1.go  ` (or wherever the view-response Go type
lives — `default_view.go  ` per the recent merge commit) — add `Mention  ` type
and `Mentions  ` field.
- New: `internal/dataentry/mentions.go  ` (or co-locate in `sections.go  `) —
goldmark-based scan + store resolution.
- New: `internal/dataentry/mentions_test.go  ` — table-driven tests for the
scan (known/manual/unknown/multi-token/inside-code-block).

Frontend:
- `frontend/src/api/views.ts  ` — add `Mention  ` and `mentions  ` to
`ViewResponse  `.
- `frontend/src/utils/markdown.ts  ` — extend `renderMarkdown  ` with
`refResolver  `; add walkTokens rewrite.
- `frontend/src/utils/markdown.test.ts  ` — add cases (see Test Plan).
- `frontend/src/components/entity/EntityDetail.vue  ` — derive resolver from
`viewData.value.mentions  `; pass to all three `renderMarkdown  ` call sites.

**Alternatives considered (rejected):**

- Client-side prefix map from schema `IDPrefix  ` — rejected: misses `id_type:
manual  ` entities (e.g. concepts like `data-entry-ui  `).
- Standalone `GET /api/v1/_entity_refs  ` endpoint covering the whole project
— rejected per user: payload bigger than necessary; only the IDs referenced by
the currently-viewed entity matter.
- Server pre-rewrites markdown into markdown links before returning content
— rejected: changes content semantics on the wire, harder to reason about for
downstream consumers (export, copy-content, etc.).
- Two-phase client render (scan content for IDs, fetch missing entities,
re-render) — rejected: visible flicker, async glue in render path, no benefit
over a server-supplied per-response map.

**Dependencies:** None new. goldmark + GFM extensions already imported in
`internal/dataentry/  `; `marked  ` already in the frontend bundle.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- **Entity content (markdown body).** Author-controlled. Already rendered
via marked + DOMPurify today. The new rewrite happens at the **token level**
before HTML serialization, so HTML-significant chars are escaped by marked's
renderer like any other inline.
- **Entity titles** (link text). Author-controlled. Risk: `<script>  ` in a
title could break out if injected as raw HTML. **Mitigation:** title is pushed
into a `text  ` token (`tokens:[{type:'text', text: title}]  `); marked's link
renderer HTML-escapes link text. Final DOMPurify pass is a second line of
defense, not the primary defense.
- **ID syntax matched against** a small anchored regex on the server (same
shape as the Lua side: one short non-space token). Only matched candidates are
looked up in the store. Unknown IDs are silently dropped.
- **href**: built server-side from validated `(type, id)  ` strings already
in the store. Same-origin path `/entity/<type>/<id>  `. No query strings, no
user-controllable URL parts.
- **Inaccessible entities (git-crypt):** the store's `Title()  ` may return
the ID for an inaccessible entity. That's still a valid link target — the detail
screen handles inaccessible entities (commit 6dc2efa). Acceptable.

**Security-Sensitive Operations:**

- Title-as-link-text composition: rely on marked's link renderer for
escaping; DOMPurify final pass.
- Server goldmark parse is on already-loaded markdown; no extra IO.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

| AC | Test | Layer |
|----|------|-------|
| 1 | View response for an entity whose body has `` `TKT-LXYHQ` `` includes `mentions["TKT-LXYHQ"]  ` and SPA renders an `<a>  ` with the target title | Go (`mentions_test.go  `) + TS (`markdown.test.ts  `) |
| 2 | Same flow with a manual ID concept (e.g. `data-entry-ui  `) | Go + TS |
| 3 | View response for `` `TKT-NOPE` `` does NOT include the ID in `mentions  `; renderer leaves `<code>  ` | Go + TS |
| 4 | `` `TKT-1 and TKT-2` `` → not collected, not rewritten | Go + TS |
| 5 | Content with ID inside a fenced/indented code block → not collected; existing `[TKT-1](url)  ` link text → not rewritten | Go (server skips code blocks); TS (`walkTokens  ` only fires on `codespan  `) |
| 6 | Title `<img onerror=x>  ` → link text rendered as escaped text; no DOM injection; DOMPurify pass preserves the link | TS |
| 7 | Self-reference: entity content includes its own ID; link appears with own title | Go + TS |
| 8 | Inaccessible target: server emits `inaccessible:true, inaccessible_reason:"git-crypt" `; renderer outputs `<a>ID 🔒</a> ` with tooltip | Go + TS |
| 9 | Manual run: dev server, click a rendered link, verify navigation | Manual |
| 10 | Playwright e2e: built `rela-server ` serves SPA; spec asserts rendered link + click navigates | e2e (`e2e/tests/entity-refs.spec.ts  `) |

**Edge Cases:**

- Empty content → empty `mentions: {}  `; renderer fast path returns `''  `.
- Resolver not passed → `renderMarkdown  ` behaves exactly like today
(backwards-compatible). Existing tests stay green.
- Resolver throws (defensive) → swallow; leave codespan intact.
- Unicode title → preserved verbatim by marked's text-token escaping.
- Many refs in one document → single goldmark walk server-side; single
`walkTokens  ` pass client-side. O(n) in token count.
- Mixed known/unknown in one paragraph → each handled independently.
- Inaccessible entity title (git-crypt) → server-side resolution detects
the entity is inaccessible and surfaces it in the `mentions ` map with an
`inaccessible ` flag and the reason (`"git-crypt" `). The frontend renders the
link with the ID as text plus a trailing 🔒 lock affordance, mirroring the
existing `PropertyDisplay.vue ` pattern (commit 6dc2efa). Tooltip on the lock
gives `"git-crypt encrypted (run 'git-crypt unlock' to read)" ` — same copy as
`PropertyDisplay `'s `inaccessibleTooltip `. The link still navigates to the
entity's detail page (where the existing inaccessible handling takes over). Wire
shape: `Mention { type, title, inaccessible?, inaccessibleReason? } `.

**Negative Tests:**

- `mentions  ` includes an entry whose value is missing `type  ` or `title  `
→ frontend resolver defensively returns null, codespan untouched.

**Integration approach:**

- **Go:** add a focused unit test in `internal/dataentry/mentions_test.go  `
for the scan + resolve logic against an in-memory store/metamodel. Existing
view-response tests get extra assertions for the `Mentions  ` field on the JSON
payload.
- **TS:** `markdown.test.ts  ` extends with `walkTokens  ` cases (the existing
Vitest harness uses JSDOM; perfect for the DOM-walking that DOMPurify does at
the end).
- **Playwright e2e:** new `e2e/tests/entity-refs.spec.ts  ` against the
built `rela-server ` binary that embeds the SPA bundle. Mirrors the existing
patterns in `e2e/tests/entity-detail.spec.ts ` and `git-crypt.spec.ts `. Seeds a
fixture project (or uses an existing fixture) with two entities where one
references the other in a backticked code span; asserts the rendered `<a> `
exists with the expected href and title text, then clicks it and asserts
navigation to the target detail page (AC 10).
- **Manual:** dev server end-to-end click-through (AC 9).

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- **Goldmark parse cost per view fetch.** The content is small (a single
entity's body + section blobs); negligible vs. the rest of the view rendering.
Mitigation: cheap.
- **Store lookup cost.** A handful of `Get(type, id)  ` per view fetch.
Negligible.
- **Wire-payload growth.** `mentions  ` adds ~50–150 bytes per referenced
entity. For a typical entity with 0–10 refs, payload growth is in the low
hundreds of bytes. Acceptable.
- **Marked token-mutation contract.** Marked's `walkTokens  ` lets you mutate
the token in place; the contract is documented and stable. Mitigation: add an
explicit unit test asserting the mutation pattern, so a future marked upgrade
that changes the token shape fails loudly.
- **Inaccessible entity titles.** Surfaced explicitly via `inaccessible` and
`inaccessible_reason` on the mention record; rendered as ID + 🔒 with the same
tooltip copy as inaccessible properties (see Approach §4).

**Effort:** `m  ` (matches the ticket's recorded effort).

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [ ] User guide: brief mention in the data-entry guide (rela-docs) that
backticked entity IDs in markdown content become clickable links — same
convention as the Lua side. Create a `docs-checklist  ` on the ticket when
transitioning to review unless the user opts out.
- [x] N/A — CLAUDE.md, README.md, internal architecture docs unaffected.
- [x] N/A — no new CLI flags / no new API surface beyond `mentions  ` on
`/_views/<type>/<id>  `.

## Design Review

- [ ] Run `/design-review  ` before starting implementation
- [ ] All critical/significant findings addressed in plan

**Design Review Findings:** <!-- populated after running /design-review -->
