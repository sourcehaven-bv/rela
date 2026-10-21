---
id: RR-57P2ZC
type: review-response
title: Generated-ID collision is not retried
finding: createCore re-mints up to 10 times on ErrEntityAlreadyExists; the converter calls buildCandidateEntity + persistCandidate once, so a concurrent create of the same type fails the conversion with 409.
severity: significant
resolution: Fixed by moving to CreateLinked, whose createCore retries a generated-id collision.
status: addressed
---
