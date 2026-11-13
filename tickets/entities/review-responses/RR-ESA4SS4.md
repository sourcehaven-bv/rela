---
id: RR-ESA4SS4
type: review-response
title: Relation push reads endpoint readability twice per relation
finding: pushRelation called the read gate for both endpoints of every relation on every run.
severity: minor
resolution: The push keeps readability per endpoint for the run (relPushRun.readable) so each endpoint is checked once.
status: addressed
---
