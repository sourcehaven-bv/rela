---
id: custom-view-list-section-navigation-e2e
type: automated-measure
title: 'E2E coverage: clicking a list-section item in a custom detail view navigates to the linked entity'
description: 'End-to-end test that opens a custom detail view containing a section with display: list, clicks one of the list items, and asserts that the URL changes to the target entity''s detail page (preferring the configured detail_view of the corresponding list, falling back to /entity/:type/:id).'
kind: test
location: e2e/tests/
status: proposed
---
