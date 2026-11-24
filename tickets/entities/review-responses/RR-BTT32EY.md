---
id: RR-BTT32EY
type: review-response
title: Go accepts a signed for value the SPA rejects
finding: strconv.Atoi accepts +7d so it validates; the SPA regex rejects it and drops the window.
severity: significant
resolution: ForDays matches ^[0-9]+d$ before parsing; table case for +7d.
status: addressed
---
