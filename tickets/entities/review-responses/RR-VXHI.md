---
id: RR-VXHI
type: review-response
title: HSL derivation edge cases not specified
finding: 'The plan mentions ''Lighten ~2%'', ''Darken ~3%'', ''Mix at ~15%'' for derived colors but doesn''t specify clamping behavior. What happens when surface is already #ffffff (can''t lighten card-bg)? Or when base is #000000 (sidebar-text derivation)? The derivation functions need explicit clamping rules and the tests should cover these boundary cases.'
severity: minor
resolution: 'Plan updated: HSL lightness clamped to [0,1]. When surface is near-white, derived card-bg/input-bg equal surface. When base is near-black, sidebar-text forced to #e8e8e8. Boundary tests added to test plan.'
status: addressed
---
