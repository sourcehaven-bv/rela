---
id: RR-UD1D
type: review-response
title: Snapshot/DOM-equivalence "matrix" is hand-wavy
finding: |
  AC #7 promises "every (propertyType, display mode, value shape) combination present in repo configs." Repo configs collectively have 30+ entity types and dozens of view configs; the literal cross product is hundreds of combinations. Ticket gives no enumeration strategy. History says this becomes "5 cases and call it done" -- the failure mode RR-W3J1A flagged in TKT-MZSIJ.
severity: significant
resolution: |
  Plan revised. Replaced the matrix-from-configs framing with two concrete tests: (a) per-widget display-mode unit tests catch forward routing regressions: ~3 cases per widget x 8 widgets = ~24 assertions; (b) one-time pre/post DOM diff at merge captures the rendered HTML of populated properties/cards/list sections on develop vs this branch, documented in the review checklist. The diff is reviewed against the Known behaviour deltas table (RR-UD1C); any unlisted diff is a regression. This matches the verification rigor that shipped TKT-MZSIJ cleanly.
status: addressed
---
