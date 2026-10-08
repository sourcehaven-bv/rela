---
id: RR-I0EEAS
type: review-response
title: Review nits
finding: Name clash with store.HistoryReader, rune arithmetic in test, uncapped version number, bare Ref.
severity: nit
resolution: 'Renamed lua.HistoryReader to EntityVersionReader; strconv in test helper; version capped at MaxInt32. Bare Ref in waitForVersions kept: the doc type is faceless.'
status: addressed
---
