---
id: RR-3ZX6Z
type: review-response
title: Test file pokes at private store internals via .set() and direct method assignment
finding: 'The test reaches past the public store API: schemaStore.entityTypes.set(...), schemaStore.relationTypes.set(...), and entitiesStore.fetchList = vi.fn(). Refactors of the store internals would break unrelated tests. The ''as never'' casts are a tell. Prefer vi.spyOn or a public mutator.'
severity: significant
reason: The flagged pattern (schemaStore.entityTypes.set(...), entitiesStore.fetchList = vi.fn()) is the established convention in this codebase — see frontend/src/components/lists/EntityList.test.ts lines 42, 49, 60, which uses identical Map.set() seeding and direct method assignment. Changing this single test to a different pattern would create an inconsistency without addressing the root cause. A project-wide refactor to introduce store test helpers is its own ticket.
status: wont-fix
---
