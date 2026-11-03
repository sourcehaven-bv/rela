---
id: TKT-D72A6HF
type: ticket
title: Dashboard waits for _analyze before rendering its cards
kind: enhancement
priority: medium
effort: s
status: backlog
---

## Description

Found while measuring TKT-QM833U. `DashboardView.vue` (`loadData`) awaits
`analyze()` together with the card queries (`Promise.all`), so the dashboard
shows nothing until `GET /api/v1/_analyze` returns. On atlas that took up to 16
s. The dashboard only uses `errors` and `warnings`, and those counts are capped
at 100 per section (see `analysisIssueCounts` in
`internal/dataentry/analyze.go`).

## Approach

- Render the cards as soon as their queries return; load the analysis badge
separately, with its own loading state.
- Consider whether the badge should say "100+" for a truncated section, or
use a count-only endpoint.

Acceptance: with `_analyze` delayed (e.g. 5 s in a test double), the cards
render before the analysis badge; a failed analysis does not mark the whole
dashboard as failed.
