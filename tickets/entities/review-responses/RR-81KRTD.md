---
id: RR-81KRTD
type: review-response
title: expiryHeader is misnamed
finding: 'internal/dataentry/sse_expiry_test.go:23: the constant holds the assertion header name (X-Auth-Assertion), not an expiry. Rename it to assertionHeader.'
severity: nit
resolution: The test uses the package's existing assertionHeader constant.
status: addressed
---
