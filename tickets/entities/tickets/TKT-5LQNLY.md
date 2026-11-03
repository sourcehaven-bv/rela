---
id: TKT-5LQNLY
type: ticket
title: 'Named dashboards: dashboards: in data-entry.yaml and dashboard: <name>'
kind: enhancement
priority: medium
effort: m
started: "2026-10-15"
completed: "2026-10-15"
status: done
---

## Description

A project has one dashboard today: the top-level `dashboard:` block, opened by
`dashboard: true` on a navigation entry, page tab, command scope or space
`home:`, and served by `/api/v1/_dashboard`. With spaces (TKT-GNKR5H,
FEAT-XT3SOP) each space wants its own home. Sourcehaven's atlas CRM space needs
a pipeline dashboard separate from the global one. TKT-GNKR5H already sketched
`home: dashboard_projects`, a named dashboard home.

### Config

```yaml
dashboard:              # the default dashboard, unchanged
  title: Overview
  cards: [...]

dashboards:             # named dashboards, same shape as dashboard:
  crm:
    title: Pipeline
    cards: [...]

spaces:
  - id: crm
    home: {dashboard: crm}
    navigation:
      - label: Pipeline
        dashboard: crm
```

- `dashboard:` and `dashboards:` coexist. `dashboard:` stays the default
dashboard.
- `default` is reserved: `dashboards.default` is a config error, and
`dashboard: default` in a reference means the same as `dashboard: true`.
- Everywhere `dashboard: true` is accepted, `dashboard: <name>` is accepted
too. An unknown name fails config load. `dashboard: true` without a `dashboard:`
block keeps its current behaviour (an empty dashboard).
- API: `/api/v1/_dashboard?name=<name>`; no name means the default. An
unknown name is a 404 `dashboard_not_found`. Card `permission:` filtering is the
same as for the default dashboard.
- SPA: `/dashboard/<name>` and `/s/<space>/dashboard/<name>`; the sidebar
highlights the entry of a named dashboard.

## Out of scope

- New card kinds such as sums (TKT-AIEGHU).
- Creating or renaming named dashboards in the Configure space (editing an
existing one is in scope).

## Acceptance criteria

- A config with only `dashboard:` behaves as before.
- A named dashboard is reachable from a navigation entry, a page tab and a
space home, at `/dashboard/<name>` (also under `/s/<space>/`).
- `/_dashboard?name=crm` serves the crm dashboard with permission-filtered
cards; an unknown name returns 404 `dashboard_not_found`.
- An unknown dashboard name in any reference, an invalid name, or a
`dashboards.default` entry fails config load with a message naming it.
