---
id: RR-AAQSM35
type: review-response
title: Threat model misses Lua code injection
finding: Inline lua is rendered with interpolateSafeOnly. A unified render step that used interpolate would turn entity values into Lua source. The empty-owner pile rule and background jobs are also not listed.
severity: significant
resolution: 'Added to the threat model: script outputs keep interpolateSafeOnly, the empty-owner skip in planPilePush is kept, and background scripts still go to jobs.Queue through autocascade. The synthetic golden has an inline lua that names new.title, which must stay uninterpolated.'
status: addressed
---
