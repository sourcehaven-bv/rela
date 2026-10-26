---
id: RR-N2PACB
type: review-response
title: A bind failure in the pre-check is reported as a 403 denial
finding: 'internal/appbuild/realtime_outputs.go:83-86: any error from o.as (membership walk failure, store error) becomes outputCheckDenial(nil), a 403 session_output_denied "the write is refused". A server fault then looks like a permission problem to the poster and in logs. Fix: map acl.ErrUnstampedPrincipal and similar identity refusals to the denial, and return other errors so they surface as 500 and are logged.'
severity: minor
resolution: Check returns a non-denial bind error as a 500; only ErrUnstampedPrincipal stays a 403. TestOutputCheck_BindError.
status: addressed
---
