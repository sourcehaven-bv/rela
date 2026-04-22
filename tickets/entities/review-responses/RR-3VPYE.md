---
id: RR-3VPYE
type: review-response
title: eslint selector misses page.request.fetch bypass
finding: no-restricted-syntax catches locator/getBy* but not page.request.fetch. A spec can bypass the api fixture and call request.fetch directly with no Origin header. Not a lint failure, but a test-quality failure.
severity: significant
resolution: Added third no-restricted-syntax rule in e2e/eslint.config.js banning request.fetch calls from specs. The origin-security canary spec uses playwrightRequest.newContext directly, not request.fetch, so it isn't caught.
status: addressed
---
