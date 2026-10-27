---
id: RR-1KTZJI
type: review-response
title: grey badge alias ignored for dark rl tokens
finding: rlTokens read the raw dark badges map so dark.badges.grey never reached the grey tokens.
severity: significant
resolution: foldBadgeAliases before rlTokens for the dark palette; tested.
status: addressed
---
