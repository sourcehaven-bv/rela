---
id: RR-Y6QTU
type: review-response
title: rewriteNodes/rewriteNode methods don't use the receiver
finding: rewriteNodes and rewriteNode are methods on *Runtime but never reference r. Other walker code legitimately uses r.L.NewTable(); these don't. Should be free functions to match the existing pattern (padCell, renderSeparator, etc.).
severity: minor
resolution: rewriteNodes and rewriteNode are now free functions. Same change for the old buildRefRegex/isBoundary which were replaced by free functions sortKeysByLength, isWordRune, boundaryBefore, boundaryAfter.
status: addressed
---

# Finding

`rewriteNodes` (`markdown.go:1481`) and `rewriteNode` (`markdown.go:1494`) are
declared as `func (r *Runtime) ...` but never reference `r`. The existing
pattern in this file mixes free functions and methods; helpers that don't need
`r.L` (like `padCell`, `renderSeparator`) are free.

# Resolution

Drop the receiver. Make them free functions: `func rewriteNodes(...)`, `func
rewriteNode(...)`. Update callers.
