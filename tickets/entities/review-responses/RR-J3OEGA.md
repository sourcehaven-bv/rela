---
id: RR-J3OEGA
type: review-response
title: Crossing the type-section threshold strands the highlight, making Enter insert a paragraph break
finding: '`refreshTypeItems` could shrink `typeItems` to `[]` without touching `highlightedIndex`, so in the 150ms window before the search response the index pointed past the end of the combined list and `current()` returned null. `mentionKeymap` returns WITHOUT `preventDefault` when `current()` is null, so ProseMirror handled the key: the user pressed Enter on a visibly highlighted row and got a paragraph break in the document. Reproduced independently: highlight=3 against a 2-item list, current()=null. Reachable by typing any query longer than TYPE_SECTION_MAX_QUERY (6) after arrowing onto an entity.'
severity: critical
resolution: 'Fixed structurally rather than by clamping. `highlightedIndex` is no longer stored state: `MentionMenuState.highlight` now holds the row''s IDENTITY (`{kind:''type'',name}` | `{kind:''entity'',id}` | null) and `highlightedIndex()` resolves it against the live rows on every read, falling back to the first row when the highlighted row is gone and -1 when there are no rows. A stale or out-of-range index is now unrepresentable. Pinned by ''never points past the end when the type section disappears'', which highlights the LAST entity so a stale index cannot coincidentally land on another valid row (an earlier version of the test passed while the bug was present, and was strengthened after mutation testing showed it).'
status: addressed
---
