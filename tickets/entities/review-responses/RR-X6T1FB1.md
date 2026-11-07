---
id: RR-X6T1FB1
type: review-response
title: Rejected verified events leave no log entry
finding: A 400 for a missing required claim was not logged; the IdP will not retry a 400 so a dropped revocation leaves no trace.
severity: minor
resolution: A Warn line with event and jti and family and missing claim is logged before the 400; the user id is not logged.
status: addressed
---
