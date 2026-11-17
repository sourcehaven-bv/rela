---
id: RR-PR1NTC3
type: review-response
title: Print spills cell text into the next column
finding: The print rule set overflow visible on every element in the main area, which undid the ellipsis on list cells. Found in a print preview.
severity: minor
resolution: Removed the rule. A Playwright print of All Tickets and of a long ticket gives the same pages without it, with cells truncated.
status: addressed
---
