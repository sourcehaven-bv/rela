---
id: RR-B1YIPG
type: review-response
title: analyze all --prose JSON drops prose details
finding: The JSON summary carried only a count, so CI could not tell which checker or entity failed.
severity: significant
resolution: details.prose_entities lists findings and errors per entity; status is error when a checker run failed; TestAnalyzeProse_JSONFailureStatusAndAllDetails.
status: addressed
---
