---
id: RR-EGTWJ3
type: review-response
title: 'Review nits: test name, deadline factor, Token contract'
finding: TestBus_ConcurrentSubscribeLosesNothing name overstated; 2*ping deadline unexplained; Subscription.Token accepts any seq without a caller contract; write helper assumes one event per create; initial-connect gap predates change.
severity: nit
resolution: Renamed to TestBus_ConcurrentSubscribe with an accurate comment; named sseMissedPings with a reason; documented that Token's caller must pass a delivered position. The write helper and initial-connect gap are left as is (harmless today; the gap is noted for TKT-HJ5Z9Y/TKT-FMS8BS).
status: addressed
---
