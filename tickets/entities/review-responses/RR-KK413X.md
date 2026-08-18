---
id: RR-KK413X
type: review-response
title: Engine.WithOptions mutated a shared receiver
severity: significant
status: addressed
finding: 'kvuserstate.load() cached the document after first use and never re-read it, so a second process observed NONE of the first process s writes -- not merely conflicting ones. A user snoozing on server A kept seeing the suggestion on server B indefinitely, and B s next write clobbered A s whole document. The package doc understated this as last-writer-wins.'
resolution: 'load() re-reads on every operation, so a cross-process write is observed. The remaining limitation (concurrent WRITES still clobber the whole document) is now stated plainly in the package doc rather than understated, with the postgres backend named as the multi-process answer. A test with two Stores over one KV pins visibility, and was mutation-tested: restoring the cache makes it fail.'
---
