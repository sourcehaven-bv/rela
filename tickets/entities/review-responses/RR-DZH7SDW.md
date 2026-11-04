---
id: RR-DZH7SDW
type: review-response
title: A client without a surface bypasses placement
finding: An empty surface skipped enabled and visible_on, so a cached old SPA ignored dashboard switches.
severity: minor
resolution: An unplaced request is read as the default dashboard. TestNextAction_UnplacedIsTheDashboard.
status: addressed
---
