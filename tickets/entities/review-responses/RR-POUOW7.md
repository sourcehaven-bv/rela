---
id: RR-POUOW7
type: review-response
title: JSON objects lose key order
finding: map[string]any has no order and browsers sort integer-like keys first; property order drives PropertyOrder.
severity: significant
resolution: 'Already addressed in code: mappings travel as {"$m": [[key, value], ...]} and are decoded with a token-based ordered decoder (configedit/tree.go).'
status: addressed
---
