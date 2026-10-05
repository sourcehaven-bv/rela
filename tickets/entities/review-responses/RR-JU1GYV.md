---
id: RR-JU1GYV
type: review-response
title: Neighbour head read failure looked like 'no links'
finding: 'resolveHeads became infallible: ResolveIDs answers a header-read fault as no ids, so an outage silently dropped every link on GET, list and include (RR-4TFZNL).'
severity: significant
resolution: Added visibility.Resolver.ResolveIDsErr, which returns a failed header read (a gate error still hides that type). resolveHeads returns the error again and worldScopedNeighbors/worldNeighborsForPage propagate it.
status: addressed
---
