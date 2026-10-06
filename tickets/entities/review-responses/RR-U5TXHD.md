---
id: RR-U5TXHD
type: review-response
title: Owner delete 403 names a hidden owned entity's type
finding: Deleting an owner whose owned entity the caller may not delete returns 403 naming that type.
severity: minor
resolution: Documented as a residual in docs/acl-security.md; it fails closed and withholds the id.
status: addressed
---
