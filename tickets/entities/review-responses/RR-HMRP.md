---
id: RR-HMRP
type: review-response
title: Sparse items table truncates silently
finding: renderList iterates 1..items.Len(), but Lua's table length is the border, not count. {[1]=a,[3]=c} has length 1. Scripts that nil out items will produce mysterious truncation. Pre-existing pattern but more impactful now that scripts will mutate task lists.
severity: significant
resolution: 'renderList now skips lua.LNil items so scripts that mutate items via items[i] = nil get a compact rendering instead of empty bullets. Documented in code comment. New TestMdRenderListSparseTable verifies that scripts deleting items don''t produce empty bullets. Note: a full sparse-table compaction would require a different iteration strategy; nil-skip is the minimal correct fix.'
status: addressed
---
