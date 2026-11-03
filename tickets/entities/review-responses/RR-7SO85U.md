---
id: RR-7SO85U
type: review-response
title: Cost bound is per connection, not global
finding: connections x subscriptions x rounds has no global limit, and the claim that -verbose timing shows it is false because replays bypass request stats.
severity: significant
resolution: Rounds never overlap; 256 replays per round with refetch fallback beyond it; 64 subscriptions per principal; one log line per round with replay count, duration and store queries.
status: addressed
---
