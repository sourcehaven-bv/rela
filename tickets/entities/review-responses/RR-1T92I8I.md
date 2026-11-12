---
id: RR-1T92I8I
type: review-response
title: Reconcile acceptance overstates
finding: handleV1Reconcile can refuse a signed-in caller through the admission gate on load. The reason should say it never refuses on authorization.
severity: minor
resolution: Reason now says it never refuses on authorization, names the admission gate, and cites TestReconcile_NoIdentityFailsClosed.
status: addressed
---
