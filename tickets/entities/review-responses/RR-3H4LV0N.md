---
id: RR-3H4LV0N
type: review-response
title: Edit times are per row, not per field
finding: latest compares the row's last-updated time on each side, so an edit to another field on the same row decides the winner.
severity: minor
resolution: Documented in remote-sync.md; the API has no per-field time.
status: addressed
---
