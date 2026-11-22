---
id: RR-IYUNBS1
type: review-response
title: Focus ring used undefined tokens
finding: AsOfPicker's Now button used --focus-ring and --focus-ring-gap which the SPA no longer defines; cssCustomProperties.test.ts failed.
severity: minor
resolution: Uses a 2px outline in --rl-color-focus like SidebarFooter.
status: addressed
---
