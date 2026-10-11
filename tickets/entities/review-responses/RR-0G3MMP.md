---
id: RR-0G3MMP
type: review-response
title: Intraword emphasis breaks round trip
finding: price<strong>$100</strong>now gives price**$100**now, which goldmark does not parse as bold, so the next pass escapes it.
severity: significant
resolution: FromHTML now returns a fixed point (bounded passes), so such text settles once; pinned in TestRoundTripStable.
status: addressed
---
