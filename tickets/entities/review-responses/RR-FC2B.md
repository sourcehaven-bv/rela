---
id: RR-FC2B
type: review-response
title: 'Round 2 NEW-1: V1ViewEntity._props/_fields false-positive (reviewer read stale branch)'
finding: |
  Reviewer asserted V1ViewEntity._props and ._fields do not exist in api_v1.go or views.ts. The plan would be writing against a non-existent wire surface.
severity: critical
status: addressed
resolution: |
  False positive. TKT-IHC7D PR #950 merged to develop at 2026-06-11 22:16 UTC, commit e31d5faf. The wire surface exists today:
    - `internal/dataentry/api_v1.go:3012-3014` — V1ViewEntity has `Props map[string]any` (_props) and `FieldAffordances *map[string]V1FieldAffordance` (_fields).
    - `frontend/src/api/views.ts:39-40` — ViewEntity has `_props?: Record<string, unknown>` and `_fields?: Record<string, FieldAffordance>`.

  The IHC7C branch was created off develop BEFORE the IHC7D merge timestamp. Reviewer received findings against the stale base. After rebase onto current develop (which includes e31d5faf), the surface is present. No plan changes needed.
---
