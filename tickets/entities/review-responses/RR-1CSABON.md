---
id: RR-1CSABON
type: review-response
title: Verdict and header read are two statements
finding: headersByIDs answers the verdict with MatchingFaces and reads headers in a second call, so a write between them can serve a newer header.
severity: minor
resolution: Documented in the headersByIDs godoc\; it is the same two-step loadRowContent already takes and reads never wait for writes.
status: addressed
---
