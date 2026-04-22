---
id: RR-QED77
type: review-response
title: 'ExecuteDocument threads path twice: NewWriterRuntime + SetScriptPath'
finding: internal/script/executor.go:88-123. path goes to NewWriterRuntime for per-script secret lookup AND to runtime.SetScriptPath for rela.cache.* namespacing. 5-line comment admits the awkwardness. Latent footgun for the next contributor.
severity: minor
reason: 'Deferred: affects ExecuteAction too, not just ExecuteDocument; best fixed in one sweep. Filed as TKT-5LCNM. Not a correctness bug, just an ergonomic cleanup.'
status: deferred
---

From go-architect review finding #11 (other observations) and post-impl cranky
review.

Deferred: fix is a lua.WithScriptPath option or have NewWriterRuntime call
SetScriptPath internally. Affects ExecuteAction too. Filed as separate backlog
ticket.
