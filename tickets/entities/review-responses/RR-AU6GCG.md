---
id: RR-AU6GCG
type: review-response
title: Lookup error turns a miss into a 5xx
finding: When the retired-id lookup failed, redirectRetired wrote the gate error, so a request that would have been a plain 404 or 422 became a 5xx because of a convenience feature.
severity: minor
resolution: A failed lookup is logged with slog.Warn and answered as the miss it would have been. TestRetiredRedirect_LookupFailureIsAMiss pins it for the entity GET and the view.
status: addressed
---
