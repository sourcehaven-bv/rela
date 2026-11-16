---
id: PLAN-X8274XF
type: planning-checklist
title: 'Planning: Document and dashboard tabs on entity pages'
started: "2026-11-15"
completed: "2026-11-15"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

In scope:
- A document tab on an entity page, with `scope: root`, rendering an
entity-anchored document of the page's entity type for the anchor.
- A dashboard tab on an entity page, with `scope: root`, showing an entity
dashboard: a named dashboard with `entity_type:` whose `list:` cards each carry
`scope: {relation, direction}`.
- `GET /api/v1/_dashboard/card?anchor=<id>` computes such a card over the
rows the anchor reaches.
- The SPA tells an entity dashboard by its cards' `scope`. `GET /api/v1/_dashboard`
serves no `entity_type` (RR-UIDD3CD).
- Config validation, SPA wiring, Dutch and English strings, guide docs.

Out of scope:
- `query:` cards on an entity dashboard (they run in the SPA through
`/_search`, which has no anchor narrowing). Refused at load.
- Calendar tabs on entity pages. Still refused.
- An entity dashboard as a navigation entry or as the default dashboard.
It has no anchor there. Refused at load.
- Card links that open a narrowed list. A scoped card shows no link, and a
scoped matrix shows plain cells, because `/list/<id>` would show the whole type.

**Acceptance Criteria:**
1. A page tab `document: dpia_report` with `scope: root` on an entity page
of type `dpia` loads when the document has `entity_type: dpia`. The tab renders
`/_documents/dpia_report/<anchor>`. Test: config table test and e2e step.
2. Config load refuses: a standalone document on an entity page, a document
of another type, a document or dashboard tab without `scope: root`, an entity
dashboard of another type, the default dashboard on an entity page, a
`dashboard:` navigation entry naming an entity dashboard, `entity_type` on the
default `dashboard:`, an entity-dashboard card with `query:`, without `scope`,
with `scope: root`, or with a relation that does not connect the types, and
`scope` on a card of a plain dashboard. Test: Go table test per case.
3. `/_dashboard/card?name=d&card=i&anchor=A` counts only the list's rows
that A reaches over the card's relation, for count, breakdown, ratio, matrix,
summary and table cards. Test: Go handler test with two anchors.
4. Row-gate before fold: a hidden row related to the anchor is not counted
and its values fold into no group. Test: Go ACL test.
5. A hidden anchor and a missing one both answer the uniform 404
(`not_found`), before any row is read. Test: Go handler test.
6. An entity dashboard card without `anchor` is 400; `anchor` on a plain
dashboard card is 400. Test: Go handler test.
7. The SPA passes the anchor to the document and dashboard views, and hides
the project-wide validation card and unscoped links on an entity dashboard.
Test: Vitest and e2e.

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A: the approach reuses two existing mechanisms; no open
design space worth a research entity.

**Existing Solutions:**
- Documents are already parameterised by an entity: `documents.<d>.entity_type`
makes an entity-anchored document served at `/_documents/{name}/{entry}`, gated
on that entity with the uniform 404 (`export_document.go` `resolveDocument`,
`addressRef`). The SPA `DocumentView` takes `entityId`.
- Dashboards are parameterised only by `?name=`; a `list:` card is computed
by `dashboardCardValue` over `navStatusRows`, which gates and redacts the list's
rows before folding (`dashboard_card.go`, `nav_status.go`).
- List and kanban tabs narrow with `resolvePageScope` (`pagescope.go`): one
gated anchor read (uniform 404), one relation query, ids intersected with the
ACL-scoped rows.
- The gantt `scope: root` (anchor is the root) is the existing scalar scope.
- No external library applies.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

Scope form: both new tab kinds take the existing scalar `scope: root` ("the tab
starts at the anchor"). Why this is the smallest form:
- Document: the anchor IS the document's entry entity. The entity-anchored
document already exists, so the tab needs no new key: `scope: root` plus the
document's own `entity_type`. No server change for rendering.
- Dashboard: one tab-level relation cannot serve cards over different types
(a DPIA reaches risks, measures and processing activities over different
relations). So the relation sits on the card, as `scope: {relation, direction}`,
the same mapping and the same validation a list tab uses. The dashboard declares
`entity_type:` like an entity-anchored document, so the two kinds of dashboard
are told apart in one place and validation can check every card's relation
against the anchor type at load.

Alternatives rejected:
- A new scalar such as `scope: entity`: one more keyword with the same
meaning as root.
- A tab-level relation for a dashboard: cannot express the DPIA case.
- A per-card `anchor:` query parameter naming the relation: request-chosen
narrowing, which `pagescope.go` deliberately forbids.
- Passing `scope_page/scope_tab/anchor` into the card's list read: those name
a list tab, and a card is not one.

Server:
1. `dataentryconfig`: `DashboardConfig.EntityType`; `DashboardCard.Scope
*PageTabScope`; `IsEntityDashboard()`. Split `validateRelationScope`'s
connection check into `checkAnchorRelation` and reuse it for cards.
`validatePageTabScope` accepts document and dashboard tabs with
`validateAnchoredScope`. `navTargetErrors` gets the anchor type so a page tab
may name an entity document or dashboard of its page's type, and a navigation
entry may not name an entity dashboard.
2. `dataentry/pagescope.go`: extract `anchorReach` (gated anchor read, one
relation query, owned-edge filter, id set) from `resolvePageScope`; both the
list tab and the card use it.
3. `dashboard_handler.go`: read `anchor`; 400 when missing on an entity
dashboard or given on a plain one; resolve the anchor once, before rows.
4. `dashboard_card.go`: the anchor's reach rides on `gatedListQuery.reach`
and narrows the gated, scoped, conditioned list read by id (`listRowsFor`), as a
page scope does; then the rows fold. A scoped table card sorts in the same
narrowed read (RR-IRXJWNP). Nothing is cached; the value is per request.
5. `apiwire`: no `DashboardResponse.EntityType`; the SPA reads the cards'
`scope` instead (RR-UIDD3CD).

SPA:
1. `PageView.vue`: `scope: root` on a document tab passes `entityId`, on a
dashboard tab passes `anchor`.
2. `DashboardView.vue`: `anchor` prop, passed to `getDashboardCard`; an
entity dashboard without anchor shows an empty state; hides the validation card
and unscoped links when anchored.
3. `DashboardMatrix.vue`: `unlinked` prop to render plain cells.
4. Types and i18n (en, nl).

**Files to modify:**
- internal/dataentryconfig/{config.go,page.go,validate.go,dashboard.go}
- internal/dataentryconfig/page_test.go (new cases)
- internal/dataentry/{pagescope.go,dashboard_handler.go,dashboard_card.go,views_handler.go}
- internal/dataentry/dashboard_anchor_test.go (new)
- internal/apiwire/v1/responses.go
- frontend/src/views/{PageView.vue,DashboardView.vue}
- frontend/src/components/dashboard/DashboardMatrix.vue
- frontend/src/api/dashboard.ts, frontend/src/types/config.ts
- frontend/src/i18n/locales/{en,nl}.json
- frontend tests; e2e spec and fixture
- docs-project/entities/guides/GUIDE-data-entry.md

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**
- `anchor` query parameter: exactly once, non-empty; read through the gated
`visibleReader.address`. Hidden, missing and wrong-type all give
`errPageAnchorNotFound`, the uniform 404.
- The relation and direction come only from config, never from the request.
- `name`, `card`: unchanged handling.

**Security-Sensitive Operations:**
- Aggregates: rows are read by `listRowsFor` (row gate, world, faces,
query scope, condition) narrowed to the anchor's reach, and redacted once per
row before a group or matrix fold reads a value. The reach only removes rows. A
pre-filter count is never served: `total` is the narrowed length. No server
cache; the SPA keeps values in component state only.
- The anchor is read before the rows, so the 404 does not depend on the
list's contents (no existence oracle).

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**
- AC1, AC2: `dataentryconfig` table tests over a small metamodel.
- AC3: handler test, two anchors with different related rows, each display.
- AC4: ACL test with a row hidden by `acl.yaml` related to the anchor.
- AC5: hidden anchor vs missing anchor give the same status and code.
- AC6: 400 cases.
- AC7: Vitest for PageView props and DashboardView anchor handling; e2e on a
steps entity page with a document step and a dashboard step.

**Edge Cases:**
- Anchor with no related rows: count 0, empty groups.
- Incoming direction scope on a card.
- Faced anchor type: only edges the anchor's face owns, as a list tab.
- A row related twice to the anchor counts once (id set).

**Negative Tests:**
- Missing/duplicate `anchor`, `anchor` on a plain dashboard, unknown anchor.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**
- Diverging from the list tab's notion of "related": mitigated by sharing
`anchorReach`.
- A scoped card could cost a full list read per card: mitigated by
narrowing the read by id to the anchor's reach (`gatedListQuery.reach`), so it
reads only the reached rows; no per-row lookups (RR-IRXJWNP).
- `DashboardCard` passes the plimsoll field line: pinned with the
format-mirror exception, as `List` is.

Effort: m.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
- [x] docs-project GUIDE-data-entry: pages section (entity page tabs) and
dashboards section (entity dashboards).

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** Self-review, no critical or significant findings.
Two adjustments made to the plan: the anchor is resolved before the rows so the
404 cannot depend on row contents, and card links are hidden on scoped cards so
a click does not open the unscoped list under an entity title.
