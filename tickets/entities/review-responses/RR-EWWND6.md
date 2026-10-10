---
id: RR-EWWND6
type: review-response
title: Usage endpoint accepted a free-form value
finding: GET usage with arbitrary value= over the raw store recovers hidden values and counts.
severity: significant
resolution: 'There is no free-form usage endpoint. Counts are computed only inside a save plan from the schema diff: the value is an option the new schema removes. Corrected after the CISO review on #1792: no per-type ACL or visibility check applies; config:edit is admin-equivalent (see RR-GOCII8).'
status: addressed
---
