---
id: RR-36PGJ
type: review-response
title: documentScriptEngine interface shape — 6 args, time.Duration tail
finding: 'internal/dataentry/document.go:36-39. ExecuteDocument takes (path, deps, stdout, documentID, entryID, timeout). Could bundle documentID/entryID or use an opts struct. Trade-off: matches ExecuteAction''s flat shape for consistency. Works today; revisit if we grow a 7th arg.'
severity: nit
reason: Matches ExecuteAction's precedent in internal/script/action.go. Consistency across Engine's typed methods outweighs the minor elegance win from bundling. Revisit if ExecuteDocument grows a 7th argument.
status: wont-fix
---

From go-architect review finding #2.

Won't fix: matches ExecuteAction precedent in internal/script/action.go. A
consistent flat shape across Engine's typed methods outweighs the minor elegance
win from bundling. Revisit if we need ExecuteDocument to grow an argument.
