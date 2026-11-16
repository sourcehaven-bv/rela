---
id: TKT-OF4O8PZ
type: ticket
title: Document and dashboard tabs on entity pages
kind: enhancement
priority: medium
effort: m
started: "2026-11-15"
completed: "2026-11-15"
status: done
---

## Description

An entity page (`entity_type:`, URL `/p/<page>/<entity>/<tab>`) can only show
list, kanban and gantt tabs today. A DPIA workshop needs one DPIA entity page
with a step that renders the DPIA report (a document) and a step with dashboard
cards counted over the DPIA's related entities.

Add two tab kinds that narrow to the anchor entity:

- A document tab with `scope: root` renders an entity-anchored document
(`documents.<d>.entity_type` equal to the page's) with the anchor as its entry
entity. It reuses `/_documents/{name}/{entry}` unchanged.
- A dashboard tab with `scope: root` shows an entity dashboard:
`dashboards.<d>.entity_type` set, and each card with `scope: {relation,
direction}` from the anchor to the card list's type. The card endpoint takes
`anchor=<id>`, reads the anchor through the gated reader (uniform 404), gates
the list rows, keeps those the anchor reaches, then folds.

## Acceptance criteria

- Config load accepts both tab kinds and refuses mismatched types, unscoped
cards, query cards and navigation entries to an entity dashboard.
- Card counts are computed over gated rows only, per request, never cached.
- A hidden or missing anchor answers the same 404 as an entity GET.
- Docs in the data-entry guide; Go, Vitest and e2e tests.
