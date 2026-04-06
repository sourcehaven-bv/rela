---
id: RR-X8V0
type: review-response
title: Use explicit role selection instead of browser focus for swatch assignment
finding: The plan says 'click a swatch to assign to the focused role' but clicking a swatch steals browser focus from the input. Use an explicit 'selectedRole' ref — user clicks a role label to select it (highlighted border), then clicks a swatch to assign. This is more reliable and clearer than relying on native focus events.
severity: minor
resolution: Using explicit selectedRole ref instead of browser focus. Click role label to select, click swatch to assign.
status: addressed
---
