---
id: RR-IC3U0
type: review-response
title: 'rela.cache namespace footgun: shared script across docs, unqualified keys collide'
finding: 'Two documents pointing at one .lua file share a cache namespace (script path). If the script doesn''t include rela.document.id in its keys, data for different docs collides silently. Guide warns about this, but warn-in-docs-only is a trap. Design question: auto-inject rela.document.id into the namespace for doc-mode runtimes, or expose rela.cache.scoped(prefix) as a helper.'
severity: minor
reason: 'Deferred: design question with trade-offs (auto-inject vs. opt-in helper). Filed as TKT-96VGO with recommendation for rela.cache.scoped(prefix). Will address next time someone hits the footgun or when we have bandwidth to land the API addition.'
status: deferred
---

From post-impl cranky review "leverage" section.

Deferred: design question, not a regression. Filed as backlog ticket with
alternatives spelled out. Current state is correct for the documented contract
but invites footguns.
