---
id: RR-THW7A
type: review-response
title: Workspace.Tracer()/Searcher() allocate fresh per call
finding: 'internal/workspace/services.go:46-48, 133-136 — Tracer() and Searcher() return fresh values every invocation. For automation runs that trigger many Lua scripts in sequence, each executeLuaActions call allocates afresh. Both are cheap, but the churn is unnecessary and the ''materialise'' comment is slightly misleading. Fix: cache tracer and searcher on Workspace at construction, or document per-call freshness.'
severity: minor
resolution: Memoised Workspace.Tracer() and Workspace.Searcher() using sync.Once fields. First call builds the wrapper; subsequent calls return the cached value. Eliminates per-call allocation churn during automation runs that trigger many Lua scripts.
status: addressed
---
