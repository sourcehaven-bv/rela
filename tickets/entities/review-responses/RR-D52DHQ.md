---
id: RR-D52DHQ
type: review-response
title: 'PR 8: single-face AtFaces renders as a set predicate'
finding: Hot single-face reads sent face = ANY($n) / IN (json_each) instead of an equality.
severity: minor
resolution: faceSelectionCond renders a one-face AtFaces as an equality on pg and sqlite.
status: addressed
---
