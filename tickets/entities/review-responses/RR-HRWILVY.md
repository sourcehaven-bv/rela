---
id: RR-HRWILVY
type: review-response
title: x check proves alignment only between tiles
finding: tile.x was compared with another tile, not with the grid.
severity: nit
resolution: Dropped the x check; width equal to the grid plus no overflow already implies it.
status: addressed
---
