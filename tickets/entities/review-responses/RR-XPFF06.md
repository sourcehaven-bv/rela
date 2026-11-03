---
id: RR-XPFF06
type: review-response
title: The e2e spec reached the second tab by a full page load
finding: Both checks in 'the menu offers only the types the page can link' used appPage.goto, so the in-app tab switch, the path users take, was not exercised end to end.
severity: minor
resolution: 'The spec now closes the menu and switches to Blockers by clicking the tab (PageTabsPage.selectTab), waits for the URL, then checks the rows. The full-list case off an entity page stays covered by spaces.spec.ts ''the Create menu offers the space''s types''. Ran space-create-page-link, spaces, pages and relation-order specs: 21 passed.'
status: addressed
---
