---
id: RR-J6OY1MZ
type: review-response
title: In-memory migration sees the migrated schema while rela migrate sees the raw one
finding: '[security] Schema-aware data-entry migrations would get a different schema at load than in rela migrate.'
severity: minor
resolution: loadConfig now passes a nil schema, exactly as load-time detection did before, so the applied set equals the set that used to refuse.
status: addressed
---
