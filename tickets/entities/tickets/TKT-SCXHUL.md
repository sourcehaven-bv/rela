---
id: TKT-SCXHUL
type: ticket
title: E2E coverage for relation history UI (RelationHistoryView + per-relation affordance)
kind: test
priority: low
status: backlog
---

## Problem

The relation-history frontend from TKT-92JL8P has only type-check + unit/build
coverage. The route-level view and the per-relation affordance are not exercised
end-to-end, unlike the entity history UI which has e2e coverage.

Uncovered surfaces:

- `frontend/src/views/RelationHistoryView.vue` — the timeline, two-version
compare dropdowns + swap, prop/content diff, restore button.
- The **per-relation History affordance** on `RelationCards.vue` (outgoing
relations only — the FROM entity owns the history).
- The `/relation-history/:fromType/:from/:relType/:to` route + its API calls.
- The dual-endpoint 404 behavior surfacing correctly in the UI (a relation whose
TO endpoint the user can't read shows as not-found, not a partial render).

## Scope

Add an e2e test (mirroring the entity history e2e) that, against the built
`rela-server` on a postgres-backed fixture: creates a relation, edits it a
couple of times (or drives the sweep), opens the from-entity detail page, clicks
the relation's History affordance, asserts the timeline renders, diffs two
versions, and restores. Confirm the affordance is ABSENT on incoming relation
cards.

Deliver as its own follow-up PR (per the TKT-92JL8P review discussion).

## Origin

TKT-92JL8P follow-up — "add e2e as follow up pr."

## Re-verification (2026-07-25, against develop dd0fe649)

STILL VALID, scope UNCHANGED. Target surfaces exist and have zero e2e coverage:

- `frontend/src/views/RelationHistoryView.vue` present; per-relation History
  affordance present (`RelationCards.vue:511-516` History button;
  `:167-176` `openRelationHistory`, outgoing-only).
- No relation-history e2e anywhere in `e2e/` (grep for
  relation-history/_relation_history/RelationHistory → zero). `relation-cards.spec.ts`
  covers cards generally, not history. Entity history HAS e2e — this is the gap.

Lifetime UI is OUT OF SCOPE (deliberately). TKT-HGE4KW shipped the
deleted-relation LIFETIMES feature entirely backend + CLI — no frontend files
changed. `frontend/src/api/history.ts` has no `listRelationLifetimes`/`record_id`
and `RelationHistoryView.vue` has no lifetime picker. So there is nothing
lifetime-related for this e2e to cover. Building the lifetime picker/affordance in
the frontend (a `_lifetimes` client + a lifetime selector in RelationHistoryView)
is a SEPARATE ticket that must land first; only then should an e2e cover it.
