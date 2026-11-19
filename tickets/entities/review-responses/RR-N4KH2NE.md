---
id: RR-N4KH2NE
type: review-response
title: '[security] Copy-created faces not fenced'
finding: '[security] A copy-state that created a face did not start that face''s lifecycle so an earlier deleted entity at the face showed through.'
severity: significant
resolution: 'StartsLifecycle marks a copy that created the face (CopyStateSummary suffix). Backfills updated. Test: ForEntityCopyCreatesFace.'
status: addressed
---

[security] A copy-state that created a face did not start that face's lifecycle
so an earlier deleted entity at the face showed through.
