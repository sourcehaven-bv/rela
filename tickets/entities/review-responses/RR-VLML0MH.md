---
id: RR-VLML0MH
type: review-response
title: Unique violation on any field deferred forever
finding: IsUniqueViolation matched any unique property, so a unique mapped field collision was silently deferred each run.
severity: minor
resolution: uniqueOn defers only a violation on the ref property (or one naming none); others are rejected with an issue.
status: addressed
---
