---
id: RR-9WOSL
type: review-response
title: api fixture depends on appPage, taxing pure-API tests
finding: api fixture pulls in appPage (browser context + navigate). Pure-API tests pay ~200ms unnecessarily. Use playwright.request.newContext for the api fixture instead.
severity: nit
reason: Optimisation only. api fixture depends on appPage (which already starts a context) so the browser startup cost is already paid by any test that needs the UI. Pure API-only tests could optimise with playwright.request.newContext; defer until suite runtime is actually a problem.
status: deferred
---
