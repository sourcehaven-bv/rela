---
id: RR-K6DJL
type: review-response
title: api fixture always sets Origin, masking CSRF regressions
finding: 'Every api call sets Origin: serverUrl, so the Origin allowlist middleware is never exercised. A regression that disables the allowlist would pass tests silently. Backend security canary is missing.'
severity: critical
resolution: Added e2e/tests/origin-security.spec.ts with three canary tests covering no-Origin (403), mismatched-Origin (403), and matching-Origin (ok). Bypasses the api fixture using playwrightRequest.newContext.
status: addressed
---
