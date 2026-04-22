---
id: RR-2NUE3
type: review-response
title: Hot-reload skips CheckDocumentScriptExists (and CheckActionScriptExists)
finding: 'internal/dataentry/watcher.go:165-223 reloads data-entry.yaml without re-running ValidateConfig or the script existence checks. A user who adds a new script: doc via hot-reload gets HTTP 500 at first render instead of startup error. The new guide text implies the existence check applies on reload; it doesn''t. Pre-existing pattern for action scripts.'
severity: minor
reason: 'Deferred: pre-existing gap (action scripts have the same issue). Filed as TKT-IMBOK to fix all reload-time config checks together. Out of scope for TKT-CGBVW; guide wording for this ticket reflects current reality.'
status: deferred
---

From post-impl cranky review.

Deferred: pre-existing, not introduced by this ticket. Filed as separate backlog
ticket TKT-XXX to fix both action and document script hot-reload checks
together. For this PR, soften guide wording to match reality rather than expand
scope.
