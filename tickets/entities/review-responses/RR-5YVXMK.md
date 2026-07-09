---
id: RR-5YVXMK
type: review-response
title: Planned component test cannot exercise the pagination loop through the existing @/api mock
finding: 'KanbanView.test.ts mocks the ''@/api'' module boundary (vi.mock(''@/api'', ...) replacing listEntities). If listAllEntities lives in api/entities.ts and calls listEntities internally, that call is a module-local reference — the ''@/api'' mock never intercepts it, so a component test that mocks listEntities with paged responses would exercise... nothing (the real listAllEntities would issue real HTTP in jsdom). The plan''s regression test as described would silently not test the loop. The test seam must be chosen explicitly: mock the HTTP client (''@/api/client'' api.get) so the real loop runs in the component test, or mock listAllEntities in the component test and cover the loop in API-layer unit tests.'
severity: significant
resolution: 'Plan revised with an explicit seam: the component regression test lives in a new KanbanView.pagination.test.ts that mocks ''@/api/client'' (api.get) only, so the real listAllEntities loop runs inside the mounted component. Loop-logic details (dedupe, response-driven advance, cap) are covered by API-layer unit tests at the same seam. The existing KanbanView.test.ts keeps its ''@/api'' mock, switched from listEntities to listAllEntities.'
status: addressed
---
