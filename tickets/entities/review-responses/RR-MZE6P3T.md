---
id: RR-MZE6P3T
type: review-response
title: Anonymous requests log a Warn per request
finding: access.Opener.Open logs identity resolution failed at Warn for each anonymous request. For a client error this is log noise.
severity: minor
reason: The log line lives in internal/access and predates this fix. Changing log levels there is outside the scope of the status-code fix.
status: deferred
---
