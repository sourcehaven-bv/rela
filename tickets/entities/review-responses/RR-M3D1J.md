---
id: RR-M3D1J
type: review-response
title: getRowCount + expect().toBe() not retrying
finding: '`expect(await listPage.getRowCount()).toBe(n)` runs once. Use `expect(locator).toHaveCount(n)` with a locator-returning accessor.'
severity: nit
reason: Nit. Would need a rows locator accessor on ListPage; can be added when the first flake surfaces.
status: deferred
---
