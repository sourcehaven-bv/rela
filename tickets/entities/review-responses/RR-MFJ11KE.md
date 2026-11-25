---
id: RR-MFJ11KE
type: review-response
title: Groups recompile config per group
finding: serveListGroups called viewCondition per group and recompiled every view condition N times per request
severity: significant
resolution: Compile once per request and look up each group from that lookup
status: addressed
---
