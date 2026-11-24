---
id: RR-QQ4DD1S
type: review-response
title: httptest recorder in request path
finding: applyAssign calls the PATCH handler through httptest.NewRecorder.
severity: minor
reason: Reusing the handler keeps one write path; extracting the PATCH core is a refactor of write_handler.go beyond this ticket.
status: deferred
---
