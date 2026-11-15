---
id: RR-6EHB7NR
type: review-response
title: Empty IdP org status read as active
finding: The operator API client mapped an empty or missing status to active, so a malformed answer could resume a suspended tenant.
severity: minor
resolution: 'Fixed in 3a5b2d9ff: the client accepts only active and suspended. Any other value, including empty, is ErrUnavailable, so the IdP retries and nothing resumes. Pinned by idporg/client_test.go.'
status: addressed
---
