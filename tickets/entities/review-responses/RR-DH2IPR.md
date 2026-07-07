---
id: RR-DH2IPR
type: review-response
title: hiddenSearchFields stale-branch comment misdescribed why it was safe
finding: 'The GetEntity-failure ''return nil,nil'' claimed the seam''s found=false path drops the hit, but that path was never reached: len(hiddenFields)==0 short-circuits before MatchedFields runs. The real safety net was an undocumented cross-consumer contract (every caller must re-load and drop).'
severity: significant
resolution: 'Made moot by RR-DILMO4: the seam loads once and its own GetEntity-failure branch (in fieldVisible) drops the hit fail-closed with a correct comment. The consumer callback no longer loads, so there is no misdescribed branch left.'
status: addressed
---
