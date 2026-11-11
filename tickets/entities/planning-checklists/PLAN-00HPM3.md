---
id: PLAN-00HPM3
type: planning-checklist
title: 'Planning: Follow a retired id to its successor on the entity detail page'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

In scope:
- `GET /api/v1/_views/{type}/{id}` and `GET /api/v1/{plural}/{id}`: when the id does not resolve and the store maps it (store.RetiredIDs) to a successor the caller may read, answer `308 Permanent Redirect` to the successor's address.
- SPA entity detail page: when the view that lands names another entity than the route, `router.replace` to that entity's page (world query and hash kept).

Out of scope:
- fsstore/memstore (no retired-id mapping; they rewrite mention text instead).
- Writes (PATCH, DELETE, relations): an old id stays not found. Redirecting a write would apply it to an entity the caller did not name.
- MCP `show_entity` and the CLI.
- Chains of conversions are already flattened by the store (`Successors` returns the current id).

**Acceptance Criteria:**
1. A reader who may read the successor and opens `/entity/topic/TOPIC-AOWJ` (converted to `INIT-P2HD`) lands on `/entity/initiatief/INIT-P2HD`; the browser URL shows the new address.
2. A reader who may not read the successor gets exactly the response a never-existing id gets (same status, same body).
3. A live id is never redirected, and a never-existing id answers as today.
4. The `world` query parameter survives the redirect.
5. `GET /api/v1/{plural}/{oldId}` redirects to `/api/v1/{newPlural}/{newId}` under the same rules.

## Research

- [x] ~~For larger features: run `/research`~~ (N/A: small change on an existing seam)
- [x] ~~Searched for existing libraries~~ (N/A: plain HTTP 308)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: standard moved-resource redirect)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:**
- `visibility.Resolver.ResolveMentions` (internal/visibility/mentions.go:196) already maps a retired id to its gated successor header for @-mentions. It is the only sanctioned reader of `store.RetiredIDs` (archguard mentionindex_test.go).
- The SPA already redirects in `loadView`'s `prepare` hook for owned entities (EntityDetail.vue, `_owner` → `router.replace`). The new case reuses that hook.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

Server (internal/dataentry):
1. `retiredRedirect(w, r, world, id, pathFor func(type, id) string) bool`: calls `viewReader.Resolver().ResolveMentions(ctx, world, []string{id})`. If the header it returns has another id, write `308` with a RELATIVE `Location` (so a path prefix in front of `/api` keeps working) that keeps the raw query, and return true. Otherwise return false and the caller writes its usual miss response.
2. It runs ONLY on a miss, so a live read costs nothing extra:
   - `handleV1Views`: on read-gate deny (today 404) and on `errViewEntryNotFound` (today 422) and on the world-absent miss. The gate call is split so the handler can try the redirect before writing the 404.
   - `handleV1GetEntity`: on the `readAddressedOr404` miss (a new read-only variant that does not write on a miss; writes keep `readAddressedOr404`).
3. The face suffix is dropped: the successor type may not declare that face. The world still picks the face.

SPA (frontend):
4. In `EntityDetail.vue` `loadView` `prepare`: if `view.entry` has another bare id or type than the route, `router.replace` to `entityDetailHref(view.entry)` with the world query and hash, and return false. Axios follows the 308 itself, so the SPA only sees the successor's view.

Alternatives rejected:
- Serve the successor's view under the old URL without a redirect: the page and its live subscription, autosave and comments would stay keyed to an id that no longer exists.
- A not-found body carrying the new address: every client has to learn it; a 308 works for any HTTP client.
- Resolve on every read: one extra gated query per page load for a rare case.

**Files to modify:**
- internal/dataentry/views_handler.go (miss paths), internal/dataentry/api_v1.go (entity GET), new internal/dataentry/retired_redirect.go (+ _test.go)
- internal/dataentry/visiblereader.go (non-writing address read, if needed)
- frontend/src/components/entity/EntityDetail.vue (+ test)
- docs-project api-reference entry for `_views` / entity GET (308 note)

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**
- The path id: already parsed by `entity.ParseRef`; a malformed address stays the uniform 404 and is never looked up.
- `Location` is built from the successor header's type and id (validated store values), path-escaped, plus the request's own raw query. No open redirect: the target is always a relative path inside the API.

**Security-Sensitive Operations:**
- Existence disclosure: the redirect fires only when the successor is readable in the request's world (ResolveMentions applies the row gate, face gate and world). An unreadable successor falls through to the exact miss response. The old→new mapping of a readable successor is already disclosed by @-mention resolution, so no new channel.
- Writes are never redirected.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**
- AC1/AC5: Go handler tests on a store with RetiredIDs (sqlitestore or a memstore wrapper with a fake Successors), converting A→B, then GET `_views/a/A` and `GET /{plural}/A`: 308, Location `../b/B` (relative), keeps `?world=`.
- AC2: same, with an ACL that hides B: response byte-identical to a GET of a never-existing id.
- AC3: live id: 200 and no Location; unknown id: today's answer.
- AC4: query kept in Location.
- SPA: EntityDetail test where the view for `topic/T-1` returns entry `initiatief/I-1`: router.replace to `/entity/initiatief/I-1` with world query.
- Manual: atlas on localhost:8891, open `/entity/topic/TOPIC-AOWJ` in the browser.

**Edge Cases:**
- Old id with a face suffix (`TOPIC-AOWJ@draft`): redirect to the bare successor.
- Successor of another type: Location uses the successor's type.
- Store without RetiredIDs: no redirect, unchanged.
- ResolveMentions error: written as the gate error, as other read failures are.

**Negative Tests:**
- Hidden successor → uniform miss; PATCH on the old id → still 404.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**
- Relative Location resolution differs per client: RFC 9110 allows it and browsers and axios resolve it against the request URL; test the exact value.
- SPA redirect loop if the server returns a view whose id differs for another reason (owned entity is handled first; bare-id comparison ignores faces).

Effort: m

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
- docs-project data-entry API reference: note the 308 for a retired id on the two GET routes.

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: small change on existing gated seams; the security reviewer runs in the review phase)
- [x] ~~All critical/significant findings addressed in plan~~ (N/A: see above)

**Design Review Findings:** none
