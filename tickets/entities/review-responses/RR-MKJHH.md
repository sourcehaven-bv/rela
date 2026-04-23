---
id: RR-MKJHH
type: review-response
title: Test helper hardcodes brittle URL-encoding in assertions
finding: frontend/src/composables/useDocumentClicks.test.ts hardcodes %23biz, %2Fentity, etc. in expectations. For a URL-building test arguably fine, but if goldmark/vue-router change encoding those all break silently. Extract shared constants (anchorId, returnPath) and build both DOM + expected from them.
severity: nit
reason: URL-encoding assertions hardcode %23, %2F, etc. — brittle if goldmark/vue-router ever change encoding. Cosmetic; tests would break loudly and be easy to fix at that point. Could be addressed with shared constants later if this area churns.
status: deferred
---
