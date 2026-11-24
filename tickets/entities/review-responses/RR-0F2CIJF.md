---
id: RR-0F2CIJF
type: review-response
title: PATCH used the list type
finding: A subtype row was patched on the parent route.
severity: minor
resolution: Uses the row type.
reason: The list route is the list type; the server resolves a subtype row there and enforces the write. Kept to match move actions.
status: wont-fix
---
