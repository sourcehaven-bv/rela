---
id: RR-TPD401
type: review-response
title: Stale doc comments after timed-feed change (3 spots)
finding: 'Code review (clean pass, no critical/significant) found 3 stale doc comments the commit left behind: (1) feed_provider.go:59 declarativeFeed doc says ''all-day calendar events'' (now also timed); (2) validate_feeds.go:59-62 validateFeeds doc says ''date must be date-typed'' (now date OR datetime); (3) ical.go:242 formatDateTimeUTC doc says ''for DTSTAMP'' (now also timed DTSTART/DTEND). All load-bearing code confirmed correct by the reviewer (byte-identical all-day render, correct Timed default, mismatch validation avoids double-reporting, JSON backward-compat, provider Timed keyed on start prop).'
severity: minor
resolution: 'Fixed all three doc comments: declarativeFeed -> ''calendar events (all-day or timed, per the source date property''s type)''; validateFeeds -> ''date must be date- or datetime-typed; a datetime source yields a timed event''; formatDateTimeUTC -> ''used for DTSTAMP and for a timed event''s DTSTART/DTEND''. No code change needed.'
status: addressed
---
