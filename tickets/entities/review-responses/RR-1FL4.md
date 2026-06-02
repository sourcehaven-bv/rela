---
id: RR-1FL4
type: review-response
title: CDN allowlist hardcodes a single host instead of asserting same-origin
finding: cdnHits only matches /maxcdn\.bootstrapcdn\.com/. If a future EasyMDE upgrade switches the FA host to cdn.jsdelivr.net or cdn.fontawesome.com, AC1 passes silently. The product invariant is 'no third-party origins are contacted'; assert that directly by comparing each captured http(s) URL's origin against page.url()'s origin. The same-origin form is strictly stronger and subsumes both AC1 and AC3.
severity: significant
resolution: Fixture now compares each request's origin against new URL(serverUrl).origin (fixtures.ts:332-340). Any off-origin host fails the test, not just maxcdn.bootstrapcdn.com — catches future CDN swaps (jsdelivr, cdn.fontawesome.com, etc.) automatically.
status: addressed
---
