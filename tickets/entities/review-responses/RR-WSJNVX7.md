---
id: RR-WSJNVX7
type: review-response
title: Root guard had no unit test
finding: 'Only e2e covered the root redirect: not the failure fallback, the loaded-store branch without a request, or that lastSpace() reaches getSidebar.'
severity: minor
resolution: frontend/src/router/rootRedirect.test.ts covers no spaces, server-resolved space, remembered space passed as ?space=, request failure, loaded store without request, and loaded store with spaces.
status: addressed
---
