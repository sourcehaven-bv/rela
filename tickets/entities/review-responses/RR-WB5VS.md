---
id: RR-WB5VS
type: review-response
title: Dashboard creation+search-index race
finding: Dashboard Critical Issues card is a bleve query. Seeding in beforeEach then immediately navigating to dashboard may race indexing. Add api.waitForIndexed helper or expect.poll against _search.
severity: significant
resolution: Added api.waitForIndexed(id) helper that polls GET /{plural}/{id} until it returns 200. Dashboard beforeEach now waits for the last seeded entity before tests navigate.
status: addressed
---
