---
id: RR-Y2WKG0
type: review-response
title: No dataentry test for a failed peer header read
finding: A failed header read now fails the write as a 500 instead of target_not_found. Only the visibility package tested it.
severity: minor
resolution: Added TestRelationPeerReadFault_FailsTheWrite.
status: addressed
---
