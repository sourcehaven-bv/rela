---
id: RR-V8E4IDV
type: review-response
title: Kind can be silently ignored
finding: A backend arm that misses Kind falls back to text comparison where 10 < 9. Reject Kind on non-ordered ops and default unknown kinds to ErrInvalidQuery.
severity: minor
resolution: ValidateCond and the prop validator reject Kind on non-ordered ops and unknown kinds with ErrInvalidQuery; every backend default arm fails on an unknown kind. Covered by RunGraphCondTests.
status: addressed
---
