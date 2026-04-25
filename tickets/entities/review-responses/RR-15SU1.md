---
id: RR-15SU1
type: review-response
title: Legacy handler used 400 instead of 422 for the same validation failure
finding: handleAPICreateEntity (handlers_api.go:387) returned http.StatusBadRequest (400) for validateCreateIDOpts failures, while the v1 handler returns 422. 400 means malformed request; the body here is well-formed JSON failing a semantic rule. The two endpoints disagreed on status for identical inputs.
severity: significant
resolution: Switched legacy handler to http.StatusUnprocessableEntity (422) at handlers_api.go to align with the v1 handler. Documented the choice in a code comment.
status: addressed
---
