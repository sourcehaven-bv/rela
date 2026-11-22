---
id: RR-LK2PXSJ
type: review-response
title: Realtime callers loosened without tests or docs
finding: 'FieldRestricted also gates realtime conditions, channel from: roles, triggeracl and flow templates. Only flows had a test and docs.'
severity: significant
resolution: 'Added TestParse_ReadsPropertyEveryVisibleBlockShows (realtimecfg) for a trigger when: and a from: role; GUIDE-realtime defines restricted.'
status: addressed
---
