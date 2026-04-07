---
id: RR-BA6N
type: review-response
title: Same-origin SSE positive-path test was missing
finding: The reviewer asked whether removing CORS reflection broke the SPA's own SSE subscription. There was a negative test (cross-origin SSE rejected) but no positive test confirming same-origin SSE still works through requireSameOrigin.
severity: minor
resolution: Added TestSecuredRouter_AllowsSameOriginSSE which exercises GET /api/events with same-origin Host and Origin headers, asserts 200 and text/event-stream Content-Type. Cancels the request context after a brief sleep to let the long-lived handler return.
status: addressed
---
