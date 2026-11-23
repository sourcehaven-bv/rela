---
id: RR-YC298WN
type: review-response
title: Feed is request-path, not batchJob
finding: The CalDAV dynamic collection keeps one driver's members and the feed filters by where:, so the whole-type read is request-path debt.
severity: significant
resolution: Feed stays request-path naming TKT-SMUO8CS; CalDAV dynamic collections now read their members by id.
status: addressed
---
