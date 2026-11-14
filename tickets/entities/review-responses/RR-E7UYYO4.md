---
id: RR-E7UYYO4
type: review-response
title: A refused network call that the code swallowed passed silently
finding: When an automation or Lua code caught the refused HTTP or AI call and went on, the case passed though it tried the network.
severity: significant
resolution: The refusing transport and AI provider set a flag; a case that tried one fails with errRealNetwork. AC5 has a handled case.
status: addressed
---
