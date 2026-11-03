---
id: RR-MZFMBTL
type: review-response
title: Docs say the server keeps only the current state
finding: docs/realtime.md:2084, GUIDE-realtime.md:2091 and RlChannelSession.vue:6-8 become false once Previous is stored.
severity: minor
resolution: 'Reworded in docs, mirror and component comment: the header marks no earlier state as done because a session may branch.'
status: addressed
---
