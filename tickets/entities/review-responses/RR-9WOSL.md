---
id: RR-9WOSL
type: review-response
title: api fixture depends on appPage, taxing pure-API tests
finding: api fixture pulls in appPage (browser context + navigate). Pure-API tests pay ~200ms unnecessarily. Use playwright.request.newContext for the api fixture instead.
severity: nit
status: open
---
