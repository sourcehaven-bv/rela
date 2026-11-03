---
id: RR-RRWHVSP
type: review-response
title: A rule without DTSTART and a past after date now steps from that date
finding: FREQ=DAILY after 2020-01-01 now returns 2020-01-02; before it jumped past now.
severity: significant
resolution: Kept as the intended fix; documented in docs/lua-scripting.md and its docs-project mirror.
reason: 'Intended: this is the fix. It matches the predicate rrule_next and rules with DTSTART. Documented in docs/lua-scripting.md.'
status: addressed
---
