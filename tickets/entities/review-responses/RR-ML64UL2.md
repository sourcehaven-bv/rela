---
id: RR-ML64UL2
type: review-response
title: Mention menu re-ranks and filters on the client
finding: mentionMenuState ranks with uFuzzy and slices 20 (mentionMenuState.ts:163); limit=20 would re-rank only the server's top 20 and drop body-only matches.
severity: significant
resolution: 'Plan changed: the mention menu requests limit=100 (the server maximum); test shows a title match ranked past 20 by the server still appears.'
status: addressed
---
