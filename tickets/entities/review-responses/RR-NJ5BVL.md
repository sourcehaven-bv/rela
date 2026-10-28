---
id: RR-NJ5BVL
type: review-response
title: Two constants for one limit
finding: channels.MaxMentions and realtimelimits.MaxMentionsPerMessage are both 10 and must stay equal. Define it once, either realtimelimits referencing channels or the reverse following the existing direction of dependency, so the 422 and ErrTooLarge cannot drift apart.
severity: nit
resolution: 'One constant: channels.MaxMentions; realtimelimits gets no copy (channels does not import it).'
status: addressed
---
