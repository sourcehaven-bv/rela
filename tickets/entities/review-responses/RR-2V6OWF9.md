---
id: RR-2V6OWF9
type: review-response
title: deniedAppend matches only the owner wrapper
finding: 'appendaudit.go: a forbidden error from any path other than writableOwners would not be audited, unlike mutate.go which matches any ErrForbidden.'
severity: minor
resolution: deniedAppend now matches any ErrForbidden and names the owner only when the wrapper carries it.
status: addressed
---
