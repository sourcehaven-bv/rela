---
id: RR-EWWND6
type: review-response
title: Usage endpoint accepted a free-form value
finding: GET usage with arbitrary value= over the raw store recovers hidden values and counts.
severity: significant
resolution: 'Plan changed: value must be a declared option of a choice-list property; the per-type AllowAll and property-visibility check applies.'
status: addressed
---
