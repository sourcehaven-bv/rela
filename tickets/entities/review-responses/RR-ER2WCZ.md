---
id: RR-ER2WCZ
type: review-response
title: Dark-only rl tokens stayed inline after switching to light
finding: A key set only by the dark palette (accent or a dark badge) was not restated in the light map.
severity: significant
resolution: Light map restates rlDefaultLightTokens for every dark-only key; TestRLDefaultTokensMatchTokensCSS pins both default maps; TestResolvePalette_ThemesSetTheSameRLKeys pins equal key sets.
status: addressed
---
