---
id: RR-1OZ2XO
type: review-response
title: Slow link delays navigation without feedback
finding: The dialog closes, then the link request runs before router.push, so a slow link looks like a dead click.
severity: minor
resolution: Kept the link before navigation and documented why in SpaceCreateMenu.
reason: The link must land before the entity page opens, or that page shows without the relation. It is one request, normally well under the navigation indicator delay.
status: wont-fix
---
