---
id: RR-JXTTP5L
type: review-response
title: Config validation narrower than ListColumn and ViewSection reuse
finding: 'ListColumn is shared by lists and dashboards; link: columns wrap an anchor; side-panel sections ignore new keys. Refuse those combinations at load.'
severity: minor
resolution: Config load refuses render on list, dashboard, nested and side-panel columns, input with link, relation/face/busy columns, and totals off tables.
status: addressed
---
