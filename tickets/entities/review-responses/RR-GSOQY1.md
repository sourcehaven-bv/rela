---
id: RR-GSOQY1
type: review-response
title: 'PR-A: O(n) family scans on every delete/rename in fs/mem'
finding: stateFamily adds a full entities-map scan (plus the existing relations scan) to every delete and rename; with idTaken a rename is ~3 full scans. A pointerless 50k-entity project pays for a feature it doesn't use. entityOrder is sorted and states share the bare-id prefix, so binary search + prefix walk gives O(log n + k).
severity: minor
reason: 'Deferred as a follow-up optimization: not a Step-1 blocker (deletes/renames are rare interactive ops; graphs rela targets are far below the sizes where three map scans matter), and the sorted-order optimization is mechanical once profiled. Flagged for the architect to file alongside the reviewer''s ReopenFactory and typed-default-face suggestions.'
status: deferred
---
