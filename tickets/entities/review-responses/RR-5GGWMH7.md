---
id: RR-5GGWMH7
type: review-response
title: Empty reach reads the whole list
finding: A nil reach in configuredListRows means no narrowing; with no candidate ids the child or count read would cover the whole list.
severity: significant
resolution: navRelatedRows skips the read when there are no candidates. Tested by an entry with no edges.
status: addressed
---
