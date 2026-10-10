---
id: RR-CA19MV
type: review-response
title: '[security] FaceMoved bypasses MaxPerTarget'
finding: '[security] Store.Add skips the per-target cap, so merging into an occupied destination can exceed it.'
severity: nit
resolution: 'Documented on FaceMoved: the cap limits client posts, and refusing mid-migration would strand comments.'
status: addressed
---
