---
id: RR-3UR3VH
type: review-response
title: --where fuzzy-with-wildcard / regex-on-typed is a hard regression (was working)
finding: 'cli/list.go applyListFilters routes --where through CompileFilter now. A --where clause filter.MatchAll tolerated but FromFilter refuses (fuzzy-with-wildcard ''title~foo*bar'', or regex/ordered op on a wrong type) now HARD-ERRORS the command (''invalid --where filter: ... not transpilable'') instead of working. --where is kept for back-compat and only deprecated, so a previously-working invocation failing outright is worse than a deprecation warning. FIX: fall back to filter.MatchAll for --where clauses FromFilter can''t transpile (keep legacy behavior for the legacy flag), or at minimum document the breakage in release notes.'
severity: significant
status: open
---
