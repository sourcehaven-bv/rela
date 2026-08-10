---
id: RR-G9KT8H
type: review-response
title: 'Automation matchTyped: transpile-error string fallback flips verdict for regex/fuzzy on typed prop'
finding: 'engine.go matchTyped: old code returned handled=true,matched=false when filter.Match ERRORED (e.g. regex/fuzzy op on an integer/date/bool property, which filter.validateOperatorForType rejects), so the string fallback did NOT run. New code returns handled=false on a CompileFilter/type error, so matchProperty falls through to matchSimple->filter.MatchValue, which HAPPILY runs the regex against the stringified value. Concrete: automation `when: priority=~[0-9]` on integer priority -> OLD: filter.Match errors -> automation does NOT fire; NEW: predicate compile fails (regex wants string, priority is IntType) -> falls to matchSimple -> regex matches ''5'' -> automation FIRES. Opposite verdict on a live write path (engine.Process runs on every create/update). The ''strictly no-worse'' comment is false. FIX: on transpile/compile error for a DECLARED property, reproduce the old contract (return handled=true,matched=false) OR fall back to filter.Match for that clause — do not silently drop to the string matcher.'
severity: critical
resolution: 'automation matchTyped now falls back to filter.Match (the exact legacy typed path) with handled=true when CompileFilter can''t transpile the clause — it does NOT drop to the string matcher. So regex/fuzzy on an integer/date/bool property reproduces filter.Match''s reject (comparison can''t hold -> no match), never the string-path accept. Pinned by TestEngine_WhenCondition_UntranspilableClauseKeepsLegacyVerdict (when: count=~[0-9] on integer count -> automation does NOT fire).'
status: addressed
---
