---
id: RR-TEFANWN
type: review-response
title: Background reload failure replaces the page
finding: A transient failure of the live-change reload sets error, which replaces the page and the open editor instead of keeping what is on screen.
severity: minor
resolution: A background load that fails transiently only logs; refusals still drop held content.
status: addressed
---
