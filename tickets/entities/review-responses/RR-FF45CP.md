---
id: RR-FF45CP
type: review-response
title: 'Calendar feed: validate_feeds.go rejects datetime; calfeed emits all-day only'
finding: 'validate_feeds.go:89,127 hard-reject a feed date/end_date source unless Type == PropertyTypeDate (verified), so a datetime prop cannot back a feed today. And calfeed emits VALUE=DATE all-day only. The plan declared timed feeds OUT of scope (that is TKT-RDM9M5/FEAT-OT4361), which is defensible, BUT the plan must at least DECIDE: (a) leave the gate as-is (datetime not feed-eligible yet) - acceptable and consistent with ''metamodel primitive only'' scope; or (b) allow datetime in the gate now but keep all-day rendering (confusing - drops the time). Recommendation: keep (a), leave validate_feeds.go untouched, and note in docs that datetime feed sources are the follow-on ticket. Just make it an explicit, documented decision, not silence.'
severity: significant
resolution: 'Explicit decision: option (a) - datetime is NOT feed-eligible in this ticket. Leave validate_feeds.go:89,127 untouched (feed sources stay date-only). This ticket is the metamodel primitive only; timed feed sources + timed DTSTART are the follow-on (TKT-RDM9M5/FEAT-OT4361). Document in docs/data-entry.md that datetime feed sources are not yet supported and point to the follow-on ticket. No silent gap - documented boundary.'
status: addressed
---
