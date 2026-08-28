---
id: IMPL-1CWS9C
type: implementation-checklist
title: 'Implementation: Standalone documents: document: as a navigation entry with optional entity_type'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

Layers changed:

- `internal/dataentryconfig` — `NavigationEntry.Document`, `DocumentConfig.Permission`,
`DocumentConfig.IsStandalone()`, relaxed `entity_type` requirement, nav document
validation, `edit:`-on-standalone rejection.
- `internal/lua` — `WithStandaloneDocumentMode`; the existing
`documentEntry != ""` guard already yields nil `entry_id`.
- `internal/script` — `Engine.ExecuteStandaloneDocument` (typed seam beside
`ExecuteDocument`/`ExecuteListDocument`).
- `internal/dataentry` — path-shape split in `handleV1Documents`,
`handleV1StandaloneDocument`, `gateDocumentPermission`,
`documentService.RenderStandalone`, sidebar href + `hidesNavEntry` filter.
- `frontend` — `/document/:name` route, optional `entityId` prop, API client
path split, TS type fixes.

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

Deny-path tests assert **renderer call count == 0**, not merely the status code
— a 404 alone would still pass if the Lua aggregation had already run.

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

Ran `rela-server` against `prototypes/data-entry/project` with a real standalone
document (`docs/status_review.lua`, aggregating tickets × categories) and a
`document:` nav entry.

| # | Criterion | Evidence |
|---|-----------|----------|
| 1 | Standalone config loads | Server started; config validated |
| 2-4 | Nav `document:` validation | Go table tests; server accepted the valid entry |
| 5 | `GET /_documents/{name}` renders | 200, real aggregated content, `cached=false`, `entity_ids=[]` |
| 6 | Kind mismatch both ways | standalone+id → 400; anchored w/o id → 400 with actionable message |
| 7 | `entry_id` nil | Script's `assert(rela.document.entry_id == nil)` passed on a live render |
| 8 | `permission:` gate | alice (holds `report:status`) → 200; bob → 404, renderer never logged |
| 8 | Non-enumerable deny | Deny body identical to unknown-doc body apart from `instance` (echoes caller's own path) |
| 9 | Ungated by default | Without `permission:`, every principal renders |
| 10 | Sidebar href + filtering | `/document/status_review` under Dashboard; absent for bob, present for alice |
| — | No regression | Entity-anchored `category_overview/backend` → 200 with `entity_ids=['backend']` |

Browser (screenshots taken): sidebar entry renders beneath Dashboard with a
document icon; clicking it loads the report; direct deep-link to
`/document/status_review` works; active-route highlight correct; no Edit button
(correct — no entity); entity-anchored view unchanged.

**Bug found and fixed during manual verification:** `DocumentView.vue` hardcoded
`<h1>{{ docTitle }}: {{ entityId }}</h1>`, rendering a dangling "Status Review:"
for standalone documents. Also fixed the empty-state message, which would have
read `the entity "" may not exist`. Neither was reachable by the unit tests —
only by looking at the page.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

Notes:

- `RenderStandalone` mirrors `RenderListMarkdown` (script-only, no hash/cache/
singleflight) with the same documented reasoning; `WithStandaloneDocumentMode`
mirrors `WithListDocumentMode` rather than passing `""` to `WithDocumentMode`.
- **plimsoll caught a real signal**: the two new handlers pushed `App` from 89
to 91 methods, over its pinned load line of 90. Fixed by making both plain
functions (they are leaf handlers) rather than raising the directive — the
guidance's "prefer splitting the type over raising the number".
- `permission:` is not cross-checked against `acl.yaml`, matching the existing
precedent for command permissions (the policy isn't available at
config-validation time). Noted rather than plumbed through; see the ticket
follow-ups.
- Test-config changes to `prototypes/.../acl.yaml` were reverted after
verification; only the intentional standalone example remains.

`go test ./...`, `just lint` (0 issues), `just arch-lint`, `just plimsoll`,
`just coverage-check` (76.8%), frontend `test:run` (1437 passed), `typecheck`,
and `lint` (0 errors) all pass.
