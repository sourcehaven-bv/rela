---
id: RR-T6SNQN
type: review-response
title: Shrinking the type list slides the highlight onto a different row than the user sees
finding: '`rankTypeNames(...).slice(0, MAX_TYPE_SUGGESTIONS)` returns a different-length list per keystroke. When it shrank by one, every index below the cut shifted up a row silently. A user with `review-response` highlighted at index 2 who typed one more character found index 2 now pointing at the first ENTITY row; Enter in that window inserted an entity reference into the document when they meant to scope to a type. A wrong write to the user''s document from a keystroke that looked correct.'
severity: critical
resolution: 'Resolved by the same identity-based highlight as RR (threshold stranding): the stored identity either still resolves to the same row or does not resolve at all, so it can never slide onto an unrelated row. Pinned by ''keeps the same type highlighted when the suggestions re-rank'', which asserts the highlight still names the same type after a re-rank, or falls back to index 0 when that type left the list.'
status: addressed
---
