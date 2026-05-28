---
id: RR-P6HP
type: review-response
title: Metamodel hot-reload not handled; resolver holds stale RecordType
finding: 'cmd/rela-server''s app.StartWatching() may reload the metamodel without rebuilding the resolver. The resolver holds *metamodel.Metamodel captured at construction; after hot-reload, predicate compiles succeed against the old shape but Evals reference stale fields. Adding a property like ''assignee'' and writing `when: entity.assignee == current_user.id` would silently bind as Nil until restart.'
severity: significant
resolution: 'Documented in docs/security.md: ''adding properties referenced by predicates requires server restart.'' Added regression test: after metamodel reload via watcher, resolver still uses original RecordType. v1 explicitly does not support live predicate reload. If app.StartWatching''s reload turns out to rebuild the resolver (TBD during implementation), update docs accordingly.'
status: addressed
---
