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
