---
id: kanban-full-fetch-pagination-test
type: automated-measure
title: 'Regression test: KanbanView fetches all pages (loops until has_more is false)'
description: 'Two-layer regression guard for BUG-5OAQUG. Component test (KanbanView.pagination.test.ts) mocks the HTTP client so the real listAllEntities loop runs inside the mounted board: asserts cards from every page render (including a page-2-only entity in its column) and that the cap-hit case shows the truncation banner. API-layer unit tests (api/entitiesListAll.test.ts) cover multi-page merge, included merge, dedupe by ID across pages, response-driven advance when the server ignores per_page, and the 50-page cap preserving has_more.'
kind: test
location: frontend/src/views/KanbanView.pagination.test.ts
status: active
---
