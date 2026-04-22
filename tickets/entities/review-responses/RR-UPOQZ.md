---
id: RR-UPOQZ
type: review-response
title: ExecuteFileWithWriter exposes too much capability
finding: Proposed script.Engine.ExecuteFileWithWriter(path, deps, stdout, opts...) takes variadic lua.Option — strictly more capability than any existing Engine method. ExecuteFile/ExecuteCode take fixed deps+entity; ExecuteAction wires mode internally. Variadic opts invites misuse by the next caller (e.g., forged WithOutputDir).
severity: significant
resolution: Typed script.Engine.ExecuteDocument(path, deps, stdout, documentID, entryID, timeout) — no variadic opts. Matches ExecuteAction shape. Plan approach §3.
status: addressed
---

From design-review on PLAN-78HJO.

Two cleaner alternatives: (a) `Engine.ExecuteDocument(path, deps, stdout,
documentID, entryID)` — encodes the data-entry contract, no opt escape hatch.
(b) Put Lua wiring inside documentService — construct the runtime directly via
NewWriterRuntime with doc-specific opts, symmetric to how Engine.ExecuteAction
does it internally.

Recommend (a) — keeps the Engine as the single seam for "run a Lua file with a
project context" while not leaking variadic opts.
