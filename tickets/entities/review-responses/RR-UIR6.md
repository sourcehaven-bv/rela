---
id: RR-UIR6
type: review-response
title: Silent catch on malformed URL hides bugs
finding: The try/catch around new URL(u) returns false silently. A genuinely malformed request URL should fail loudly, not be classified as same-origin. (Moot once S3's same-origin assertion replaces this code path.)
severity: nit
resolution: Fixture now pushes malformed URLs into offOriginRequests with a `<unparseable>` prefix instead of silently ignoring (fixtures.ts:324-331). They fail the test with the raw URL in the message.
status: addressed
---
