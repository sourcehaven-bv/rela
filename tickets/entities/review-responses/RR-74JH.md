---
id: RR-74JH
type: review-response
title: 'scanCodeSpanCandidates: codeSpanText concatenation may drop non-Text children silently'
finding: 'internal/dataentry/mentions.go lines 112-120: codeSpanText iterates the CodeSpan''s children but only keeps `*ast.Text` ones. goldmark generally builds CodeSpan contents as Text segments, but if any extension (or a future goldmark internal) introduces another inline kind inside a CodeSpan, the function silently drops content — the resulting candidate key would be a substring of the real code span text, never matching any entity ID. The error is invisible. Either (a) document the invariant ''goldmark CodeSpan children are *ast.Text only'' and `_, _ = c.(*ast.Text)` won''t fail open, or (b) walk all descendants and fall back to source byte slicing using the node''s segments. The Lua-side analog (markdown.go line 781 area in internal/lua) does the same plus a length check on the source bytes — worth aligning.'
severity: nit
resolution: codeSpanText now returns (text string, complete bool). Code spans containing non-*ast.Text children return complete=false; scanner skips them. We never silently match a partial reconstruction.
status: addressed
---
