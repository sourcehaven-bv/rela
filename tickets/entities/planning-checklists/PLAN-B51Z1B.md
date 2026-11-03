---
id: PLAN-B51Z1B
type: planning-checklist
title: 'Planning: Entity references to faced types never resolve as mentions'
started: "2026-10-11"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

In: `collectMentions` (`internal/dataentry/mentions.go`) resolves its candidate
ids in the request's world through the bare-id resolver, for both callers
(entity GET `api_v1.go:1032`, view GET`views_handler.go:1078`).

Out: `ID@face` spans as explicit references;`entityReader.defaultWorldHeaders`
(documented as default-world only); Lua `rela.md.entity_refs`/`resolve_refs`
(already world-aware via `lua.Deps.World`); carrying`?world=` on the SPA's
rendered mention links (see Risks).

**Acceptance Criteria:**

1. A faced reference whose world-served face is readable is in `mentions`
with that face's display title. Test: `facedApp` (world`select: published`),
content citing `` `POL-1` `` → mention title `PUBLISHED TEXT`.
2. A faced entity with only a lower-ranked face resolves to it. Test: world
`select: [published, draft]`, entity with only `draft` → mention with the
draft title.
3. ACL trims before the world ranks: `published` denied, `draft` readable →
draft mention; no face readable → no mention (no title, no type).
4. Unfaced references unchanged: existing `mentions_test.go`,
`mentions_entity_get_test.go`, `acl_displaytitle_surfaces_test.go` pass
unmodified.
5. Redacted display property → title falls back to the id (BUG-R9EHKV).
6. A header-read fault drops all mentions and the response still succeeds
(logged).

**Scope change (2026-10-11, agreed after planning):** in the read view a
mention link carries the page's world as `?world=` when the page is
world-bound.

- AC7: world-bound page → href `entity/<type>/<id>?world=<w>` (URL-encoded);
  unbound → no query. Test in `markdown.test.ts` with a `world` option.
- Approach: `RenderMarkdownOptions` gains `world?: string`;
  `rewriteEntityRefToken` appends the query. `EntityDetail.vue` passes
  `worldParam`, the value the view was fetched with, at its three
  `renderMarkdown` calls (same rule as neighbor links, `entityTarget`). The
  mentions response needs no change: it is resolved in that same world.
- Files: `frontend/src/utils/markdown.ts`, `markdown.test.ts`,
  `frontend/src/components/entity/EntityDetail.vue`.
- Risk: with a declared `default_world`, `worldParam` is set without
  `?world=` in the URL, so links then carry the default world explicitly, as
  neighbor links already do. Out of scope: editor link nodes
  (`milkdown/entityRefNode.ts`).

## Research

- [x] For larger features: run `/research` to create a structured research doc (N/A: small fix)
- [x] Searched for existing libraries that solve this problem (N/A: in-repo resolver exists)
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects (N/A)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:**

- `visibility.Resolver.ResolveIDs`/`ResolveIDsErr` (`internal/visibility/batch.go:81-120`):
per bare id the redacted header of the face the world serves among the faces the
principal may read. ACL first, world ranks.
- `visibleReader.servedIDsErr` (`internal/dataentry/visiblereader.go:452`)
wraps it with `worldFromContext(ctx).visibility()`.
- Same pattern already in use: `viewsHandler.visibleTitles` (`views_handler.go:1380-1400`,
`idResolver` type assertion on `viewReader`), `nav_items.go:229`,
`export.go:400`.
- FEAT-9CD2MX (content states and worlds): a bare id names "the face the
current world serves".

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

1. Give `collectMentions` an id resolver instead of the raw store: the
`ResolveIDsErr(ctx, visibility.World, ids)` shape (a small local interface
next to `idResolver`, or reuse `idResolver` with an error variant).
2. Replace `loadDefaultFaceHeaders` + `vis.Filter` with one
`ResolveIDsErr(ctx, worldFromContext(ctx).visibility(), ids)`; the result is
already ACL-trimmed and redacted. Map each header via `headerEntity` into
`buildMention` (unchanged: Type, DisplayTitle, Inaccessible).
3. Callers pass the resolver they already hold:
   - `views_handler.go:1078`: `h.viewReader` (assert as in `visibleTitles`;
production `PolicyReader` and `AllowAllReader` both implement it).
   - `api_v1.go:1032`: `a.visibleReader` (`servedIDsErr`) or `a.viewReader`,
whichever the handler holds; pick the one the entity GET already uses.
4. If the reader lacks the capability, fall back to the current behaviour
(default-face rows + `vis.Filter`), as `visibleTitles` does, so tests with a
plain `visibility.Reader` keep working. Remove the fallback if every reader
implements it (check during implementation).
5. Store fault: log and return nil, as today.
6. Update the doc comments in `mentions.go:40-49` (no longer "default-face
row").

No frontend change: `rewriteEntityRefToken` links to `entity/<type>/<id>`;
the entity page resolves the bare id in the request's world, the same world the
mention was resolved in.

Alternatives rejected:
- Link with `@face` from the server: pins the reader to one face, breaks when
the world changes (Jeroen's direction: no explicit face).
- Load all faces and pick the first readable: duplicates the world ranking
`ResolveIDs` already owns.

**Files to modify:**

- `internal/dataentry/mentions.go`
- `internal/dataentry/api_v1.go` (call site)
- `internal/dataentry/views_handler.go` (call site)
- `internal/dataentry/mentions_test.go`, `acl_displaytitle_surfaces_test.go`
(signature)
- new `internal/dataentry/mentions_face_test.go`
- `internal/dataentry/visiblereader.go`: `loadDefaultFaceHeaders` stays for
`entityReader.defaultWorldHeaders`; only its comment if needed.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

Code-span text in user markdown (unchanged parsing: whole-span id matching a
type's id prefix). The world comes from `attachWorld` middleware; a
non-default world only after the grant check in `resolveWorld`; a denied world
serves nothing.

**Security-Sensitive Operations:**

Title disclosure. `ResolveIDs` trims faces by the principal's read grant
before ranking and redacts the served header, so an unreadable face never
contributes a type or title. A denied-prime fall-through only serves a face the
principal may read. Same guarantee `visibleTitles` already relies on.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

Integration tests through the router on `facedAppWith` (real world middleware,
real `PolicyReader`), asserting `mentions` on `GET /api/v1/_views/...` and
the entity GET:
- AC1: world `select: published`; source content cites `` `POL-1` ``.
- AC2: world `select: [published, draft]`; `POL-2` has only `draft`.
- AC3: declarative ACL denying `published` for policy (reuse
`facedAppWithPolicy`); then denying both faces.
- AC5: field ACL hiding `title` → mention title is the id.
- AC4: existing tests unchanged.
- AC6: unit test with a resolver stub returning an error.

**Edge Cases:**

- Id that does not exist → no mention.
- World `otherwise: exclude` and no selected face → no mention.
- Self-reference (`TKT-SELF` test) → behaves as before.
- No candidates → no resolver call, nil map.
- Mixed faced and unfaced ids in one batch → one resolver call.

**Negative Tests:**

- Unreadable faced entity: absent from `mentions` (no type, no title).
- Denied world: `mentions` empty.
- Resolver error: `mentions` empty, response 200.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- Link vs. `?world=`: mention links carry no `?world=`, so a reader browsing
a non-default world opens the target in the default world. Atlas has one world,
so no effect there. Out of scope; noted as an open question.
- `api_v1` and views handlers hold different reader fields; mitigated by
checking which reader each entity read already uses.
- Signature change of `collectMentions` touches tests; mechanical.

Effort: s.

## Documentation Planning

For enhancements: identify what documentation needs updating.

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation (DOCS-AJ6F6F)

**Documentation Impact:**

Check `GUIDE-content-states.md` / data-entry docs for a sentence on entity
references in markdown; add that a reference resolves in the reader's world.
Otherwise N/A (bug-level behaviour fix).

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: effort s; the requester reviewed the plan directly, and the review-phase cranky review covered the design, see RR-2KZCA3 and RR-QD5JRS)
- [x] ~~All critical/significant findings addressed in plan~~ (N/A: no design review was run; review-phase findings are tracked on the ticket)

**Design Review Findings:** <!-- List review-response IDs, e.g., RR-xxxx -->
