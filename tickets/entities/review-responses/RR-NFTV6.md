---
id: RR-NFTV6
type: review-response
title: Table horizontal scroll may break sticky header
finding: overflow-x:auto wrapper around table with position:sticky thead is a known gotcha. Sticky top:0 becomes no-op if container has no fixed height. Plan doesn't address this interaction.
severity: significant
resolution: Addressed in updated plan PLAN-L6U02
status: addressed
---
