---
id: RR-XTU5FL
type: review-response
title: Container-query threshold is coupled to track size and gap
finding: The 580px clamp for wide tiles is derived from minmax(280px) + 20px gap. CSS cannot read a custom property in an @container condition, so a later change to the track width silently breaks the clamp.
severity: significant
resolution: One comment block beside the grid rule states the derivation and names both rules that must change together. The narrow regime (200px + 12px) stays at most two columns below 580px, so 1 / -1 is correct there too.
status: addressed
---
