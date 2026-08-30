---
id: RR-PRY567
type: review-response
title: CLAUDE.md still instructs readers to depend on the deleted type
finding: The No repository or transaction abstractions rule names entitymanager.EntityManager
severity: minor
resolution: Updated CLAUDE.md's 'No repository or transaction abstractions' rule to name entitymanager.Manager.
status: addressed
---

Repo-root `CLAUDE.md`, under "No repository or transaction abstractions":
*"Depend directly on `store.Store`, `tracer.Tracer`, `search.Searcher`,
`entitymanager.EntityManager`."*

That symbol no longer exists. The rule's intent survives (depend on the manager,
not a repo layer), but it now points at a deleted type — in the same document
whose first rule motivated this ticket. Update to `entitymanager.Manager`.
