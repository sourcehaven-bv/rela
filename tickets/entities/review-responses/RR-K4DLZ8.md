---
id: RR-K4DLZ8
type: review-response
title: TrimSuffix of the id ORDER BY relied on an invariant enforced elsewhere
finding: buildGraphQuerySQLSelect trims the plain ORDER BY only in the default-world arm; that is correct only because graphWorldScope returns distinctOn and rankOrder together, which nothing asserted.
severity: minor
resolution: The pairing is asserted at the call site (a one-sided return panics with a message naming the invariant) so a future change cannot corrupt the SQL silently.
status: addressed
---
