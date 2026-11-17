---
id: RR-F9JV1S7
type: review-response
title: 'Trigger ACL check: cascade skips the reader rule'
finding: cascade judged a reached trigger only with unbounded and writeViolations. An edited trigger could copy a source value into a record read as narrowly as the source, and an unchanged created trigger on that type could copy it on into a record read more widely.
severity: significant
resolution: 'Each cascade event carries the edited trigger''s subjects and faces; rowViolationsFrom holds every record a reached trigger creates to readersCovered against them. Test: "a second hop creates a record read more widely".'
status: addressed
---
