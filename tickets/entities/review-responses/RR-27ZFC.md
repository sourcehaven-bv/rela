---
id: RR-27ZFC
type: review-response
title: AC7 asserts wrong surface (href instead of behaviour)
finding: 'AC7 says ''verifies the detail page shows a Back button whose href equals the original category URL.'' vue-router''s <router-link> resolve() can diverge from the ?return_to= value (trailing slash, query order, encoding). The meaningful assertion is: click the button, waitForURL matches the original URL, the document body re-renders. Also: round-trip should preserve ?doc=X so user returns to the same document tab, not just the category shell.'
severity: significant
resolution: 'AC7 rewritten as a behavioural e2e: click the button, waitForURL matches original (including ?doc=X), document body re-renders. No attribute-shape assertion.'
status: addressed
---
