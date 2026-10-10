---
id: RR-PXB8WV
type: review-response
title: Local-role test did not prove the grant was real
finding: The local-role case asserted absent counts without checking that alice could read the thread.
severity: minor
resolution: Fixed. The case now also requires a 200 from GET _comments/ticket/TKT-001 as alice.
status: addressed
---
