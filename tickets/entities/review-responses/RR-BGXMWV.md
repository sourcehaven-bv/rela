---
id: RR-BGXMWV
type: review-response
title: S2-dateadd-overflow
finding: date_add did int(n.Float()) with no range or integrality check. n=1e20 saturated to maxint and AddDate then wrapped the date BACKWARDS; a fractional count truncated silently (1.9 -> 1).
severity: significant
resolution: Reject a non-integral or out-of-range count with a clear error - the same refuse-ambiguity posture as the month/year restriction. Pinned by TestDateAdd_RejectsFractionalAndHugeCounts.
status: addressed
---
