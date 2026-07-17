---
id: RR-H5I31
type: review-response
title: <100ms timing assertion is a latent flake if the cap is ever tuned down
finding: The <100ms assertion is robust today only because rejection is O(1) (a `.length` check short-circuits before any regex runs). If the cap is lowered so real matching happens near the boundary, this assertion starts timing actual regex work on a shared CI runner and will flake. Prefer asserting behaviour (return value + warn) and drop wall-clock timing, or keep a generous budget commented as a smoke check.
severity: minor
resolution: All wall-clock timing assertions removed from the suite. The rewritten tests assert behaviour only (return value + warn presence/absence), so there is no CI-load flake surface left. The <100ms assertions belonged to the abandoned cap-as-fix approach.
status: addressed
---
